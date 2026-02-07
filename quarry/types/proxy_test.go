package types //nolint:revive // types is a valid package name

import (
	"testing"
)

func TestProxyEndpoint_Warnings_Socks5(t *testing.T) {
	// socks5 should generate a warning
	ep := &ProxyEndpoint{
		Protocol: ProxyProtocolSOCKS5,
		Host:     "proxy.example.com",
		Port:     1080,
	}

	warnings := ep.Warnings()
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for socks5, got %d", len(warnings))
	}
	if len(warnings) > 0 && warnings[0] == "" {
		t.Error("warning should not be empty")
	}
}

func TestProxyEndpoint_Warnings_HTTP(t *testing.T) {
	// http should not generate warnings
	ep := &ProxyEndpoint{
		Protocol: ProxyProtocolHTTP,
		Host:     "proxy.example.com",
		Port:     8080,
	}

	warnings := ep.Warnings()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for http, got %d", len(warnings))
	}
}

func TestProxyPool_Warnings_LargeRoundRobin(t *testing.T) {
	// Large pool with round_robin should generate a warning
	endpoints := make([]ProxyEndpoint, LargePoolThreshold+1)
	for i := range endpoints {
		endpoints[i] = ProxyEndpoint{
			Protocol: ProxyProtocolHTTP,
			Host:     "proxy.example.com",
			Port:     8080 + i,
		}
	}

	pool := &ProxyPool{
		Name:      "large-pool",
		Strategy:  ProxyStrategyRoundRobin,
		Endpoints: endpoints,
	}

	warnings := pool.Warnings()
	found := false
	for _, w := range warnings {
		if w != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for large round_robin pool")
	}
}

func TestProxyPool_Warnings_LargeRandom(t *testing.T) {
	// Large pool with random should NOT generate a warning
	endpoints := make([]ProxyEndpoint, LargePoolThreshold+1)
	for i := range endpoints {
		endpoints[i] = ProxyEndpoint{
			Protocol: ProxyProtocolHTTP,
			Host:     "proxy.example.com",
			Port:     8080 + i,
		}
	}

	pool := &ProxyPool{
		Name:      "large-pool",
		Strategy:  ProxyStrategyRandom,
		Endpoints: endpoints,
	}

	warnings := pool.Warnings()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for large random pool, got %d", len(warnings))
	}
}

func TestProxyPool_Warnings_Socks5Endpoint(t *testing.T) {
	// Pool with socks5 endpoint should generate a warning
	pool := &ProxyPool{
		Name:     "socks-pool",
		Strategy: ProxyStrategyRandom,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "http.example.com", Port: 8080},
			{Protocol: ProxyProtocolSOCKS5, Host: "socks.example.com", Port: 1080},
		},
	}

	warnings := pool.Warnings()
	found := false
	for _, w := range warnings {
		if w != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for pool with socks5 endpoint")
	}
}

func TestProxyPool_Warnings_NoWarnings(t *testing.T) {
	// Normal pool should not generate warnings
	pool := &ProxyPool{
		Name:     "normal-pool",
		Strategy: ProxyStrategyRoundRobin,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "proxy.example.com", Port: 8080},
		},
	}

	warnings := pool.Warnings()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for normal pool, got %d", len(warnings))
	}
}

func TestProxyPool_Validate_RecencyWindowZero(t *testing.T) {
	w := 0
	pool := &ProxyPool{
		Name:          "test",
		Strategy:      ProxyStrategyRandom,
		Endpoints:     []ProxyEndpoint{{Protocol: ProxyProtocolHTTP, Host: "p.example.com", Port: 8080}},
		RecencyWindow: &w,
	}

	err := pool.Validate()
	if err == nil {
		t.Error("expected validation error for recency_window = 0")
	}
}

func TestProxyPool_Validate_RecencyWindowNegative(t *testing.T) {
	w := -1
	pool := &ProxyPool{
		Name:          "test",
		Strategy:      ProxyStrategyRandom,
		Endpoints:     []ProxyEndpoint{{Protocol: ProxyProtocolHTTP, Host: "p.example.com", Port: 8080}},
		RecencyWindow: &w,
	}

	err := pool.Validate()
	if err == nil {
		t.Error("expected validation error for recency_window = -1")
	}
}

func TestProxyPool_Validate_RecencyWindowValid(t *testing.T) {
	w := 3
	pool := &ProxyPool{
		Name:     "test",
		Strategy: ProxyStrategyRandom,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p2.example.com", Port: 8081},
			{Protocol: ProxyProtocolHTTP, Host: "p3.example.com", Port: 8082},
		},
		RecencyWindow: &w,
	}

	if err := pool.Validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestProxyPool_Warnings_RecencyWindowNonRandom(t *testing.T) {
	w := 2
	pool := &ProxyPool{
		Name:          "test",
		Strategy:      ProxyStrategyRoundRobin,
		Endpoints:     []ProxyEndpoint{{Protocol: ProxyProtocolHTTP, Host: "p.example.com", Port: 8080}},
		RecencyWindow: &w,
	}

	warnings := pool.Warnings()
	found := false
	for _, msg := range warnings {
		if msg != "" && contains(msg, "recency_window") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for recency_window on non-random strategy")
	}
}

func TestProxyPool_Warnings_RecencyWindowRandom(t *testing.T) {
	w := 2
	pool := &ProxyPool{
		Name:     "test",
		Strategy: ProxyStrategyRandom,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p2.example.com", Port: 8081},
		},
		RecencyWindow: &w,
	}

	warnings := pool.Warnings()
	for _, msg := range warnings {
		if contains(msg, "recency_window") {
			t.Errorf("unexpected recency_window warning for random strategy: %s", msg)
		}
	}
}

// contains checks if substr is in s (avoids importing strings in test).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
