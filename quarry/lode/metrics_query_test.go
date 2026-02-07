package lode

import (
	"errors"
	"strings"
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
	ds, err := NewReadDataset("quarry", factory)
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
	ds, err := NewReadDataset("quarry", factory)
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
	ds, err := NewReadDataset("quarry", factory)
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

	ds, err := NewReadDataset("quarry", factory)
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

	ds, err := NewReadDataset("quarry", factory)
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
		"ts":                             "2026-02-03T15:00:00Z",
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

	if parsed.Ts != "2026-02-03T15:00:00Z" {
		t.Errorf("Ts = %q, want %q", parsed.Ts, "2026-02-03T15:00:00Z")
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

func TestParseMetricsRecord_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		record map[string]any
		errMsg string
	}{
		{
			name:   "missing ts",
			record: map[string]any{"record_kind": "metrics", "run_id": "run-1", "policy": "strict", "executor": "e.js", "storage_backend": "fs"},
			errMsg: "ts",
		},
		{
			name:   "missing run_id",
			record: map[string]any{"record_kind": "metrics", "ts": "2026-02-03T15:00:00Z", "policy": "strict", "executor": "e.js", "storage_backend": "fs"},
			errMsg: "run_id",
		},
		{
			name:   "missing policy",
			record: map[string]any{"record_kind": "metrics", "ts": "2026-02-03T15:00:00Z", "run_id": "run-1", "executor": "e.js", "storage_backend": "fs"},
			errMsg: "policy",
		},
		{
			name:   "missing executor",
			record: map[string]any{"record_kind": "metrics", "ts": "2026-02-03T15:00:00Z", "run_id": "run-1", "policy": "strict", "storage_backend": "fs"},
			errMsg: "executor",
		},
		{
			name:   "missing storage_backend",
			record: map[string]any{"record_kind": "metrics", "ts": "2026-02-03T15:00:00Z", "run_id": "run-1", "policy": "strict", "executor": "e.js"},
			errMsg: "storage_backend",
		},
		{
			name:   "all required missing",
			record: map[string]any{"record_kind": "metrics"},
			errMsg: "ts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMetricsRecord(tt.record)
			if err == nil {
				t.Fatal("expected error for missing required field, got nil")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestParseMetricsRecord_Ts(t *testing.T) {
	record := map[string]any{
		"record_kind":    "metrics",
		"ts":             "2026-02-03T15:30:00Z",
		"run_id":         "run-1",
		"policy":         "strict",
		"executor":       "executor.js",
		"storage_backend": "fs",
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}
	if parsed.Ts != "2026-02-03T15:30:00Z" {
		t.Errorf("Ts = %q, want %q", parsed.Ts, "2026-02-03T15:30:00Z")
	}
}

// TestQueryLatestMetrics_RunIDSubstringNoCollision verifies that filtering
// by run_id=run-1 does not match run_id=run-10 (substring false positive).
func TestQueryLatestMetrics_RunIDSubstringNoCollision(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	completedAt := time.Date(2026, 2, 3, 15, 0, 0, 0, time.UTC)

	// Write metrics for run-1 and run-10
	for i, runID := range []string{"run-1", "run-10"} {
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

	ds, err := NewReadDataset("quarry", factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	// Filter by run-1 — must NOT match run-10
	record, err := QueryLatestMetrics(t.Context(), ds, "run-1", "")
	if err != nil {
		t.Fatalf("QueryLatestMetrics failed: %v", err)
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunID != "run-1" {
		t.Errorf("RunID = %q, want %q (must not match run-10)", parsed.RunID, "run-1")
	}
	if parsed.RunsStarted != 1 {
		t.Errorf("RunsStarted = %d, want 1", parsed.RunsStarted)
	}
}

// TestQueryLatestMetrics_SourceSubstringNoCollision verifies that filtering
// by source=alpha does not match source=alphabet.
func TestQueryLatestMetrics_SourceSubstringNoCollision(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	completedAt := time.Date(2026, 2, 3, 15, 0, 0, 0, time.UTC)

	// Write metrics for source=alpha and source=alphabet
	for i, source := range []string{"alpha", "alphabet"} {
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

	ds, err := NewReadDataset("quarry", factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	// Filter by source=alpha — must NOT match source=alphabet
	record, err := QueryLatestMetrics(t.Context(), ds, "", "alpha")
	if err != nil {
		t.Fatalf("QueryLatestMetrics failed: %v", err)
	}

	parsed, err := ParseMetricsRecord(record)
	if err != nil {
		t.Fatalf("ParseMetricsRecord failed: %v", err)
	}

	if parsed.RunsStarted != 1 {
		t.Errorf("RunsStarted = %d, want 1 (alpha source, not alphabet)", parsed.RunsStarted)
	}
}

// TestQueryLatestMetrics_RecordLevelFiltering verifies that record-level
// run_id filtering works when manifest paths might match broadly.
func TestQueryLatestMetrics_RecordLevelFiltering(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	completedAt := time.Date(2026, 2, 3, 15, 0, 0, 0, time.UTC)

	// Write metrics for run-abc
	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-abc",
	}

	client, err := NewLodeClientWithFactory(cfg, factory)
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	snap := metrics.Snapshot{
		RunsStarted:   5,
		RunID:          "run-abc",
		Policy:         "strict",
		Executor:       "executor.js",
		StorageBackend: "fs",
	}

	if err := client.WriteMetrics(t.Context(), snap, completedAt); err != nil {
		t.Fatalf("WriteMetrics failed: %v", err)
	}

	ds, err := NewReadDataset("quarry", factory)
	if err != nil {
		t.Fatalf("NewReadDataset failed: %v", err)
	}

	// Filter by a run_id that doesn't exist — should get ErrNoMetricsFound
	_, err = QueryLatestMetrics(t.Context(), ds, "run-nonexistent", "")
	if err == nil {
		t.Fatal("expected error for non-matching run_id filter, got nil")
	}
	if !errors.Is(err, ErrNoMetricsFound) {
		t.Errorf("expected ErrNoMetricsFound, got: %v", err)
	}
}

// TestQueryLatestMetrics_TsRoundTrip verifies ts survives write/read cycle.
func TestQueryLatestMetrics_TsRoundTrip(t *testing.T) {
	store := lode.NewMemory()
	factory := sharedFactory(store)

	cfg := Config{
		Dataset:  "quarry",
		Source:   "test-source",
		Category: "test-category",
		Day:      "2026-02-03",
		RunID:    "run-001",
	}

	client, err := NewLodeClientWithFactory(cfg, factory)
	if err != nil {
		t.Fatalf("NewLodeClientWithFactory failed: %v", err)
	}

	completedAt := time.Date(2026, 2, 3, 15, 30, 0, 0, time.UTC)
	snap := metrics.Snapshot{
		RunsStarted:   1,
		RunID:          "run-001",
		Policy:         "strict",
		Executor:       "executor.js",
		StorageBackend: "fs",
	}

	if err := client.WriteMetrics(t.Context(), snap, completedAt); err != nil {
		t.Fatalf("WriteMetrics failed: %v", err)
	}

	ds, err := NewReadDataset("quarry", factory)
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

	if parsed.Ts != "2026-02-03T15:30:00Z" {
		t.Errorf("Ts = %q, want %q", parsed.Ts, "2026-02-03T15:30:00Z")
	}
}
