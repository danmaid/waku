package httpd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	tlsmgr "github.com/danmaid/dynamic-proxy/internal/tls"
)

// ProxyConfig は単一のプロキシ設定
type ProxyConfig struct {
	Host        string `json:"host"`        // バーチャルホスト名
	Backend     string `json:"backend"`     // バックエンドURL
	Description string `json:"description"` // 説明
	DNSAddress  string `json:"dns_address"` // DNS応答で返すIPアドレス
	CSPPolicy   string `json:"csp"`         // Content-Security-Policy 上書き
}

// Manager はhttpd設定管理マネージャー
type Manager struct {
	configs    map[string]*ProxyConfig
	mu         sync.RWMutex
	httpdConf  string // httpd設定ファイル（単一の真実の源）
	reloadFunc func() error
	tlsManager *tlsmgr.Manager // TLS証明書マネージャー
}

// NewManager は新しいhttpd設定マネージャーを作成
func NewManager() *Manager {
	return NewManagerWithConfig("/etc/httpd/conf.d/dynamic-proxy.conf", nil)
}

// NewManagerWithConfig はテスト用途などで設定ファイルを指定して作成
func NewManagerWithConfig(httpdConf string, reloadFunc func() error) *Manager {
	m := &Manager{
		configs:    make(map[string]*ProxyConfig),
		httpdConf:  httpdConf,
		reloadFunc: reloadFunc,
	}
	if m.reloadFunc == nil {
		m.reloadFunc = m.reloadHttpd
	}

	// httpd設定ファイルから設定を読み込み
	if err := m.LoadConfig(); err != nil {
		log.Printf("Warning: Failed to load httpd config: %v", err)
		// 設定ファイルが存在しない場合は初期状態で生成
		if os.IsNotExist(err) {
			if err := m.generateHttpdConfig(); err != nil {
				log.Printf("Warning: Failed to generate initial httpd config: %v", err)
			}
		}
	}

	return m
}

// SetTLSManager はTLS証明書マネージャーを設定
func (m *Manager) SetTLSManager(tlsManager *tlsmgr.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tlsManager = tlsManager

	// 既存のすべてのホストに対して証明書を生成
	for host := range m.configs {
		if err := tlsManager.GenerateServerCert(host); err != nil {
			log.Printf("Warning: Failed to generate certificate for %s: %v", host, err)
		}
	}
}

// AddProxy はプロキシ設定を追加し、httpd設定を更新
func (m *Manager) AddProxy(config *ProxyConfig) error {
	m.mu.Lock()
	m.configs[config.Host] = config
	m.mu.Unlock()

	// TLS証明書を生成
	if m.tlsManager != nil {
		if err := m.tlsManager.GenerateServerCert(config.Host); err != nil {
			log.Printf("Warning: Failed to generate certificate for %s: %v", config.Host, err)
		}
	}

	// httpd設定を生成（単一の真実の源）
	if err := m.generateHttpdConfig(); err != nil {
		return fmt.Errorf("failed to generate httpd config: %w", err)
	}

	// httpdをリロード
	if err := m.reloadFunc(); err != nil {
		log.Printf("Warning: Failed to reload httpd: %v", err)
	}

	log.Printf("Added proxy: %s -> %s", config.Host, config.Backend)
	return nil
}

// RemoveProxy はプロキシ設定を削除し、httpd設定を更新
func (m *Manager) RemoveProxy(host string) error {
	m.mu.Lock()
	if _, ok := m.configs[host]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("proxy not found: %s", host)
	}
	delete(m.configs, host)
	m.mu.Unlock()

	// TLS証明書を削除
	if m.tlsManager != nil {
		if err := m.tlsManager.DeleteServerCert(host); err != nil {
			log.Printf("Warning: Failed to delete certificate for %s: %v", host, err)
		}
	}

	// httpd設定を生成（単一の真実の源）
	if err := m.generateHttpdConfig(); err != nil {
		return fmt.Errorf("failed to generate httpd config: %w", err)
	}

	// httpdをリロード
	if err := m.reloadFunc(); err != nil {
		log.Printf("Warning: Failed to reload httpd: %v", err)
	}

	log.Printf("Removed proxy: %s", host)
	return nil
}

