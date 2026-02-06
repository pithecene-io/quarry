package lode

import (
	"errors"
	"testing"
	"time"

	"github.com/justapithecus/lode/lode"

	"github.com/justapithecus/quarry/metrics"
)

// sharedFactory returns a StoreFactory that always returns the given store.
// This allows write and read datasets to share the same in-memory state.
func sharedFactory(store lode.Store) lode.StoreFactory {
	return func() (lode.Store, error) { return store, nil }
}

func TestQueryLatestMetrics_WriteAndRead(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-001",
	}

	// Write via LodeClient
	client, err := NewLodeClientWithFactory(cfg, factory)
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	snap := metrics.Snapshot{
		RunsStarted:          1,
		RunsCompleted:        1,
		EventsReceived:       42,
		EventsPersisted:      40,
		EventsDropped:        2,
		DroppedByType:        map[string]int64{"debug": 2},
		ExecutorLaunchSuccess: 1,
		LodeWriteSuccess:     10,
		Policy:               "strict",
		Executor:             "executor.js",
		StorageBackend:       "fs",
		RunID:                "run-001",
		JobID:                "job-xyz",
	}

	completedAt := time.Date(2026, 2, 3, 15, 0, 0, 0, time.UTC)
	if err := client.WriteMetrics(t.Context(), snap, completedAt); err != nil {
		t.Fatalf("WriteMetrics failed: %v", err)
	}

	// Read via QueryLatestMetrics
	ds, err := NewReadDataset(factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	record, err := QueryLatestMetrics(t.Context(), ds, "", "")
	if err != nil {
		t.Fatalf("QueryLatestMetrics failed: %v", err)
	}

	// Verify round-trip fidelity
	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunsStarted != 1 {
		t.Errorf("RunsStarted = %d, want 1", parsed.RunsStarted)
	}
	if parsed.RunsCompleted != 1 {
		t.Errorf("RunsCompleted = %d, want 1", parsed.RunsCompleted)
	}
	if parsed.EventsReceived != 42 {
		t.Errorf("EventsReceived = %d, want 42", parsed.EventsReceived)
	}
	if parsed.EventsPersisted != 40 {
		t.Errorf("EventsPersisted = %d, want 40", parsed.EventsPersisted)
	}
	if parsed.EventsDropped != 2 {
		t.Errorf("EventsDropped = %d, want 2", parsed.EventsDropped)
	}
	if parsed.ExecutorLaunchSuccess != 1 {
		t.Errorf("ExecutorLaunchSuccess = %d, want 1", parsed.ExecutorLaunchSuccess)
	}
	if parsed.LodeWriteSuccess != 10 {
		t.Errorf("LodeWriteSuccess = %d, want 10", parsed.LodeWriteSuccess)
	}
	if parsed.Policy != "strict" {
		t.Errorf("Policy = %q, want %q", parsed.Policy, "strict")
	}
	if parsed.Executor != "executor.js" {
		t.Errorf("Executor = %q, want %q", parsed.Executor, "executor.js")
	}
	if parsed.StorageBackend != "fs" {
		t.Errorf("StorageBackend = %q, want %q", parsed.StorageBackend, "fs")
	}
	if parsed.RunID != "run-001" {
		t.Errorf("RunID = %q, want %q", parsed.RunID, "run-001")
	}
	if parsed.JobID != "job-xyz" {
		t.Errorf("JobID = %q, want %q", parsed.JobID, "job-xyz")
	}
	if parsed.DroppedByType == nil {
		t.Fatal("DroppedByType should not be nil")
	}
	if parsed.DroppedByType["debug"] != 2 {
		t.Errorf("DroppedByType[debug] = %d, want 2", parsed.DroppedByType["debug"])
	}
}

