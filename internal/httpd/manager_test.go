package httpd

import (
	"path/filepath"
	"testing"
)

func TestManagerInitializationAndEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")
	manager := NewManagerWithConfig(confPath, func() error { return nil })
	configs, err := manager.ParseConfigFile()
	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected empty configs, got %d", len(configs))
	}
}

func TestWriteAndParseConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")
	manager := NewManagerWithConfig(confPath, func() error { return nil })
	cfg := &ProxyConfig{
		Host:        "app.local",
		Backend:     "http://localhost:3000",
		Description: "test app",
		DNSAddress:  "127.0.0.1",
		CSPPolicy:   "frame-ancestors 'self'",
	}
	configs := []*ProxyConfig{cfg}
	if err := manager.WriteConfigFile(configs); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	parsed, err := manager.ParseConfigFile()
	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 config, got %d", len(parsed))
	}
	if parsed[0].Host != "app.local" || parsed[0].Backend != "http://localhost:3000" {
		t.Fatalf("parsed config mismatch: %+v", parsed[0])
	}
}

func TestHasHostAndFindConfigByHost(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")
	manager := NewManagerWithConfig(confPath, func() error { return nil })
	cfg := &ProxyConfig{Host: "api.local", Backend: "http://localhost:4000"}
	configs := []*ProxyConfig{cfg}
	if err := manager.WriteConfigFile(configs); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if !manager.HasHost("api.local") {
		t.Fatal("expected HasHost true for api.local")
	}
	parsed, err := manager.ParseConfigFile()
	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}
	found := findConfigByHost(parsed, "api.local")
	if found == nil || found.Backend != "http://localhost:4000" {
		t.Fatalf("findConfigByHost failed: %+v", found)
	}
	if manager.HasHost("notfound.local") {
		t.Fatal("expected HasHost false for notfound.local")
	}
}