// GetProxy は指定されたホストのプロキシ設定を取得
func (m *Manager) GetProxy(host string) (*ProxyConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, ok := m.configs[host]
	if !ok {
		return nil, fmt.Errorf("proxy not found: %s", host)
	}

	return config, nil
}

// HasHost は指定ホストが管理対象かを判定
func (m *Manager) HasHost(host string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.configs[host]
	return ok
}

// GetDNSAddress は指定ホストのDNS応答IPアドレスを取得
func (m *Manager) GetDNSAddress(host string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, ok := m.configs[host]
	if !ok || config.DNSAddress == "" {
		return "", false
	}
	return config.DNSAddress, true
}

func (m *Manager) ListProxies() []*ProxyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]*ProxyConfig, 0, len(m.configs))
	for _, config := range m.configs {
		configs = append(configs, config)
	}

	return configs
}

// LoadConfig はhttpd設定ファイルをパースして設定を読み込み
func (m *Manager) LoadConfig() error {
	data, err := os.ReadFile(m.httpdConf)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 既存の設定をクリア
	m.configs = make(map[string]*ProxyConfig)

	// httpd設定ファイルをパース
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var currentVHost *ProxyConfig
	var inVHost bool
	var lastComment string

	reServerName := regexp.MustCompile(`^\s*ServerName\s+(\S+)`)
	reProxyPass := regexp.MustCompile(`^\s*ProxyPass\s+/\s+(\S+)`)
	reComment := regexp.MustCompile(`^\s*#\s*(.*)`)
	reDNS := regexp.MustCompile(`^\s*#\s*DNS:\s*(\S+)`)
	reCSP := regexp.MustCompile(`^\s*Header\s+(?:always\s+)?set\s+Content-Security-Policy\s+"([^"]*)"`)

	for scanner.Scan() {
		line := scanner.Text()

		// DNS設定行
		if matches := reDNS.FindStringSubmatch(line); matches != nil {
			if inVHost && currentVHost != nil {
				currentVHost.DNSAddress = matches[1]
			}
			continue
		}

		// コメント行（description候補）
		if matches := reComment.FindStringSubmatch(line); matches != nil {
			comment := strings.TrimSpace(matches[1])
			// 自動生成コメントは無視
			if !strings.HasPrefix(comment, "Auto-generated") &&
				!strings.HasPrefix(comment, "Managed by") &&
				!strings.HasPrefix(comment, "DNS:") &&
				comment != "" {
				lastComment = comment
			}
			continue
		}

		// VirtualHost開始
		if strings.Contains(line, "<VirtualHost") {
			inVHost = true
			currentVHost = &ProxyConfig{
				Description: lastComment,
			}
			lastComment = ""
			continue
		}

		// VirtualHost終了
		if strings.Contains(line, "</VirtualHost>") {
			if currentVHost != nil && currentVHost.Host != "" && currentVHost.Backend != "" {
				m.configs[currentVHost.Host] = currentVHost
			}
			inVHost = false
			currentVHost = nil
			continue
		}

		// VirtualHost内の設定
		if inVHost && currentVHost != nil {
			// ServerName
			if matches := reServerName.FindStringSubmatch(line); matches != nil {
				currentVHost.Host = matches[1]
			}
			// ProxyPass（バックエンドURL）
			if matches := reProxyPass.FindStringSubmatch(line); matches != nil {
				backend := matches[1]
				// 末尾の / を削除
				currentVHost.Backend = strings.TrimSuffix(backend, "/")
			}
			// Content-Security-Policy
			if matches := reCSP.FindStringSubmatch(line); matches != nil {
				currentVHost.CSPPolicy = unescapeCSPPolicy(matches[1])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading httpd config: %w", err)
	}

	log.Printf("Loaded %d proxy configurations from %s", len(m.configs), m.httpdConf)
	return nil
}

// generateHttpdConfig はApache httpd設定ファイルを生成
func (m *Manager) generateHttpdConfig() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("# Apache httpd configuration for Dynamic Proxy (HTTPS)\n")
	sb.WriteString("# This file can be edited manually by administrators\n")
	sb.WriteString("# Changes will be read on next application startup or reload\n")
	sb.WriteString("# NOTE: Update SSLCertificateFile/Key if needed.\n\n")

	// 各プロキシ設定のVirtualHostを生成
	for _, config := range m.configs {
		if config.Description != "" {
			sb.WriteString(fmt.Sprintf("# %s\n", config.Description))
		}
		if config.DNSAddress != "" {
			sb.WriteString(fmt.Sprintf("# DNS: %s\n", config.DNSAddress))
		}
		sb.WriteString("<VirtualHost *:8443>\n")
		sb.WriteString(fmt.Sprintf("    ServerName %s\n", config.Host))
		sb.WriteString("    \n")
		sb.WriteString("    SSLEngine on\n")

		// ホストごとの証明書を使用（TLSマネージャーが設定されている場合）
		if m.tlsManager != nil {
			sb.WriteString(fmt.Sprintf("    SSLCertificateFile %s\n", m.tlsManager.GetCertPath(config.Host)))
			sb.WriteString(fmt.Sprintf("    SSLCertificateKeyFile %s\n", m.tlsManager.GetKeyPath(config.Host)))
		} else {
			// フォールバック：デフォルトの証明書
			sb.WriteString("    SSLCertificateFile /etc/httpd/tls/localhost.crt\n")
			sb.WriteString("    SSLCertificateKeyFile /etc/httpd/tls/localhost.key\n")
		}

		sb.WriteString("    \n")
		sb.WriteString("    # HTTPSバックエンド対応\n")
		sb.WriteString("    SSLProxyEngine On\n")
		sb.WriteString("    SSLProxyVerify none\n")
		sb.WriteString("    SSLProxyCheckPeerCN off\n")
		sb.WriteString("    SSLProxyCheckPeerName off\n")
		sb.WriteString("    SSLProxyProtocol all -SSLv3\n")
		sb.WriteString("    \n")
		if strings.TrimSpace(config.CSPPolicy) != "" {
			policy := escapeCSPPolicy(strings.TrimSpace(config.CSPPolicy))
			sb.WriteString("    # CSP override for iframe integration\n")
			sb.WriteString("    <Location />\n")
			sb.WriteString("        Header unset X-Frame-Options\n")
			sb.WriteString("        Header unset Content-Security-Policy\n")
			sb.WriteString(fmt.Sprintf("        Header set Content-Security-Policy \"%s\"\n", policy))
			sb.WriteString("    </Location>\n")
			sb.WriteString("    \n")
		}
		sb.WriteString("    ProxyPreserveHost Off\n")
		sb.WriteString(fmt.Sprintf("    ProxyPass / %s/ disablereuse=on\n", config.Backend))
		sb.WriteString(fmt.Sprintf("    ProxyPassReverse / %s/\n", config.Backend))
		sb.WriteString("    \n")
		sb.WriteString(fmt.Sprintf("    ErrorLog /var/log/httpd/%s_error.log\n", sanitizeHost(config.Host)))
		sb.WriteString(fmt.Sprintf("    CustomLog /var/log/httpd/%s_access.log combined\n", sanitizeHost(config.Host)))
		sb.WriteString("</VirtualHost>\n\n")
	}

	// ファイルに書き込み
	configContent := []byte(sb.String())
	if err := os.WriteFile(m.httpdConf, configContent, 0644); err != nil {
		return fmt.Errorf("failed to write httpd config: %w", err)
	}

	// 設定をテスト
	if err := m.testHttpdConfig(); err != nil {
		// テスト失敗時は設定ファイルを削除
		os.Remove(m.httpdConf)
		return fmt.Errorf("httpd config validation failed: %w", err)
	}

	return nil
}

// testHttpdConfig はhttpd -tで設定をテスト
func (m *Manager) testHttpdConfig() error {
	cmd := exec.Command("httpd", "-t")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("httpd config test failed: %s", string(output))
	}

	// 成功時でもwarningがある場合がある
	if strings.Contains(string(output), "Syntax error") || strings.Contains(string(output), "Error") {
		return fmt.Errorf("httpd config error: %s", string(output))
	}

	log.Printf("[httpd] Configuration test passed")
	return nil
}

