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

func TestProxyPool_Validate_RecencyWindow_Positive(t *testing.T) {
	w := 3
	pool := &ProxyPool{
		Name:     "recency-pool",
		Strategy: ProxyStrategyRandom,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p4.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	if err := pool.Validate(); err != nil {
		t.Errorf("expected valid pool, got %v", err)
	}
}

func TestProxyPool_Validate_RecencyWindow_Zero(t *testing.T) {
	w := 0
	pool := &ProxyPool{
		Name:     "recency-pool",
		Strategy: ProxyStrategyRandom,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	if err := pool.Validate(); err == nil {
		t.Error("expected validation error for zero recency window")
	}
}

func TestProxyPool_Validate_RecencyWindow_Negative(t *testing.T) {
	w := -1
	pool := &ProxyPool{
		Name:     "recency-pool",
		Strategy: ProxyStrategyRandom,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	if err := pool.Validate(); err == nil {
		t.Error("expected validation error for negative recency window")
	}
}

func TestProxyPool_Warnings_RecencyWindow_NonRandom(t *testing.T) {
	w := 3
	pool := &ProxyPool{
		Name:     "recency-rr",
		Strategy: ProxyStrategyRoundRobin,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	warnings := pool.Warnings()
	found := false
	for _, warning := range warnings {
		if warning != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for recency_window on non-random strategy")
	}
}

func TestProxyPool_Warnings_RecencyWindow_Random_NoWarning(t *testing.T) {
	w := 3
	pool := &ProxyPool{
		Name:     "recency-random",
		Strategy: ProxyStrategyRandom,
		Endpoints: []ProxyEndpoint{
			{Protocol: ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
			{Protocol: ProxyProtocolHTTP, Host: "p4.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	warnings := pool.Warnings()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for random with recency window, got %d: %v", len(warnings), warnings)
	}
}
