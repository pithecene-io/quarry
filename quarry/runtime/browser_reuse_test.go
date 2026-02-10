package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pithecene-io/quarry/types"
)

func TestProxyHash_NilProxy(t *testing.T) {
	hash := proxyHash(nil)
	if hash != "" {
		t.Errorf("expected empty string for nil proxy, got %q", hash)
	}
}

func TestProxyHash_Deterministic(t *testing.T) {
	proxy := &types.ProxyEndpoint{
		Protocol: types.ProxyProtocolHTTP,
		Host:     "proxy.example.com",
		Port:     8080,
	}

	h1 := proxyHash(proxy)
	h2 := proxyHash(proxy)
	if h1 != h2 {
		t.Errorf("proxy hash not deterministic: %q != %q", h1, h2)
	}
	if h1 == "" {
		t.Error("expected non-empty hash for non-nil proxy")
	}
	if len(h1) < 10 {
		t.Errorf("hash suspiciously short: %q", h1)
	}
}

func TestProxyHash_DifferentProxiesDifferentHashes(t *testing.T) {
	proxy1 := &types.ProxyEndpoint{
		Protocol: types.ProxyProtocolHTTP,
		Host:     "proxy1.example.com",
		Port:     8080,
	}
	proxy2 := &types.ProxyEndpoint{
		Protocol: types.ProxyProtocolHTTP,
		Host:     "proxy2.example.com",
		Port:     8080,
	}

	h1 := proxyHash(proxy1)
	h2 := proxyHash(proxy2)
	if h1 == h2 {
		t.Errorf("different proxies should have different hashes: %q == %q", h1, h2)
	}
}

func TestProxyHash_IncludesUsername(t *testing.T) {
	user := "admin"
	proxyWithUser := &types.ProxyEndpoint{
		Protocol: types.ProxyProtocolHTTP,
		Host:     "proxy.example.com",
		Port:     8080,
		Username: &user,
	}
	proxyWithoutUser := &types.ProxyEndpoint{
		Protocol: types.ProxyProtocolHTTP,
		Host:     "proxy.example.com",
		Port:     8080,
	}

	h1 := proxyHash(proxyWithUser)
	h2 := proxyHash(proxyWithoutUser)
	if h1 == h2 {
		t.Errorf("proxy with/without username should have different hashes")
	}
}

func TestProxyHash_ExcludesPassword(t *testing.T) {
	user := "admin"
	pass1 := "secret1"
	pass2 := "secret2"

	proxy1 := &types.ProxyEndpoint{
		Protocol: types.ProxyProtocolHTTP,
		Host:     "proxy.example.com",
		Port:     8080,
		Username: &user,
		Password: &pass1,
	}
	proxy2 := &types.ProxyEndpoint{
		Protocol: types.ProxyProtocolHTTP,
		Host:     "proxy.example.com",
		Port:     8080,
		Username: &user,
		Password: &pass2,
	}

	h1 := proxyHash(proxy1)
	h2 := proxyHash(proxy2)
	if h1 != h2 {
		t.Errorf("password should not affect hash: %q != %q", h1, h2)
	}
}

func TestDiscoveryDir_Fallback(t *testing.T) {
	// Unset XDG_RUNTIME_DIR to test fallback
	orig := os.Getenv("XDG_RUNTIME_DIR")
	t.Setenv("XDG_RUNTIME_DIR", "")

	dir, err := discoveryDir()
	if err != nil {
		t.Fatalf("discoveryDir failed: %v", err)
	}

	if dir == "" {
		t.Error("expected non-empty discovery dir")
	}

	// Verify directory was created
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("discovery dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("discovery dir is not a directory")
	}

	// Restore (t.Setenv handles cleanup)
	_ = orig
}

func TestDiscoveryDir_XDG(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpDir)

	dir, err := discoveryDir()
	if err != nil {
		t.Fatalf("discoveryDir failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "quarry")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("discovery dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("discovery dir is not a directory")
	}
}

func TestReadWriteDiscovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "browser.json")

	disc := &BrowserDiscovery{
		WSEndpoint: "ws://127.0.0.1:9222/devtools/browser/abc123",
		PID:        12345,
		ProxyHash:  "sha256:deadbeef",
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	// Write
	if err := writeDiscovery(path, disc); err != nil {
		t.Fatalf("writeDiscovery failed: %v", err)
	}

	// Read back
	got, err := readDiscovery(path)
	if err != nil {
		t.Fatalf("readDiscovery failed: %v", err)
	}

	if got.WSEndpoint != disc.WSEndpoint {
		t.Errorf("WSEndpoint: got %q, want %q", got.WSEndpoint, disc.WSEndpoint)
	}
	if got.PID != disc.PID {
		t.Errorf("PID: got %d, want %d", got.PID, disc.PID)
	}
	if got.ProxyHash != disc.ProxyHash {
		t.Errorf("ProxyHash: got %q, want %q", got.ProxyHash, disc.ProxyHash)
	}
	if got.StartedAt != disc.StartedAt {
		t.Errorf("StartedAt: got %q, want %q", got.StartedAt, disc.StartedAt)
	}
}

func TestReadDiscovery_MissingFile(t *testing.T) {
	_, err := readDiscovery(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadDiscovery_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "browser.json")
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := readDiscovery(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadDiscovery_MissingEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "browser.json")

	disc := BrowserDiscovery{PID: 1234}
	data, _ := json.Marshal(disc)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := readDiscovery(path)
	if err == nil {
		t.Error("expected error for missing ws_endpoint")
	}
}

func TestWriteDiscovery_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "browser.json")

	disc := &BrowserDiscovery{
		WSEndpoint: "ws://127.0.0.1:9222/devtools/browser/abc",
		PID:        1,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	if err := writeDiscovery(path, disc); err != nil {
		t.Fatal(err)
	}

	// Verify no temp file left behind
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == "browser.json.tmp" {
			t.Error("temp file should be renamed, not left behind")
		}
	}
}

func TestHealthCheck_InvalidURL(t *testing.T) {
	err := healthCheck("not-a-url")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestHealthCheck_UnreachablePort(t *testing.T) {
	// Use a port that's almost certainly not listening
	err := healthCheck("ws://127.0.0.1:19999/devtools/browser/test")
	if err == nil {
		t.Error("expected error for unreachable port")
	}
}

func TestMustParseTime_Valid(t *testing.T) {
	ts := "2026-02-10T12:00:00Z"
	result := mustParseTime(ts)
	if result.IsZero() {
		t.Error("expected non-zero time for valid RFC3339")
	}
}

func TestMustParseTime_Invalid(t *testing.T) {
	result := mustParseTime("not-a-time")
	if !result.IsZero() {
		t.Errorf("expected zero time for invalid input, got %v", result)
	}
}

func TestIdleTimeoutFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected time.Duration
	}{
		{"unset", "", 0},
		{"valid 30", "30", 30 * time.Second},
		{"valid 120", "120", 120 * time.Second},
		{"invalid string", "abc", 0},
		{"zero", "0", 0},
		{"negative", "-1", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("QUARRY_BROWSER_IDLE_TIMEOUT", tt.envVal)
			got := IdleTimeoutFromEnv()
			if got != tt.expected {
				t.Errorf("IdleTimeoutFromEnv() = %v, want %v", got, tt.expected)
			}
		})
	}
}
