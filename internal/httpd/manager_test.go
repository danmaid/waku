package httpd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewManagerWithConfig は設定ファイルが存在しない場合の初期化をテスト
func TestNewManagerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	manager := NewManagerWithConfig(confPath, func() error { return nil })

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(manager.ListProxies()) != 0 {
		t.Fatalf("expected empty proxies, got %d", len(manager.ListProxies()))
	}
}

// TestLoadConfigWithVirtualHosts はVirtualHost解析をテスト
func TestLoadConfigWithVirtualHosts(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	// テスト用httpd設定
	configContent := `
# Comment describing example.local
<VirtualHost *:8443>
    ServerName example.local
    ProxyPass / http://localhost:3000/
	Header always set Content-Security-Policy "frame-ancestors 'self' https://portal.local"
</VirtualHost>

# Another service comment
<VirtualHost *:8443>
    ServerName api.local
    ProxyPass / http://localhost:4000/
</VirtualHost>
`
	if err := os.WriteFile(confPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	manager := NewManagerWithConfig(confPath, func() error { return nil })

	proxies := manager.ListProxies()
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(proxies))
	}

	// example.local の確認
	config, err := manager.GetProxy("example.local")
	if err != nil {
		t.Fatalf("failed to get example.local: %v", err)
	}
	if config.Backend != "http://localhost:3000" {
		t.Fatalf("expected backend http://localhost:3000, got %s", config.Backend)
	}
	if config.Description != "Comment describing example.local" {
		t.Fatalf("expected description, got %s", config.Description)
	}
	if config.CSPPolicy != "frame-ancestors 'self' https://portal.local" {
		t.Fatalf("expected CSP policy, got %s", config.CSPPolicy)
	}

	// api.local の確認
	config, err = manager.GetProxy("api.local")
	if err != nil {
		t.Fatalf("failed to get api.local: %v", err)
	}
	if config.Backend != "http://localhost:4000" {
		t.Fatalf("expected backend http://localhost:4000, got %s", config.Backend)
	}
}

// TestGenerateHttpdConfig は設定ファイル生成をテスト
func TestGenerateHttpdConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	// 初期化
	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	manager := NewManagerWithConfig(confPath, func() error { return nil })

	// プロキシ追加
	config := &ProxyConfig{
		Host:        "test.local",
		Backend:     "http://backend:5000",
		Description: "Test service",
		CSPPolicy:   "frame-ancestors 'self' https://portal.local",
	}
	if err := manager.AddProxy(config); err != nil {
		t.Fatalf("failed to add proxy: %v", err)
	}

	// ファイル内容確認
	content, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	confStr := string(content)

	if !strings.Contains(confStr, "ServerName test.local") {
		t.Fatal("expected ServerName in config")
	}
	if !strings.Contains(confStr, "ProxyPass / http://backend:5000/") {
		t.Fatal("expected ProxyPass in config")
	}
	if !strings.Contains(confStr, "<VirtualHost *:8443>") {
		t.Fatal("expected HTTPS VirtualHost in config")
	}
	if !strings.Contains(confStr, "SSLEngine on") {
		t.Fatal("expected SSL configuration in config")
	}
	if !strings.Contains(confStr, "Test service") {
		t.Fatal("expected description in config")
	}
	if !strings.Contains(confStr, "Content-Security-Policy") {
		t.Fatal("expected CSP header in config")
	}
}

// TestAddProxyGeneratesConfig は AddProxy で設定ファイルに反映されることをテスト
func TestAddProxyGeneratesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	manager := NewManagerWithConfig(confPath, func() error { return nil })

	config := &ProxyConfig{
		Host:    "app1.local",
		Backend: "http://localhost:3001",
	}
	if err := manager.AddProxy(config); err != nil {
		t.Fatalf("failed to add proxy: %v", err)
	}

	// メモリに読み込まれたか確認
	if !manager.HasHost("app1.local") {
		t.Fatal("expected HasHost to return true")
	}

	// ファイルに書き込まれたか確認
	content, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if !strings.Contains(string(content), "app1.local") {
		t.Fatal("expected app1.local in config file")
	}
}

