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

	// Select in round-robin order (Commit: true to advance counter)
	hosts := make([]string, 6)
	for i := 0; i < 6; i++ {
		ep, err := s.Select(SelectRequest{Pool: "test", Commit: true})
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

	// Same job should get same endpoint (Commit: true to store sticky entry)
	ep1, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123", Commit: true})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	ep2, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123", Commit: true})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if ep1.Host != ep2.Host {
		t.Errorf("same job got different endpoints: %q vs %q", ep1.Host, ep2.Host)
	}

	// Different job may get different endpoint (not guaranteed, but verify no error)
	_, err = s.Select(SelectRequest{Pool: "test", JobID: "job-456", Commit: true})
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

	// Get initial assignment (Commit: true to store sticky entry)
	ep1, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123", Commit: true})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// After TTL, may get different endpoint
	ep2, err := s.Select(SelectRequest{Pool: "test", JobID: "job-123", Commit: true})
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

	// Explicit sticky key takes precedence over domain (Commit: true to store)
	ep1, err := s.Select(SelectRequest{
		Pool:      "test",
		StickyKey: "my-explicit-key",
		Domain:    "example.com",
		Commit:    true,
	})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	ep2, err := s.Select(SelectRequest{
		Pool:      "test",
		StickyKey: "my-explicit-key",
		Domain:    "different.com", // Different domain, but same explicit key
		Commit:    true,
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

func TestSelector_Commit_RoundRobin(t *testing.T) {
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

	// Peek (Commit: false) should return same endpoint repeatedly
	ep1, _ := s.Select(SelectRequest{Pool: "test", Commit: false})
	ep2, _ := s.Select(SelectRequest{Pool: "test", Commit: false})
	ep3, _ := s.Select(SelectRequest{Pool: "test", Commit: false})

	if ep1.Host != ep2.Host || ep2.Host != ep3.Host {
		t.Errorf("peek should return same endpoint: got %q, %q, %q", ep1.Host, ep2.Host, ep3.Host)
	}

	// Commit should advance the counter
	epCommit, _ := s.Select(SelectRequest{Pool: "test", Commit: true})
	if epCommit.Host != ep1.Host {
		t.Errorf("first commit should return same as peek: got %q, want %q", epCommit.Host, ep1.Host)
	}

	// Now peek should return next endpoint
	epPeek, _ := s.Select(SelectRequest{Pool: "test", Commit: false})
	if epPeek.Host == ep1.Host {
		t.Errorf("peek after commit should return next endpoint, got same: %q", epPeek.Host)
	}
}

func TestSelector_Commit_Sticky(t *testing.T) {
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategySticky,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
		},
		Sticky: &types.ProxySticky{
			Scope: types.ProxyStickyJob,
		},
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Peek (Commit: false) should NOT store the sticky entry
	ep1, _ := s.Select(SelectRequest{Pool: "test", JobID: "job-new", Commit: false})

	// Check stats - should have 0 sticky entries since we didn't commit
	stats, _ := s.Stats("test")
	if stats.StickyEntries != 0 {
		t.Errorf("StickyEntries after peek = %d, want 0", stats.StickyEntries)
	}

	// Now commit - should store the entry
	_, _ = s.Select(SelectRequest{Pool: "test", JobID: "job-new", Commit: true})
	stats, _ = s.Stats("test")
	if stats.StickyEntries != 1 {
		t.Errorf("StickyEntries after commit = %d, want 1", stats.StickyEntries)
	}

	// Subsequent selects (even peek) should return the stored assignment
	ep2, _ := s.Select(SelectRequest{Pool: "test", JobID: "job-new", Commit: false})
	// Note: ep2 should equal the committed assignment, not necessarily ep1
	// (ep1 may differ due to random selection without commit)
	_ = ep1
	_ = ep2
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

	// Make some selections (Commit: true to store sticky entries)
	_, _ = s.Select(SelectRequest{Pool: "test", JobID: "job-1", Commit: true})
	_, _ = s.Select(SelectRequest{Pool: "test", JobID: "job-2", Commit: true})

	stats, err := s.Stats("test")
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.StickyEntries != 2 {
		t.Errorf("StickyEntries = %d, want 2", stats.StickyEntries)
	}
}

func TestSelector_Random_RecencyAvoidance(t *testing.T) {
	w := 3
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategyRandom,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p4.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p5.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// Collect 10 committed selections; no selection should repeat
	// any of the previous 3 (the recency window).
	hosts := make([]string, 10)
	for i := 0; i < 10; i++ {
		ep, err := s.Select(SelectRequest{Pool: "test", Commit: true})
		if err != nil {
			t.Fatalf("Select %d failed: %v", i, err)
		}
		hosts[i] = ep.Host

		// Check against the recency window
		windowStart := i - w
		if windowStart < 0 {
			windowStart = 0
		}
		for j := windowStart; j < i; j++ {
			if hosts[j] == hosts[i] {
				t.Errorf("selection %d (%s) repeats selection %d within recency window of %d",
					i, hosts[i], j, w)
			}
		}
	}
}

func TestSelector_Random_RecencyLRUFallback(t *testing.T) {
	w := 3
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategyRandom,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// With 3 endpoints and window=3, after 3 commits all are in the ring.
	// Candidates shrink: 3 → 2 → 1, then the 4th must be the LRU.
	hosts := make([]string, 3)
	for i := 0; i < 3; i++ {
		ep, err := s.Select(SelectRequest{Pool: "test", Commit: true})
		if err != nil {
			t.Fatalf("Select %d failed: %v", i, err)
		}
		hosts[i] = ep.Host
	}

	// All 3 should be distinct (each was excluded from prior selections)
	seen := map[string]bool{}
	for _, h := range hosts {
		if seen[h] {
			t.Fatalf("expected all distinct in first 3, got duplicate: %s", h)
		}
		seen[h] = true
	}

	// 4th selection: all excluded, LRU fallback should return hosts[0]
	ep4, err := s.Select(SelectRequest{Pool: "test", Commit: true})
	if err != nil {
		t.Fatalf("Select 4 failed: %v", err)
	}
	if ep4.Host != hosts[0] {
		t.Errorf("LRU fallback: got %q, want %q (first-selected)", ep4.Host, hosts[0])
	}

	// 5th selection: LRU should now be hosts[1]
	ep5, err := s.Select(SelectRequest{Pool: "test", Commit: true})
	if err != nil {
		t.Fatalf("Select 5 failed: %v", err)
	}
	if ep5.Host != hosts[1] {
		t.Errorf("LRU fallback: got %q, want %q (second-selected)", ep5.Host, hosts[1])
	}
}

func TestSelector_Random_RecencyPeekNoUpdate(t *testing.T) {
	w := 2
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategyRandom,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p4.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	// One committed selection
	_, err := s.Select(SelectRequest{Pool: "test", Commit: true})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	stats, _ := s.Stats("test")
	if stats.RecencyFill != 1 {
		t.Fatalf("RecencyFill after 1 commit = %d, want 1", stats.RecencyFill)
	}

	// Multiple peeks should not advance the ring
	for i := 0; i < 10; i++ {
		_, err := s.Select(SelectRequest{Pool: "test", Commit: false})
		if err != nil {
			t.Fatalf("Peek %d failed: %v", i, err)
		}
	}

	stats, _ = s.Stats("test")
	if stats.RecencyFill != 1 {
		t.Errorf("RecencyFill after peeks = %d, want 1", stats.RecencyFill)
	}
}

func TestSelector_Random_RecencyWindowNil(t *testing.T) {
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

	// Regression guard: 100 selections with no recency window should work
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		ep, err := s.Select(SelectRequest{Pool: "test", Commit: true})
		if err != nil {
			t.Fatalf("Select %d failed: %v", i, err)
		}
		seen[ep.Host] = true
	}

	if len(seen) < 2 {
		t.Errorf("expected multiple unique hosts, got %d", len(seen))
	}
}