// reloadHttpd はApache httpdをリロード
func (m *Manager) reloadHttpd() error {
	// 設定ファイルの構文チェックは generateHttpdConfig() で既に済み

	// 使用可能なリロードコマンドをリストから試す
	commands := [][]string{
		{"apachectl", "graceful"},
		{"systemctl", "reload", "httpd"},
		{"httpd", "-k", "graceful"},
	}

	var lastErr error
	for _, cmd := range commands {
		// コマンドが存在するか確認
		if _, err := exec.LookPath(cmd[0]); err != nil {
			log.Printf("[httpd] %s not available", cmd[0])
			continue
		}

		// コマンド実行
		output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("%s failed: %w (output: %s)", strings.Join(cmd, " "), err, string(output))
			log.Printf("[httpd] %v", lastErr)
			continue
		}

		log.Printf("[httpd] Successfully reloaded using: %s", strings.Join(cmd, " "))
		return nil
	}

	// すべての方法が失敗
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no suitable method found to reload httpd")
}

// GetVirtualHostConfig はホストのVirtualHost設定を取得
func (m *Manager) GetVirtualHostConfig(host string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, ok := m.configs[host]
	if !ok {
		return "", fmt.Errorf("proxy not found: %s", host)
	}

	// VirtualHost設定文字列を生成
	var sb strings.Builder

	if config.Description != "" {
		sb.WriteString(fmt.Sprintf("# %s\n", config.Description))
	}
	if config.DNSAddress != "" {
		sb.WriteString(fmt.Sprintf("# DNS: %s\n", config.DNSAddress))
	}
	sb.WriteString("<VirtualHost *:8443>\n")
	sb.WriteString(fmt.Sprintf("    ServerName %s\n", config.Host))
	sb.WriteString("    \n")
	sb.WriteString("    SSLEngine on\n")

	// ホストごとの証明書を使用（TLSマネージャーが設定されている場合）
	if m.tlsManager != nil {
		sb.WriteString(fmt.Sprintf("    SSLCertificateFile %s\n", m.tlsManager.GetCertPath(config.Host)))
		sb.WriteString(fmt.Sprintf("    SSLCertificateKeyFile %s\n", m.tlsManager.GetKeyPath(config.Host)))
	} else {
		// フォールバック：デフォルトの証明書
		sb.WriteString("    SSLCertificateFile /etc/httpd/tls/localhost.crt\n")
		sb.WriteString("    SSLCertificateKeyFile /etc/httpd/tls/localhost.key\n")
	}

	sb.WriteString("    \n")
	sb.WriteString("    # HTTPSバックエンド対応\n")
	sb.WriteString("    SSLProxyEngine On\n")
	sb.WriteString("    SSLProxyVerify none\n")
	sb.WriteString("    SSLProxyCheckPeerCN off\n")
	sb.WriteString("    SSLProxyCheckPeerName off\n")
	sb.WriteString("    SSLProxyProtocol all -SSLv3\n")
	sb.WriteString("    \n")
	if strings.TrimSpace(config.CSPPolicy) != "" {
		policy := escapeCSPPolicy(strings.TrimSpace(config.CSPPolicy))
		sb.WriteString("    # CSP override for iframe integration\n")
		sb.WriteString("    <Location />\n")
		sb.WriteString("        Header unset X-Frame-Options\n")
		sb.WriteString("        Header unset Content-Security-Policy\n")
		sb.WriteString(fmt.Sprintf("        Header set Content-Security-Policy \"%s\"\n", policy))
		sb.WriteString("    </Location>\n")
		sb.WriteString("    \n")
	}
	sb.WriteString("    ProxyPreserveHost Off\n")
	sb.WriteString(fmt.Sprintf("    ProxyPass / %s/ disablereuse=on\n", config.Backend))
	sb.WriteString(fmt.Sprintf("    ProxyPassReverse / %s/\n", config.Backend))
	sb.WriteString("    \n")
	sb.WriteString(fmt.Sprintf("    ErrorLog /var/log/httpd/%s_error.log\n", sanitizeHost(config.Host)))
	sb.WriteString(fmt.Sprintf("    CustomLog /var/log/httpd/%s_access.log combined\n", sanitizeHost(config.Host)))
	sb.WriteString("</VirtualHost>")

	return sb.String(), nil
}

