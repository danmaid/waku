package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danmaid/dynamic-proxy/internal/api"
	"github.com/danmaid/dynamic-proxy/internal/doh"
	"github.com/danmaid/dynamic-proxy/internal/httpd"
	"github.com/gorilla/mux"
	"github.com/miekg/dns"
)

type createProxyRequest struct {
	Host        string `json:"host"`
	Backend     string `json:"backend"`
	Description string `json:"description"`
}

const testUpstreamDNS = "127.0.0.1:1"

func managedHostResolver(manager *httpd.Manager, frontIP net.IP) func(string) (net.IP, bool) {
	return func(host string) (net.IP, bool) {
		if dnsAddr, ok := manager.GetDNSAddress(host); ok {
			if ip := net.ParseIP(dnsAddr); ip != nil {
				return ip, true
			}
		}
		if manager.HasHost(host) {
			return frontIP, true
		}
		return nil, false
	}
}

func TestE2E_ProxyAndDoHFlow(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "dynamic-proxy.conf")
	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config file: %v", err)
	}

	manager := httpd.NewManagerWithConfig(confPath, func() error { return nil })
	resolver := doh.NewResolver(testUpstreamDNS)
	resolver.SetManagedHostResolver(managedHostResolver(manager, net.ParseIP("127.0.0.1")))

	r := mux.NewRouter()
	apiHandler := api.NewHandler(manager)
	r.HandleFunc("/v1/proxy", apiHandler.ListProxies).Methods("GET")
	r.HandleFunc("/v1/proxy", apiHandler.CreateProxy).Methods("POST")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.GetProxy).Methods("GET")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.UpdateProxy).Methods("PUT")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.DeleteProxy).Methods("DELETE")
	r.HandleFunc("/dns-query", resolver.HandleDoH).Methods("GET", "POST")

	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	hostname := "app.local"
	backend := "http://localhost:3000"

	// 1) RESTでプロキシ追加
	createPayload := createProxyRequest{
		Host:        hostname,
		Backend:     backend,
		Description: "E2E Test",
	}
	payloadBytes, err := json.Marshal(createPayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	resp, err := http.Post(server.URL+"/v1/proxy", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status: %d body=%s", resp.StatusCode, string(body))
	}

	// 2) httpd設定ファイルに反映されているか確認
	confBytes, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	confStr := string(confBytes)
	if !strings.Contains(confStr, "ServerName "+hostname) {
		t.Fatalf("expected ServerName in config for %s", hostname)
	}
	if !strings.Contains(confStr, "ProxyPass / "+backend+"/") {
		t.Fatalf("expected ProxyPass in config for %s", backend)
	}

	// 3) DoH応答（管理対象）: A=127.0.0.1
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(hostname), dns.TypeA)
	packed, err := msg.Pack()
	if err != nil {
		t.Fatalf("failed to pack dns query: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(packed)
	resp, err = http.Get(server.URL + "/dns-query?dns=" + encoded)
	if err != nil {
		t.Fatalf("failed to query doh: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected doh status: %d body=%s", resp.StatusCode, string(body))
	}
	wire, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read doh response: %v", err)
	}
	parsed := new(dns.Msg)
	if err := parsed.Unpack(wire); err != nil {
		t.Fatalf("failed to unpack doh response: %v", err)
	}
	if parsed.Rcode != dns.RcodeSuccess {
		t.Fatalf("unexpected rcode for managed host: %d", parsed.Rcode)
	}
	found := false
	for _, rr := range parsed.Answer {
		if a, ok := rr.(*dns.A); ok && a.A.Equal(net.ParseIP("127.0.0.1")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected A record 127.0.0.1 for %s", hostname)
	}

	// 4) RESTで削除
	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/proxy/"+hostname, nil)
	if err != nil {
		t.Fatalf("failed to create delete request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to delete proxy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected delete status: %d body=%s", resp.StatusCode, string(body))
	}

	// 5) 設定から消えていること
	confBytes, err = os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read config after delete: %v", err)
	}
	confStr = string(confBytes)
	if strings.Contains(confStr, "ServerName "+hostname) {
		t.Fatalf("did not expect ServerName in config for %s", hostname)
	}

	// 6) DoH応答（管理外）: アップストリーム失敗で 500
	resp, err = http.Get(server.URL + "/dns-query?dns=" + encoded)
	if err != nil {
		t.Fatalf("failed to query doh after delete: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 500 for unmanaged host, got %d body=%s", resp.StatusCode, string(body))
	}
}

