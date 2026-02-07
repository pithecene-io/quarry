package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/justapithecus/quarry/types"
)

func TestLoad_FullConfig(t *testing.T) {
	yaml := `source: my-source
category: production
executor: ./executor.js

storage:
  dataset: quarry
  backend: s3
  path: my-bucket/prefix
  region: us-east-1
  endpoint: https://example.com
  s3_path_style: true

policy:
  name: buffered
  flush_mode: at_least_once
  buffer_events: 1000
  buffer_bytes: 10485760

proxies:
  pool_a:
    strategy: round_robin
    endpoints:
      - protocol: https
        host: proxy.example.com
        port: 8080

proxy:
  pool: pool_a
  strategy: round_robin

adapter:
  type: webhook
  url: https://hooks.example.com/quarry
  headers:
    Authorization: Bearer token123
  timeout: 10s
  retries: 3
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Top-level fields
	assertEqual(t, "source", cfg.Source, "my-source")
	assertEqual(t, "category", cfg.Category, "production")
	assertEqual(t, "executor", cfg.Executor, "./executor.js")

	// Storage
	assertEqual(t, "storage.backend", cfg.Storage.Backend, "s3")
	assertEqual(t, "storage.path", cfg.Storage.Path, "my-bucket/prefix")
	assertEqual(t, "storage.region", cfg.Storage.Region, "us-east-1")
	assertEqual(t, "storage.endpoint", cfg.Storage.Endpoint, "https://example.com")
	if !cfg.Storage.S3PathStyle {
		t.Error("expected storage.s3_path_style=true")
	}

	// Policy
	assertEqual(t, "policy.name", cfg.Policy.Name, "buffered")
	assertEqual(t, "policy.flush_mode", cfg.Policy.FlushMode, "at_least_once")
	if cfg.Policy.BufferEvents != 1000 {
		t.Errorf("expected buffer_events=1000, got %d", cfg.Policy.BufferEvents)
	}
	if cfg.Policy.BufferBytes != 10485760 {
		t.Errorf("expected buffer_bytes=10485760, got %d", cfg.Policy.BufferBytes)
	}

	// Proxy selection
	assertEqual(t, "proxy.pool", cfg.Proxy.Pool, "pool_a")
	assertEqual(t, "proxy.strategy", cfg.Proxy.Strategy, "round_robin")

	// Adapter
	assertEqual(t, "adapter.type", cfg.Adapter.Type, "webhook")
	assertEqual(t, "adapter.url", cfg.Adapter.URL, "https://hooks.example.com/quarry")
	if cfg.Adapter.Timeout.Duration != 10*time.Second {
		t.Errorf("expected adapter.timeout=10s, got %v", cfg.Adapter.Timeout.Duration)
	}
	if cfg.Adapter.Retries == nil || *cfg.Adapter.Retries != 3 {
		t.Errorf("expected adapter.retries=3")
	}
	if cfg.Adapter.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("expected Authorization header")
	}
}

func TestLoad_EmptyConfig(t *testing.T) {
	path := writeTemp(t, "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Source != "" {
		t.Errorf("expected empty source, got %q", cfg.Source)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/quarry.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "{{invalid yaml")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_SOURCE", "expanded-source")

	yaml := `source: ${TEST_SOURCE}`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertEqual(t, "source", cfg.Source, "expanded-source")
}

func TestProxyPools_Conversion(t *testing.T) {
	cfg := &Config{
		Proxies: map[string]ProxyPoolConfig{
			"beta_pool": {
				Strategy: types.ProxyStrategyRandom,
				Endpoints: []types.ProxyEndpoint{
					{Protocol: types.ProxyProtocolHTTP, Host: "b.example.com", Port: 8080},
				},
			},
			"alpha_pool": {
				Strategy: types.ProxyStrategyRoundRobin,
				Endpoints: []types.ProxyEndpoint{
					{Protocol: types.ProxyProtocolHTTPS, Host: "a.example.com", Port: 443},
				},
			},
		},
	}

	pools := cfg.ProxyPools()
	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}

	// Sorted by name: alpha_pool before beta_pool
	if pools[0].Name != "alpha_pool" {
		t.Errorf("expected first pool name=alpha_pool, got %q", pools[0].Name)
	}
	if pools[1].Name != "beta_pool" {
		t.Errorf("expected second pool name=beta_pool, got %q", pools[1].Name)
	}

	if pools[0].Strategy != types.ProxyStrategyRoundRobin {
		t.Errorf("expected alpha_pool strategy=round_robin, got %q", pools[0].Strategy)
	}
}

func TestProxyPools_Empty(t *testing.T) {
	cfg := &Config{}
	pools := cfg.ProxyPools()
	if pools != nil {
		t.Errorf("expected nil for empty proxies, got %v", pools)
	}
}

func TestProxyPools_WithSticky(t *testing.T) {
	ttl := int64(3600000)
	cfg := &Config{
		Proxies: map[string]ProxyPoolConfig{
			"sticky_pool": {
				Strategy: types.ProxyStrategySticky,
				Sticky: &types.ProxySticky{
					Scope: types.ProxyStickyDomain,
					TTLMs: &ttl,
				},
				Endpoints: []types.ProxyEndpoint{
					{Protocol: types.ProxyProtocolHTTP, Host: "proxy.example.com", Port: 8080},
				},
			},
		},
	}

	pools := cfg.ProxyPools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].Sticky == nil {
		t.Fatal("expected sticky config")
	}
	if pools[0].Sticky.Scope != types.ProxyStickyDomain {
		t.Errorf("expected sticky scope=domain, got %q", pools[0].Sticky.Scope)
	}
	if pools[0].Sticky.TTLMs == nil || *pools[0].Sticky.TTLMs != 3600000 {
		t.Error("expected sticky TTL=3600000")
	}
}

func TestLoad_UnknownKeyRejected(t *testing.T) {
	yaml := `source: my-source
bogus_key: should_fail
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_key") {
		t.Errorf("error should mention the unknown key, got: %v", err)
	}
}

func TestLoad_UnknownNestedKeyRejected(t *testing.T) {
	yaml := `storage:
  backend: fs
  path: ./data
  unknown_field: bad
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown nested key, got nil")
	}
	if !strings.Contains(err.Error(), "unknown_field") {
		t.Errorf("error should mention the unknown key, got: %v", err)
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	yaml := `timeout: 30s`
	path := writeTemp(t, "adapter:\n  "+yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Adapter.Timeout.Duration != 30*time.Second {
		t.Errorf("expected 30s, got %v", cfg.Adapter.Timeout.Duration)
	}
}

// writeTemp writes content to a temp file and returns the path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "quarry.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}
