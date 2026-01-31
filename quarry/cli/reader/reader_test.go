package reader

import (
	"testing"
)

// TestInspectRunResponse verifies the response shape matches CONTRACT_CLI.md.
func TestInspectRunResponse(t *testing.T) {
	resp := InspectRun("test-run")

	if resp.RunID != "test-run" {
		t.Errorf("RunID = %q, want %q", resp.RunID, "test-run")
	}
	if resp.JobID == "" {
		t.Error("JobID should not be empty")
	}
	if resp.State == "" {
		t.Error("State should not be empty")
	}
	if resp.Attempt < 1 {
		t.Errorf("Attempt = %d, should be >= 1", resp.Attempt)
	}
	if resp.Policy == "" {
		t.Error("Policy should not be empty")
	}
	if resp.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

// TestInspectJobResponse verifies the response shape.
func TestInspectJobResponse(t *testing.T) {
	resp := InspectJob("test-job")

	if resp.JobID != "test-job" {
		t.Errorf("JobID = %q, want %q", resp.JobID, "test-job")
	}
	if resp.State == "" {
		t.Error("State should not be empty")
	}
	// RunIDs may be empty but should not be nil
	if resp.RunIDs == nil {
		t.Error("RunIDs should not be nil")
	}
}

// TestInspectProxyResponse verifies proxy pool response shape.
func TestInspectProxyResponse(t *testing.T) {
	resp := InspectProxy("test-pool")

	if resp.Name != "test-pool" {
		t.Errorf("Name = %q, want %q", resp.Name, "test-pool")
	}
	if resp.Strategy == "" {
		t.Error("Strategy should not be empty")
	}
	if resp.EndpointCnt < 0 {
		t.Errorf("EndpointCnt = %d, should be >= 0", resp.EndpointCnt)
	}
}

// TestStatsRunsResponse verifies stats response shape.
func TestStatsRunsResponse(t *testing.T) {
	resp := StatsRuns()

	if resp.Total < 0 {
		t.Errorf("Total = %d, should be >= 0", resp.Total)
	}
	if resp.Running < 0 {
		t.Errorf("Running = %d, should be >= 0", resp.Running)
	}
	if resp.Succeeded < 0 {
		t.Errorf("Succeeded = %d, should be >= 0", resp.Succeeded)
	}
	if resp.Failed < 0 {
		t.Errorf("Failed = %d, should be >= 0", resp.Failed)
	}
}

// TestListRunsNoLimit verifies that limit=0 returns all results.
func TestListRunsNoLimit(t *testing.T) {
	opts := ListRunsOptions{Limit: 0}
	results := ListRuns(opts)

	// Stub returns 4 items; with limit=0 we should get all
	if len(results) != 4 {
		t.Errorf("ListRuns with limit=0 returned %d items, expected 4", len(results))
	}
}

// TestListRunsWithLimit verifies that limit is applied.
func TestListRunsWithLimit(t *testing.T) {
	opts := ListRunsOptions{Limit: 2}
	results := ListRuns(opts)

	if len(results) != 2 {
		t.Errorf("ListRuns with limit=2 returned %d items, expected 2", len(results))
	}
}

// TestListRunsWithStateFilter verifies state filtering.
func TestListRunsWithStateFilter(t *testing.T) {
	opts := ListRunsOptions{State: "succeeded"}
	results := ListRuns(opts)

	for _, r := range results {
		if r.State != "succeeded" {
			t.Errorf("Expected state 'succeeded', got %q", r.State)
		}
	}
}

// TestListRunItemShape verifies list item response shape.
func TestListRunItemShape(t *testing.T) {
	results := ListRuns(ListRunsOptions{})

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	item := results[0]
	if item.RunID == "" {
		t.Error("RunID should not be empty")
	}
	if item.State == "" {
		t.Error("State should not be empty")
	}
	if item.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

// TestDebugResolveProxyCommitted verifies committed flag is set.
func TestDebugResolveProxyCommitted(t *testing.T) {
	// Without commit
	resp, err := DebugResolveProxy("test-pool", false)
	if err != nil {
		t.Fatalf("DebugResolveProxy failed: %v", err)
	}
	if resp.Committed {
		t.Error("Committed should be false when commit=false")
	}

	// With commit
	resp, err = DebugResolveProxy("test-pool", true)
	if err != nil {
		t.Fatalf("DebugResolveProxy failed: %v", err)
	}
	if !resp.Committed {
		t.Error("Committed should be true when commit=true")
	}
}

// TestDebugResolveProxyRequiresPool verifies pool validation.
func TestDebugResolveProxyRequiresPool(t *testing.T) {
	_, err := DebugResolveProxy("", false)
	if err == nil {
		t.Error("Expected error for empty pool name")
	}
}

// TestDebugIPCResponse verifies IPC debug response shape.
func TestDebugIPCResponse(t *testing.T) {
	resp := DebugIPC(false)

	if resp.Transport == "" {
		t.Error("Transport should not be empty")
	}
	if resp.Encoding == "" {
		t.Error("Encoding should not be empty")
	}
	if resp.MessagesSent < 0 {
		t.Errorf("MessagesSent = %d, should be >= 0", resp.MessagesSent)
	}
	if resp.Errors < 0 {
		t.Errorf("Errors = %d, should be >= 0", resp.Errors)
	}
}
