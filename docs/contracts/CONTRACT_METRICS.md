# Quarry Metrics Contract

This document defines the **metrics surface** required of the Quarry runtime.
It freezes the **metric names, semantics, and required visibility** for
CLI stats and operational observability.

This is a contract document. Implementations must conform.

---

## Scope

- Defines required runtime metrics and their meanings.
- Defines required dimensions and aggregation expectations.
- Defines exposure requirements via CLI stats.

Non-goals:
- Does not define exporter implementations (Prometheus, OTLP, etc.).
- Does not define storage schema or persistence format.
- Does not define UI or visualization.

---

## Definitions

### Metric Categories
- **Counter**: monotonically increasing.
- **Gauge**: current value snapshot.
- **Histogram**: optional distribution buckets (if supported).

### Time Horizons
- **Per-run**: metrics scoped to a single run.
- **Lifetime**: metrics aggregated since process start.
- **Rolling**: windowed metrics (optional).

### Cardinality Rules
- Metrics must avoid unbounded label sets.
- `run_id` and `job_id` are optional dimensions and should be used only for
  CLI inspection or short-lived aggregation.

---

## Required Metrics

### Run Lifecycle
- `runs_started_total` (counter)
- `runs_completed_total` (counter)
- `runs_failed_total` (counter)
- `runs_crashed_total` (counter)

### Ingestion Policy
- `events_received_total` (counter)
- `events_persisted_total` (counter)
- `events_dropped_total` (counter, by event type)

### Executor
- `executor_launch_success_total` (counter)
- `executor_launch_failure_total` (counter)
- `executor_crash_total` (counter)
- `ipc_decode_errors_total` (counter)

### Lode / Storage
- `lode_write_success_total` (counter)
- `lode_write_failure_total` (counter)
- `lode_write_retry_total` (counter)
- `lode_write_latency_ms` (histogram, optional)

---

## Required Dimensions

All metrics must support the following dimensions where applicable:
- `policy` (required)
- `executor` (required)
- `storage_backend` (required)

Optional dimensions:
- `run_id`
- `job_id`
- `adapter` (only when integrations exist)

---

## Exposure Requirements

- CLI `stats` commands must surface stable, aggregated views of required metrics.
- Metrics must be derived from runtime-owned sources, not executor counters.
- Metrics snapshots must be persisted as Lode `record_kind=metrics` records
  (see CONTRACT_LODE.md) to support stats reads across processes.
- No exporter is required for v0.3.0; exposure via CLI is mandatory.

### Data Source Progression

During 0.x, stats commands may return stub data when a Lode-backed reader
is not yet implemented. This is a transitional allowance, not a permanent state.

Requirements by milestone:
- **v0.3.0**: Write path (metrics persisted to Lode). Read path stub-backed.
  CLI output uses correct response shapes and metric names.
- **Post-v0.3.0**: Read path wired to Lode. `stats metrics` returns real
  persisted data. Stub reader retained only for testing.

---

## Invariants

- No silent loss: drops must be counted and visible.
- Metrics must not alter runtime behavior.
- Metric names and meanings must be consistent across policies.

---

## Versioning

- Additive changes only during 0.x.
- Renames or semantic changes are breaking and forbidden in 0.x.