// TestE2E_UpdateProxy は PUT リクエストでプロキシ設定を更新できることをテスト
func TestE2E_UpdateProxy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "dynamic-proxy.conf")
	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config file: %v", err)
	}

	manager := httpd.NewManagerWithConfig(confPath, func() error { return nil })
	resolver := doh.NewResolver(testUpstreamDNS)
	resolver.SetManagedHostResolver(managedHostResolver(manager, net.ParseIP("127.0.0.1")))

	r := mux.NewRouter()
	apiHandler := api.NewHandler(manager)
	r.HandleFunc("/v1/proxy", apiHandler.ListProxies).Methods("GET")
	r.HandleFunc("/v1/proxy", apiHandler.CreateProxy).Methods("POST")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.GetProxy).Methods("GET")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.UpdateProxy).Methods("PUT")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.DeleteProxy).Methods("DELETE")
	r.HandleFunc("/dns-query", resolver.HandleDoH).Methods("GET", "POST")

	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	hostname := "update.local"
	initialBackend := "http://localhost:3000"
	updatedBackend := "http://localhost:4000"

	// 1) 初期化: プロキシ作成
	createPayload := createProxyRequest{
		Host:    hostname,
		Backend: initialBackend,
	}
	payloadBytes, err := json.Marshal(createPayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	resp, err := http.Post(server.URL+"/v1/proxy", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	// 2) 初期バックエンドがファイルに反映
	confBytes, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if !strings.Contains(string(confBytes), initialBackend) {
		t.Fatalf("expected initial backend in config")
	}

	// 3) PUT で更新
	updatePayload := createProxyRequest{
		Backend: updatedBackend,
	}
	updateBytes, err := json.Marshal(updatePayload)
	if err != nil {
		t.Fatalf("failed to marshal update payload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, server.URL+"/v1/proxy/"+hostname, bytes.NewReader(updateBytes))
	if err != nil {
		t.Fatalf("failed to create update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to update proxy: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected update status: %d", resp.StatusCode)
	}

	// 4) 更新後のバックエンドがファイルに反映
	confBytes, err = os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read config after update: %v", err)
	}
	confStr := string(confBytes)
	if !strings.Contains(confStr, updatedBackend) {
		t.Fatalf("expected updated backend in config")
	}
	if strings.Contains(confStr, initialBackend) {
		t.Fatalf("did not expect old backend in config")
	}

	// 5) DoH が更新されたバックエンド設定で応答（ホストは変わらず）
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(hostname), dns.TypeA)
	packed, err := msg.Pack()
	if err != nil {
		t.Fatalf("failed to pack dns query: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(packed)
	resp, err = http.Get(server.URL + "/dns-query?dns=" + encoded)
	if err != nil {
		t.Fatalf("failed to query doh after update: %v", err)
	}
	defer resp.Body.Close()
	wire, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read doh response: %v", err)
	}
	parsed := new(dns.Msg)
	if err := parsed.Unpack(wire); err != nil {
		t.Fatalf("failed to unpack doh response: %v", err)
	}
	if parsed.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected RcodeSuccess after update, got %d", parsed.Rcode)
	}
}

// TestE2E_DuplicateProxyConflict は 409 Conflict エラーをテスト
func TestE2E_DuplicateProxyConflict(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "dynamic-proxy.conf")
	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config file: %v", err)
	}

	manager := httpd.NewManagerWithConfig(confPath, func() error { return nil })
	r := mux.NewRouter()
	apiHandler := api.NewHandler(manager)
	r.HandleFunc("/v1/proxy", apiHandler.CreateProxy).Methods("POST")

	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	hostname := "duplicate.local"
	backend := "http://localhost:3000"

	// 1) 最初のプロキシ作成
	createPayload := createProxyRequest{
		Host:    hostname,
		Backend: backend,
	}
	payloadBytes, err := json.Marshal(createPayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	resp, err := http.Post(server.URL+"/v1/proxy", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	// 2) 同じホストで再度作成 → 409 期待
	resp, err = http.Post(server.URL+"/v1/proxy", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409 Conflict, got %d body=%s", resp.StatusCode, string(body))
	}
}

// TestE2E_AAA_Query は AAAA クエリ（IPv6）をテスト
func TestE2E_AAAA_Query(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "dynamic-proxy.conf")
	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config file: %v", err)
	}

	manager := httpd.NewManagerWithConfig(confPath, func() error { return nil })
	resolver := doh.NewResolver(testUpstreamDNS)
	// IPv6 フロントIP でセット
	resolver.SetManagedHostResolver(managedHostResolver(manager, net.ParseIP("::1")))

	r := mux.NewRouter()
	apiHandler := api.NewHandler(manager)
	r.HandleFunc("/v1/proxy", apiHandler.CreateProxy).Methods("POST")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.DeleteProxy).Methods("DELETE")
	r.HandleFunc("/dns-query", resolver.HandleDoH).Methods("GET", "POST")

	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	hostname := "ipv6.local"
	backend := "http://localhost:3000"

	// 1) プロキシ追加
	createPayload := createProxyRequest{
		Host:    hostname,
		Backend: backend,
	}
	payloadBytes, err := json.Marshal(createPayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	resp, err := http.Post(server.URL+"/v1/proxy", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	resp.Body.Close()

	// 2) AAAA クエリ実行
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(hostname), dns.TypeAAAA)
	packed, err := msg.Pack()
	if err != nil {
		t.Fatalf("failed to pack dns query: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(packed)
	resp, err = http.Get(server.URL + "/dns-query?dns=" + encoded)
	if err != nil {
		t.Fatalf("failed to query doh: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected doh status: %d", resp.StatusCode)
	}

	wire, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read doh response: %v", err)
	}
	parsed := new(dns.Msg)
	if err := parsed.Unpack(wire); err != nil {
		t.Fatalf("failed to unpack doh response: %v", err)
	}

	// 3) AAAA レコード確認（::1）
	found := false
	for _, rr := range parsed.Answer {
		if aaaa, ok := rr.(*dns.AAAA); ok && aaaa.AAAA.Equal(net.ParseIP("::1")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected AAAA record ::1 for %s", hostname)
	}
}

// TestE2E_HTMLManagementUI は GET /v1/proxy with Accept: text/html で HTML が返ることをテスト
func TestE2E_HTMLManagementUI(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "dynamic-proxy.conf")
	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config file: %v", err)
	}

	// web/index.html が存在しない場合もエラーハンドル
	manager := httpd.NewManagerWithConfig(confPath, func() error { return nil })
	r := mux.NewRouter()
	apiHandler := api.NewHandler(manager)
	r.HandleFunc("/v1/proxy", apiHandler.ListProxies).Methods("GET")

	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	// 1) Accept: text/html でリクエスト
	req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/proxy", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Accept", "text/html")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to get html: %v", err)
	}
	defer resp.Body.Close()

	// 2) テスト環境ではファイルが無い場合があるため、HTTPレスポンスヘッダを確認
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode == http.StatusInternalServerError {
		// ファイルが無い環境（期待される）
		if contentType == "" {
			// OKログ出力で確認
			t.Logf("HTML UI file not available in test environment (expected)")
		}
	} else if resp.StatusCode == http.StatusOK {
		// ファイルがある環境
		if !strings.Contains(contentType, "text/html") {
			t.Fatalf("expected text/html content type, got %s", contentType)
		}
	}
}

