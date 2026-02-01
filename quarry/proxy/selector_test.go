package proxy

import (
	"testing"
	"time"

	"github.com/justapithecus/quarry/types"
)

func TestSelector_RoundRobin(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategyRoundRobin,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Select in round-robin order
	hosts := make([]string, 6)
	for i := 0; i < 6; i++ {
		ep, err := s.Select(SelectRequest{Pool: "test"})
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		hosts[i] = ep.Host
	}

	// Should cycle through endpoints
	expected := []string{
		"p1.example.com",
		"p2.example.com",
		"p3.example.com",
		"p1.example.com",
		"p2.example.com",
		"p3.example.com",
	}

	for i, exp := range expected {
		if hosts[i] != exp {
			t.Errorf("hosts[%d] = %q, want %q", i, hosts[i], exp)
		}
	}
}

func TestSelector_Random(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategyRandom,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Select multiple times - should not panic
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ep, err := s.Select(SelectRequest{Pool: "test"})
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		seen[ep.Host] = true
	}

	// With 100 selections, we should see all endpoints (probabilistically)
	if len(seen) < 2 {
		t.Errorf("random selection seems broken: only saw %d unique hosts", len(seen))
	}
}

func TestSelector_Sticky_Job(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategySticky,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
		},
		Sticky: &types.ProxySticky{
			Scope: types.ProxyStickyJob,
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Same job should get same endpoint
	ep1, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123"})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	ep2, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123"})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if ep1.Host != ep2.Host {
		t.Errorf("same job got different endpoints: %q vs %q", ep1.Host, ep2.Host)
	}

	// Different job may get different endpoint (not guaranteed, but verify no error)
	_, err = s.Select(SelectRequest{Pool: "test", JobID: "job-456"})
	if err != nil {
		t.Fatalf("Select failed for different job: %v", err)
	}
}

func TestSelector_Sticky_WithTTL(t *testing.T) {
	s := NewSelector()

	ttl := int64(50) // 50ms TTL
	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategySticky,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
		},
		Sticky: &types.ProxySticky{
			Scope: types.ProxyStickyJob,
			TTLMs: &ttl,
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Get initial assignment
	ep1, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123"})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// After TTL, may get different endpoint
	ep2, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123"})
	if err != nil {
		t.Fatalf("Select failed after TTL: %v", err)
	}

	// Note: ep2 may or may not equal ep1 (random selection after TTL)
	// We just verify it doesn't error
	_ = ep1
	_ = ep2
}

func TestSelector_Sticky_ExplicitKey(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategySticky,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
		},
		Sticky: &types.ProxySticky{
			Scope: types.ProxyStickyDomain,
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Explicit sticky key takes precedence over domain
	ep1, err := s.Select(SelectRequest{
		Pool:      "test",
		StickyKey: "my-explicit-key",
		Domain:    "example.com",
	})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	ep2, err := s.Select(SelectRequest{
		Pool:      "test",
		StickyKey: "my-explicit-key",
		Domain:    "different.com", // Different domain, but same explicit key
	})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if ep1.Host != ep2.Host {
		t.Errorf("explicit sticky key should give same endpoint: %q vs %q", ep1.Host, ep2.Host)
	}
}

func TestSelector_StrategyOverride(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategyRoundRobin, // Default is round-robin
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Override to random
	randomStrategy := types.ProxyStrategyRandom
	_, err := s.Select(SelectRequest{
		Pool:             "test",
		StrategyOverride: &randomStrategy,
	})
	if err != nil {
		t.Fatalf("Select with strategy override failed: %v", err)
	}
}

func TestSelector_PoolNotFound(t *testing.T) {
	s := NewSelector()

	_, err := s.Select(SelectRequest{Pool: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent pool")
	}
}

func TestSelector_ValidationFailure(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:      "test",
		Strategy:  types.ProxyStrategyRoundRobin,
		Endpoints: []types.ProxyEndpoint{}, // Invalid: no endpoints
	}

	err := s.RegisterPool(pool)
	if err == nil {
		t.Error("expected validation error for empty endpoints")
	}
}

func TestSelector_Stats(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategySticky,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
		},
		Sticky: &types.ProxySticky{
			Scope: types.ProxyStickyJob,
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Make some selections
	s.Select(SelectRequest{Pool: "test", JobID: "job-1"})
	s.Select(SelectRequest{Pool: "test", JobID: "job-2"})

	stats, err := s.Stats("test")
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.StickyEntries != 2 {
		t.Errorf("StickyEntries = %d, want 2", stats.StickyEntries)
	}
}