// TestRemoveProxyGeneratesConfig は RemoveProxy で設定ファイルから削除されることをテスト
func TestRemoveProxyGeneratesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	manager := NewManagerWithConfig(confPath, func() error { return nil })

	// 追加してから削除
	config := &ProxyConfig{
		Host:    "temp.local",
		Backend: "http://localhost:3000",
	}
	if err := manager.AddProxy(config); err != nil {
		t.Fatalf("failed to add proxy: %v", err)
	}

	if err := manager.RemoveProxy("temp.local"); err != nil {
		t.Fatalf("failed to remove proxy: %v", err)
	}

	// メモリから削除されたか確認
	if manager.HasHost("temp.local") {
		t.Fatal("expected HasHost to return false after deletion")
	}

	// ファイルから削除されたか確認
	content, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if strings.Contains(string(content), "temp.local") {
		t.Fatal("did not expect temp.local in config file after deletion")
	}
}

// TestHasHost はホスト判定をテスト
func TestHasHost(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	manager := NewManagerWithConfig(confPath, func() error { return nil })

	// 初期状態
	if manager.HasHost("nonexistent.local") {
		t.Fatal("expected HasHost to return false for nonexistent host")
	}

	// 追加後
	config := &ProxyConfig{
		Host:    "exists.local",
		Backend: "http://localhost:3000",
	}
	if err := manager.AddProxy(config); err != nil {
		t.Fatalf("failed to add proxy: %v", err)
	}

	if !manager.HasHost("exists.local") {
		t.Fatal("expected HasHost to return true after adding")
	}
}

// TestMultipleProxies は複数プロキシの管理をテスト
func TestMultipleProxies(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	if err := os.WriteFile(confPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	manager := NewManagerWithConfig(confPath, func() error { return nil })

	// 複数追加
	hosts := []string{"app1.local", "app2.local", "app3.local"}
	for i, host := range hosts {
		config := &ProxyConfig{
			Host:    host,
			Backend: "http://localhost:" + string(rune(3000+i)),
		}
		if err := manager.AddProxy(config); err != nil {
			t.Fatalf("failed to add proxy %s: %v", host, err)
		}
	}

	// 全て存在することを確認
	for _, host := range hosts {
		if !manager.HasHost(host) {
			t.Fatalf("expected %s to exist", host)
		}
	}

	// リスト確認
	proxies := manager.ListProxies()
	if len(proxies) != 3 {
		t.Fatalf("expected 3 proxies, got %d", len(proxies))
	}

	// 1つ削除
	if err := manager.RemoveProxy("app2.local"); err != nil {
		t.Fatalf("failed to remove proxy: %v", err)
	}

	// 削除確認
	if manager.HasHost("app2.local") {
		t.Fatal("expected app2.local to be removed")
	}
	if !manager.HasHost("app1.local") || !manager.HasHost("app3.local") {
		t.Fatal("expected other hosts to still exist")
	}
}

// TestSanitizeHost はホスト名のサニタイズをテスト
func TestSanitizeHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example_com"},
		{"sub.example.com", "sub_example_com"},
		{"example.com:8443", "example_com"},
		{"api.service", "api_service"},
	}

	for _, tt := range tests {
		result := sanitizeHost(tt.input)
		if result != tt.expected {
			t.Fatalf("sanitizeHost(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

// TestConfigReload は設定ファイル再読み込みをテスト
func TestConfigReload(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")

	// 初期設定ファイル
	initialConfig := `
<VirtualHost *:8443>
    ServerName initial.local
    ProxyPass / http://localhost:3000/
</VirtualHost>
`
	if err := os.WriteFile(confPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// マネージャー作成
	manager := NewManagerWithConfig(confPath, func() error { return nil })

	// 初期状態確認
	if !manager.HasHost("initial.local") {
		t.Fatal("expected initial.local to be loaded")
	}

	// 外部で設定ファイルを編集（例：手動編集）
	newConfig := `
<VirtualHost *:8443>
    ServerName updated.local
    ProxyPass / http://localhost:4000/
</VirtualHost>
`
	if err := os.WriteFile(confPath, []byte(newConfig), 0644); err != nil {
		t.Fatalf("failed to write new config: %v", err)
	}

	// 手動で LoadConfig() を呼び出し
	if err := manager.LoadConfig(); err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	// 新しい設定が読み込まれたか確認
	if manager.HasHost("initial.local") {
		t.Fatal("did not expect initial.local after reload")
	}
	if !manager.HasHost("updated.local") {
		t.Fatal("expected updated.local after reload")
	}
}