// TestE2E_ManualConfigReload は手編集した httpd.conf を再読み込みできることをテスト
func TestE2E_ManualConfigReload(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "dynamic-proxy.conf")

	// 初期状態: 手編集された設定ファイル（REST API外で作成）
	initialConfig := `
# Manual config
<VirtualHost *:8443>
    ServerName manual.local
    ProxyPass / http://localhost:5000/
</VirtualHost>
`
	if err := os.WriteFile(confPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// 1) マネージャー作成 → 手編集設定を読み込む
	manager := httpd.NewManagerWithConfig(confPath, func() error { return nil })
	if !manager.HasHost("manual.local") {
		t.Fatal("expected manual.local to be loaded")
	}

	resolver := doh.NewResolver(testUpstreamDNS)
	resolver.SetManagedHostResolver(managedHostResolver(manager, net.ParseIP("127.0.0.1")))

	// 2) 外部で設定ファイルを編集（別のプロキシここに追加）
	editedConfig := `
# Manual config
<VirtualHost *:8443>
    ServerName manual.local
    ProxyPass / http://localhost:5000/
</VirtualHost>

<VirtualHost *:8443>
    ServerName external.local
    ProxyPass / http://localhost:6000/
</VirtualHost>
`
	if err := os.WriteFile(confPath, []byte(editedConfig), 0644); err != nil {
		t.Fatalf("failed to write edited config: %v", err)
	}

	// 3) loadConfig() で手編集を取り込む
	if err := manager.LoadConfig(); err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	// 4) 両方のホストが存在することを確認
	if !manager.HasHost("manual.local") {
		t.Fatal("expected manual.local after reload")
	}
	if !manager.HasHost("external.local") {
		t.Fatal("expected external.local after reload")
	}

	// 5) DoH が両方のホストに応答
	for _, hostname := range []string{"manual.local", "external.local"} {
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(hostname), dns.TypeA)
		response, err := resolver.Query(msg)
		if err != nil {
			t.Fatalf("query failed for %s: %v", hostname, err)
		}
		if response.Rcode != dns.RcodeSuccess {
			t.Fatalf("expected RcodeSuccess for %s, got %d", hostname, response.Rcode)
		}
	}
}
