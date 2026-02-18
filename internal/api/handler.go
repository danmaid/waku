package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danmaid/dynamic-proxy/internal/httpd"
	"github.com/gorilla/mux"
)

// Handler はREST APIハンドラー
type Handler struct {
	httpdManager *httpd.Manager
	webRoot      string
}

// NewHandler は新しいAPIハンドラーを作成
func NewHandler(httpdManager *httpd.Manager) *Handler {
	return &Handler{
		httpdManager: httpdManager,
		webRoot:      "web", // HTMLファイルの配置場所
	}
}

// acceptsHTML はリクエストがHTMLを受け入れるかチェック
func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	// ブラウザからのリクエスト（text/html を含む）かチェック
	return strings.Contains(accept, "text/html")
}

// ListProxies は全てのプロキシ設定を返す
// GET /v1/proxy
func (h *Handler) ListProxies(w http.ResponseWriter, r *http.Request) {
	// Acceptヘッダーでブラウザからのリクエストか判定
	if acceptsHTML(r) {
		// HTML管理画面を返す
		htmlPath := filepath.Join(h.webRoot, "index.html")
		htmlContent, err := os.ReadFile(htmlPath)
		if err != nil {
			http.Error(w, "Management UI not found", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(htmlContent)
		return
	}

	// JSON APIレスポンス
	configs := h.httpdManager.ListProxies()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"proxies": configs,
		"count":   len(configs),
	})
}

// GetProxy は指定されたホストのプロキシ設定を返す
// GET /v1/proxy/{host}
func (h *Handler) GetProxy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	config, err := h.httpdManager.GetProxy(host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// CreateProxy は新しいプロキシ設定を作成
// POST /v1/proxy
// Body: {"host": "example.com", "backend": "http://localhost:3000", "description": "Example service"}
func (h *Handler) CreateProxy(w http.ResponseWriter, r *http.Request) {
	var config httpd.ProxyConfig

	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 必須フィールドの検証
	if config.Host == "" {
		http.Error(w, "Missing required field: host", http.StatusBadRequest)
		return
	}
	if config.Backend == "" {
		http.Error(w, "Missing required field: backend", http.StatusBadRequest)
		return
	}

	// 既存の設定チェック
	if _, err := h.httpdManager.GetProxy(config.Host); err == nil {
		http.Error(w, "Proxy already exists for this host", http.StatusConflict)
		return
	}

	if err := h.httpdManager.AddProxy(&config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Proxy created successfully",
		"config":  config,
	})
}

// UpdateProxy はプロキシ設定を更新
// PUT /v1/proxy/{host}
// Body: {"backend": "http://localhost:4000", "description": "Updated service"}
func (h *Handler) UpdateProxy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	// 既存の設定を確認
	if _, err := h.httpdManager.GetProxy(host); err != nil {
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}

	var config httpd.ProxyConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// ホスト名を維持
	config.Host = host

	// backendが指定されていない場合はエラー
	if config.Backend == "" {
		http.Error(w, "Missing required field: backend", http.StatusBadRequest)
		return
	}

	// 一旦削除して再追加
	if err := h.httpdManager.RemoveProxy(host); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.httpdManager.AddProxy(&config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Proxy updated successfully",
		"config":  config,
	})
}

// DeleteProxy はプロキシ設定を削除
// DELETE /v1/proxy/{host}
func (h *Handler) DeleteProxy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	if err := h.httpdManager.RemoveProxy(host); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Proxy deleted successfully",
		"host":    host,
	})
}

// GetVirtualHostConfig はホストのVirtualHost設定を取得
// GET /v1/proxy/{host}/config
func (h *Handler) GetVirtualHostConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	config, err := h.httpdManager.GetVirtualHostConfig(host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"host":   host,
		"config": config,
	})
}

// UpdateVirtualHostConfig はホストのVirtualHost設定を更新
// PUT /v1/proxy/{host}/config
// Body: {"config": "<VirtualHost ...>...custom config...</VirtualHost>"}
func (h *Handler) UpdateVirtualHostConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	var request map[string]string
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	configText, ok := request["config"]
	if !ok || configText == "" {
		http.Error(w, "Missing required field: config", http.StatusBadRequest)
		return
	}

	if err := h.httpdManager.UpdateVirtualHostConfig(host, configText); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "VirtualHost configuration updated successfully",
		"host":    host,
	})
}

// AnalyzeBackendCSP はバックエンドをスキャンしてCSP情報を取得
// GET /v1/proxy/{host}/analyze
func (h *Handler) AnalyzeBackendCSP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]

	// 既存の設定を確認
	config, err := h.httpdManager.GetProxy(host)
	if err != nil {
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}

	if config.Backend == "" {
		http.Error(w, "Backend URL not configured", http.StatusBadRequest)
		return
	}

	// HTTPS の場合は証明書検証をスキップ
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// バックエンドにリクエストを送信
	resp, err := client.Head(config.Backend)
	if err != nil {
		// HEAD が失敗した場合は GET を試す
		resp, err = client.Get(config.Backend)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to connect to backend: %v", err), http.StatusBadGateway)
			return
		}
	}
	defer resp.Body.Close()

	// ボディを読み込まない（不要なので）
	if resp.Body != nil {
		io.ReadAll(resp.Body)
	}

	// レスポンスヘッダーから情報を抽出
	cspHeader := resp.Header.Get("Content-Security-Policy")
	xFrameOptions := resp.Header.Get("X-Frame-Options")

	result := map[string]interface{}{
		"host":              host,
		"csp":               cspHeader,
		"x_frame_options":   xFrameOptions,
		"status_code":       resp.StatusCode,
		"headers_available": true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