func TestQueryLatestMetrics_MultipleRuns(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	completedAt := time.Date(2026, 2, 3, 15, 0, 0, 0, time.UTC)

	// Write metrics for 3 different runs
	for i, runID := range []string{"run-001", "run-002", "run-003"} {
		cfg := Config{
			Dataset:  "quarry",
			Source:   "test-source",
			Category: "test-category",
			Day:      "2026-02-03",
			RunID:    runID,
		}

		client, err := NewLodeClientWithFactory(cfg, factory)
		if err != nil {
			t.Fatalf("NewLodeClientWithFactory failed: %v", err)
		}

		snap := metrics.Snapshot{
			RunsStarted: int64(i + 1),
			RunID:        runID,
			Policy:       "strict",
			Executor:     "executor.js",
			StorageBackend: "fs",
		}

		if err := client.WriteMetrics(t.Context(), snap, completedAt.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("WriteMetrics for %s failed: %v", runID, err)
		}
	}

	// Read without filter — should get latest (run-003)
	ds, err := NewReadDataset(factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	record, err := QueryLatestMetrics(t.Context(), ds, "", "")
	if err != nil {
		t.Fatalf("QueryLatestMetrics failed: %v", err)
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunID != "run-003" {
		t.Errorf("RunID = %q, want %q (latest)", parsed.RunID, "run-003")
	}
	if parsed.RunsStarted != 3 {
		t.Errorf("RunsStarted = %d, want 3", parsed.RunsStarted)
	}
}

func TestQueryLatestMetrics_FilterByRunID(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	completedAt := time.Date(2026, 2, 3, 15, 0, 0, 0, time.UTC)

	// Write metrics for 3 runs
	for i, runID := range []string{"run-001", "run-002", "run-003"} {
		cfg := Config{
			Dataset:  "quarry",
			Source:   "test-source",
			Category: "test-category",
			Day:      "2026-02-03",
			RunID:    runID,
		}

		client, err := NewLodeClientWithFactory(cfg, factory)
		if err != nil {
			t.Fatalf("NewLodeClientWithFactory failed: %v", err)
		}

		snap := metrics.Snapshot{
			RunsStarted:   int64(i + 1),
			RunID:          runID,
			Policy:         "strict",
			Executor:       "executor.js",
			StorageBackend: "fs",
		}

		if err := client.WriteMetrics(t.Context(), snap, completedAt.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("WriteMetrics for %s failed: %v", runID, err)
		}
	}

	// Filter by specific run-id
	ds, err := NewReadDataset(factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	record, err := QueryLatestMetrics(t.Context(), ds, "run-002", "")
	if err != nil {
		t.Fatalf("QueryLatestMetrics failed: %v", err)
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunID != "run-002" {
		t.Errorf("RunID = %q, want %q", parsed.RunID, "run-002")
	}
	if parsed.RunsStarted != 2 {
		t.Errorf("RunsStarted = %d, want 2", parsed.RunsStarted)
	}
}

func TestQueryLatestMetrics_FilterBySource(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	completedAt := time.Date(2026, 2, 3, 15, 0, 0, 0, time.UTC)

	// Write metrics with different sources
	for i, source := range []string{"alpha", "beta"} {
		cfg := Config{
			Dataset:  "quarry",
			Source:   source,
			Category: "test-category",
			Day:      "2026-02-03",
			RunID:    "run-001",
		}

		client, err := NewLodeClientWithFactory(cfg, factory)
		if err != nil {
			t.Fatalf("NewLodeClientWithFactory failed: %v", err)
		}

		snap := metrics.Snapshot{
			RunsStarted:   int64(i + 1),
			RunID:          "run-001",
			Policy:         "strict",
			Executor:       "executor.js",
			StorageBackend: "fs",
		}

		if err := client.WriteMetrics(t.Context(), snap, completedAt.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("WriteMetrics for source %s failed: %v", source, err)
		}
	}

	ds, err := NewReadDataset(factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	// Filter by source=alpha
	record, err := QueryLatestMetrics(t.Context(), ds, "", "alpha")
	if err != nil {
		t.Fatalf("QueryLatestMetrics failed: %v", err)
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunsStarted != 1 {
		t.Errorf("RunsStarted = %d, want 1 (alpha source)", parsed.RunsStarted)
	}
}

func TestQueryLatestMetrics_NoMetrics(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	ds, err := NewReadDataset(factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	_, err = QueryLatestMetrics(t.Context(), ds, "", "")
	if err == nil {
		t.Fatal("expected error for empty dataset, got nil")
	}
	if !errors.Is(err, ErrNoMetricsFound) {
		t.Errorf("expected ErrNoMetricsFound, got: %v", err)
	}
}

func TestParseMetricsRecord(t *testing.T) {
	// Simulate a JSON-round-tripped record (float64 values)
	record := map[string]any{
		"record_kind":                    "metrics",
		"runs_started_total":             float64(5),
		"runs_completed_total":           float64(4),
		"runs_failed_total":              float64(1),
		"runs_crashed_total":             float64(0),
		"events_received_total":          float64(100),
		"events_persisted_total":         float64(98),
		"events_dropped_total":           float64(2),
		"executor_launch_success_total":  float64(5),
		"executor_launch_failure_total":  float64(0),
		"executor_crash_total":           float64(0),
		"ipc_decode_errors_total":        float64(1),
		"lode_write_success_total":       float64(50),
		"lode_write_failure_total":       float64(0),
		"lode_write_retry_total":         float64(3),
		"policy":                         "buffered",
		"executor":                       "my-executor.js",
		"storage_backend":                "s3",
		"run_id":                         "run-abc",
		"job_id":                         "job-def",
		"dropped_by_type":                map[string]any{"log": float64(2)},
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunsStarted != 5 {
		t.Errorf("RunsStarted = %d, want 5", parsed.RunsStarted)
	}
	if parsed.RunsCompleted != 4 {
		t.Errorf("RunsCompleted = %d, want 4", parsed.RunsCompleted)
	}
	if parsed.RunsFailed != 1 {
		t.Errorf("RunsFailed = %d, want 1", parsed.RunsFailed)
	}
	if parsed.EventsReceived != 100 {
		t.Errorf("EventsReceived = %d, want 100", parsed.EventsReceived)
	}
	if parsed.EventsPersisted != 98 {
		t.Errorf("EventsPersisted = %d, want 98", parsed.EventsPersisted)
	}
	if parsed.EventsDropped != 2 {
		t.Errorf("EventsDropped = %d, want 2", parsed.EventsDropped)
	}
	if parsed.IPCDecodeErrors != 1 {
		t.Errorf("IPCDecodeErrors = %d, want 1", parsed.IPCDecodeErrors)
	}
	if parsed.LodeWriteSuccess != 50 {
		t.Errorf("LodeWriteSuccess = %d, want 50", parsed.LodeWriteSuccess)
	}
	if parsed.LodeWriteRetry != 3 {
		t.Errorf("LodeWriteRetry = %d, want 3", parsed.LodeWriteRetry)
	}
	if parsed.Policy != "buffered" {
		t.Errorf("Policy = %q, want %q", parsed.Policy, "buffered")
	}
	if parsed.Executor != "my-executor.js" {
		t.Errorf("Executor = %q, want %q", parsed.Executor, "my-executor.js")
	}
	if parsed.StorageBackend != "s3" {
		t.Errorf("StorageBackend = %q, want %q", parsed.StorageBackend, "s3")
	}
	if parsed.RunID != "run-abc" {
		t.Errorf("RunID = %q, want %q", parsed.RunID, "run-abc")
	}
	if parsed.JobID != "job-def" {
		t.Errorf("JobID = %q, want %q", parsed.JobID, "job-def")
	}
	if parsed.DroppedByType == nil {
		t.Fatal("DroppedByType should not be nil")
	}
	if parsed.DroppedByType["log"] != 2 {
		t.Errorf("DroppedByType[log] = %d, want 2", parsed.DroppedByType["log"])
	}
}

func TestParseMetricsRecord_NilRecord(t *testing.T) {
	_, err := ParseMetricsRecord(nil)
	if err == nil {
		t.Error("expected error for nil record")
	}
}

func TestParseMetricsRecord_MissingFields(t *testing.T) {
	// Empty record should parse without error (all zeros/empty)
	record := map[string]any{
		"record_kind": "metrics",
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunsStarted != 0 {
		t.Errorf("RunsStarted = %d, want 0", parsed.RunsStarted)
	}
	if parsed.Policy != "" {
		t.Errorf("Policy = %q, want empty", parsed.Policy)
	}
}