func TestSelector_Random_RecencyStats(t *testing.T) {
	w := 2
	s := NewSelector()

	pool := &types.ProxyPool{
		Name:     "test",
		Strategy: types.ProxyStrategyRandom,
		Endpoints: []types.ProxyEndpoint{
			{Protocol: types.ProxyProtocolHTTP, Host: "p1.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p2.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p3.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p4.example.com", Port: 8080},
			{Protocol: types.ProxyProtocolHTTP, Host: "p5.example.com", Port: 8080},
		},
		RecencyWindow: &w,
	}

	if err := s.RegisterPool(pool); err != nil {
		t.Fatalf("RegisterPool failed: %v", err)
	}

	stats, _ := s.Stats("test")
	if stats.RecencyWindow != 2 {
		t.Errorf("RecencyWindow = %d, want 2", stats.RecencyWindow)
	}
	if stats.RecencyFill != 0 {
		t.Errorf("RecencyFill = %d, want 0", stats.RecencyFill)
	}

	// 1 commit → fill 1
	s.Select(SelectRequest{Pool: "test", Commit: true})
	stats, _ = s.Stats("test")
	if stats.RecencyFill != 1 {
		t.Errorf("RecencyFill after 1 commit = %d, want 1", stats.RecencyFill)
	}

	// 2 commits → fill 2
	s.Select(SelectRequest{Pool: "test", Commit: true})
	stats, _ = s.Stats("test")
	if stats.RecencyFill != 2 {
		t.Errorf("RecencyFill after 2 commits = %d, want 2", stats.RecencyFill)
	}

	// 3 commits → fill still 2 (capped at window size)
	s.Select(SelectRequest{Pool: "test", Commit: true})
	stats, _ = s.Stats("test")
	if stats.RecencyFill != 2 {
		t.Errorf("RecencyFill after 3 commits = %d, want 2 (capped)", stats.RecencyFill)
	}
}