// UpdateVirtualHostConfig はホストのVirtualHost設定を更新
// ユーザー入力から必要な情報を抽出して設定を更新し、全体を再生成
func (m *Manager) UpdateVirtualHostConfig(host string, configText string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// ホストが存在するか確認
	config, ok := m.configs[host]
	if !ok {
		return fmt.Errorf("proxy not found: %s", host)
	}

	// 基本的なバリデーション
	if !strings.Contains(configText, fmt.Sprintf("ServerName %s", host)) {
		return fmt.Errorf("ServerName must match the host: %s", host)
	}
	if !strings.Contains(configText, config.Backend) && !strings.Contains(configText, "ProxyPass") {
		return fmt.Errorf("invalid VirtualHost configuration: missing ProxyPass")
	}

	// ユーザー入力からバックエンドとDNS設定を抽出
	// ProxyPass行からバックエンドを抽出
	proxyPassRe := regexp.MustCompile(`ProxyPass\s+/\s+(\S+)`)
	matches := proxyPassRe.FindStringSubmatch(configText)
	if len(matches) < 2 {
		return fmt.Errorf("could not extract backend from ProxyPass directive")
	}
	backend := strings.TrimSuffix(matches[1], "/")

	// DNS設定があれば抽出
	dnsRe := regexp.MustCompile(`#\s*DNS:\s*(\S+)`)
	dnsMatches := dnsRe.FindStringSubmatch(configText)
	dnsAddr := ""
	if len(dnsMatches) > 1 {
		dnsAddr = dnsMatches[1]
	}
	// CSP設定があれば抽出
	cspRe := regexp.MustCompile(`Header\s+(?:always\s+)?set\s+Content-Security-Policy\s+"([^"]*)"`)
	cspMatches := cspRe.FindStringSubmatch(configText)
	cspPolicy := ""
	if len(cspMatches) > 1 {
		cspPolicy = unescapeCSPPolicy(strings.TrimSpace(cspMatches[1]))
	}

	// メモリ上の設定を更新
	config.Backend = backend
	config.DNSAddress = dnsAddr
	config.CSPPolicy = cspPolicy
	// Description は詳細編集では変更できないので、既存値を保持

	// 全体の設定を再生成
	m.mu.Unlock()
	err := m.generateHttpdConfig()
	m.mu.Lock()

	if err != nil {
		return fmt.Errorf("failed to regenerate config: %w", err)
	}

	// 設定をテスト
	if err := m.testHttpdConfig(); err != nil {
		// テスト失敗時は元の設定に戻す必要があるため、LoadConfig で再度読み込み
		m.mu.Unlock()
		m.LoadConfig()
		m.mu.Lock()
		return fmt.Errorf("httpd config validation failed: %w", err)
	}

	return nil
}

// sanitizeHost はホスト名をファイル名として安全な形式に変換
func sanitizeHost(host string) string {
	// ポート番号を削除
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// ドットをアンダースコアに変換
	return strings.ReplaceAll(host, ".", "_")
}

func escapeCSPPolicy(policy string) string {
	return strings.ReplaceAll(policy, "\"", "\\\"")
}

func unescapeCSPPolicy(policy string) string {
	return strings.ReplaceAll(policy, "\\\"", "\"")
}
