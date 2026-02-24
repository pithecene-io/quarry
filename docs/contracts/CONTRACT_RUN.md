# Quarry Run Identity & Lineage

This document freezes the identity and lineage rules for runs.

This is a contract document. Implementations must conform.

---

## Definitions

- **Job**: A logical unit of work requested by a user or scheduler.
- **Run**: A single execution attempt of a job.
- **Retry run**: A new run created due to failure of a previous run.

Relationship:

`job` → `run` → `retry run`

Runs are append-only and immutable once complete.

---

## Canonical Identifiers

### `run_id`
- Unique, stable identifier for a run.
- Must be globally unique across all time and jobs.
- Appears in every event envelope.
- run_id generation strategy is implementation-defined but must be collision-resistant (e.g., UUIDv7, ULID).

### `job_id`
- Identifier for the logical job.
- Stable across retries.
- Optional in the emit envelope if unknown at emit-time.

---

## Lineage Fields

When applicable, the following fields must be set:

- `parent_run_id`
  - Set when the current run is a retry or child run.
  - For first attempt runs, this field is absent.

- `attempt`
  - Integer attempt number. **Always present.**
  - Starts at **1** for the initial run of a job.
  - Incremented by 1 for each retry run.
  - A run with `attempt: 1` and no `parent_run_id` is an initial run.

### Child Runs (v0.6.0+)

When fan-out is active (`--depth > 0`), the runtime may create child runs
in response to `enqueue` events. Child runs have:

- A unique `run_id` (distinct from the parent).
- `attempt: 1` (child runs are first attempts of derived work).
- Depth tracked internally by the fan-out operator (not in the envelope).

Child runs are **not** retries. They represent derived work from a different
script, not a re-execution of the same job.

---

## Lifecycle Hooks (v0.9.0+)

Scripts may export optional lifecycle hooks alongside the default run function.
Hooks are invoked by the executor at well-defined points in the run lifecycle.

### Execution Order

```
load script
    │
    ▼
[prepare]  ──skip──▶ emit run_complete({skipped}) → run_result → return
    │
    │ continue (optionally with transformed job)
    ▼
acquire browser → create context
    │
    ▼
[beforeRun] → script() → [afterRun] (success) / [onError] (error)
    │
    ▼
[beforeTerminal]  (emit still open, terminal not yet written)
    │
    ▼
auto-emit terminal → [cleanup] → run_result → return
```

### `prepare`

- Invoked **before** browser acquisition.
- Receives the raw job payload and run metadata.
- Returns `{ action: 'continue' }` to proceed, optionally with a
  transformed `job` field that replaces the original payload.
- Returns `{ action: 'skip', reason? }` to short-circuit the run.
  On skip, the executor emits `run_complete` with `{ skipped: true }` in
  the summary and returns immediately. **No browser is launched** and no
  downstream hooks (`beforeRun`, `afterRun`, `onError`, `beforeTerminal`,
  `cleanup`) are invoked.
- If `prepare` throws, the run is classified as a crash. No browser is launched.

### `beforeTerminal`

- Invoked **after** `afterRun`/`onError` but **before** the terminal event
  (`run_complete` or `run_error`) is auto-emitted.
- The emit channel is still open; the hook may emit items, logs, or artifacts.
- Receives a `TerminalSignal`: `{ outcome: 'completed' }` on success,
  `{ outcome: 'error', error }` on script failure.
- **Not invoked** when the script has already emitted a terminal event
  manually, or when the IPC sink has failed.
- If `beforeTerminal` throws, the error is swallowed and the terminal event
  is still emitted (consistent with `onError`/`cleanup` error handling).

### Hook Contract Rules

- All hooks are optional. Scripts that do not export hooks behave identically
  to pre-0.9.0.
- Hook exports must be functions. Non-function exports of hook names cause
  a load-time validation error.
- `cleanup` is **not called** on `prepare`-skip paths (no context exists).

---

## Idempotency Expectations

- Runs are **append-only**; no event mutation is allowed after emission.
- The runtime and policies must not attempt to deduplicate events.
- Deduplication is the responsibility of **downstream consumers**.

---

## Observability Requirements

For every run, the runtime must surface:
- `run_id`
- `job_id` (if known)
- `parent_run_id` (if applicable)
- `attempt` (if applicable)
- outcome status (success, script error, executor crash, policy failure, version mismatch)

This metadata must be available to storage and logs.

---

## Structured Exit Report (v0.11.0+)

When `--report <path>` is specified, the runtime writes a structured JSON
report on exit. `--report -` writes to stderr.

The report is written after metrics persistence and adapter notification.
All data is composed from `RunResult` and `metrics.Snapshot` — no additional
data fetching occurs.

### Report JSON Structure

```json
{
  "run_id": "string",
  "job_id": "string (omitted if empty)",
  "attempt": 1,
  "outcome": "success | script_error | executor_crash | policy_failure | version_mismatch",
  "message": "string",
  "exit_code": 0,
  "duration_ms": 12345,
  "event_count": 42,
  "policy": {
    "name": "strict | buffered | streaming",
    "events_received": 42,
    "events_persisted": 42,
    "events_dropped": 0,
    "flush_triggers": { "interval": 3, "termination": 1 }
  },
  "artifacts": {
    "total": 5,
    "committed": 5,
    "orphaned": 0,
    "chunks": 10,
    "bytes": 524288
  },
  "metrics": { "/* CONTRACT_METRICS counters */" : "..." },
  "terminal_summary": { "/* from terminal event payload, if any */" : "..." },
  "proxy_used": {
    "protocol": "http",
    "host": "proxy.example.com",
    "port": 8080,
    "username": "user (omitted if none)"
  },
  "stderr": "string (omitted if empty)"
}
```

### Field Rules

- `run_id`, `attempt`, `outcome`, `message`, `exit_code`, `duration_ms`,
  `event_count`, `policy`, `artifacts`, `metrics` are always present.
- `job_id` is omitted when empty.
- `terminal_summary` is omitted when no terminal event was received.
- `proxy_used` is omitted when no proxy was configured.
- `stderr` is omitted when empty.
- `policy.flush_triggers` is omitted for non-streaming policies.
- `exit_code` matches the process exit code per §Exit Codes in CONTRACT_CLI.md.
