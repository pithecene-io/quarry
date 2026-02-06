package lode

import (
	"context"
	"fmt"

	"github.com/justapithecus/lode/lode"

	"github.com/justapithecus/quarry/cli/reader"
)

// ErrNoMetricsFound is returned when no metrics records exist in the dataset.
var ErrNoMetricsFound = fmt.Errorf("no metrics records found")

// QueryLatestMetrics finds and reads the most recent metrics record from Lode.
// Filters by runID and source if non-empty.
// Returns the raw record map or ErrNoMetricsFound if none exist.
func QueryLatestMetrics(ctx context.Context, ds lode.Dataset, runID, source string) (map[string]any, error) {
	snapshots, err := ds.Snapshots(ctx)
	if err != nil {
		return nil, WrapReadError(err, "quarry/snapshots")
	}

	// Iterate in reverse (latest first) — snapshots are ordered by creation time
	for i := len(snapshots) - 1; i >= 0; i-- {
		snap := snapshots[i]

		if !isMetricsSnapshot(snap) {
			continue
		}
		if !snapshotMatchesFilter(snap, "run_id", runID) {
			continue
		}
		if !snapshotMatchesFilter(snap, "source", source) {
			continue
		}

		// Read snapshot data
		data, err := ds.Read(ctx, snap.ID)
		if err != nil {
			return nil, WrapReadError(err, fmt.Sprintf("quarry/snapshot/%s", snap.ID))
		}

		// Find a metrics record that passes record-level filters.
		// Manifest path filtering is a coarse pre-filter; record fields
		// are authoritative (handles cumulative/multi-record snapshots).
		for _, item := range data {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if record["record_kind"] != RecordKindMetrics {
				continue
			}
			if runID != "" && toString(record["run_id"]) != runID {
				continue
			}
			if source != "" && toString(record["source"]) != source {
				continue
			}
			return record, nil
		}
	}

	return nil, ErrNoMetricsFound
}

// ParseMetricsRecord converts a Lode record (map[string]any) to a reader.MetricsSnapshot.
// Handles both int64 (direct writes) and float64 (JSON round-trips) for numeric fields.
func ParseMetricsRecord(record map[string]any) (*reader.MetricsSnapshot, error) {
	if record == nil {
		return nil, fmt.Errorf("nil record")
	}

	snap := &reader.MetricsSnapshot{
		Ts: toString(record["ts"]),

		// Run lifecycle
		RunsStarted:   toInt64(record["runs_started_total"]),
		RunsCompleted: toInt64(record["runs_completed_total"]),
		RunsFailed:    toInt64(record["runs_failed_total"]),
		RunsCrashed:   toInt64(record["runs_crashed_total"]),

		// Ingestion
		EventsReceived:  toInt64(record["events_received_total"]),
		EventsPersisted: toInt64(record["events_persisted_total"]),
		EventsDropped:   toInt64(record["events_dropped_total"]),

		// Executor
		ExecutorLaunchSuccess: toInt64(record["executor_launch_success_total"]),
		ExecutorLaunchFailure: toInt64(record["executor_launch_failure_total"]),
		ExecutorCrash:         toInt64(record["executor_crash_total"]),
		IPCDecodeErrors:       toInt64(record["ipc_decode_errors_total"]),

		// Lode / Storage
		LodeWriteSuccess: toInt64(record["lode_write_success_total"]),
		LodeWriteFailure: toInt64(record["lode_write_failure_total"]),
		LodeWriteRetry:   toInt64(record["lode_write_retry_total"]),

		// Dimensions
		Policy:         toString(record["policy"]),
		Executor:       toString(record["executor"]),
		StorageBackend: toString(record["storage_backend"]),
		RunID:          toString(record["run_id"]),
		JobID:          toString(record["job_id"]),
	}

	// Parse dropped_by_type if present
	if dbt, ok := record["dropped_by_type"]; ok && dbt != nil {
		snap.DroppedByType = parseDroppedByType(dbt)
	}

	// Validate contract-required fields per CONTRACT_CLI.md.
	// The write path always populates these; missing values indicate
	// data corruption or a malformed record.
	if snap.Ts == "" {
		return nil, fmt.Errorf("metrics record missing required field: ts")
	}
	if snap.RunID == "" {
		return nil, fmt.Errorf("metrics record missing required field: run_id")
	}
	if snap.Policy == "" {
		return nil, fmt.Errorf("metrics record missing required field: policy")
	}

	return snap, nil
}

// toInt64 converts a value to int64, handling float64 from JSON and int64 from direct writes.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	default:
		return 0
	}
}

// toString converts a value to string, returning empty string for nil/non-string.
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// parseDroppedByType converts dropped_by_type from Lode record format.
// Handles both map[string]int64 (direct) and map[string]any (JSON round-trip).
func parseDroppedByType(v any) map[string]int64 {
	switch m := v.(type) {
	case map[string]int64:
		return m
	case map[string]any:
		result := make(map[string]int64, len(m))
		for k, val := range m {
			result[k] = toInt64(val)
		}
		return result
	default:
		return nil
	}
}
