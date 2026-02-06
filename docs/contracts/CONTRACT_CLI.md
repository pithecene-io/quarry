# Quarry CLI Contract

This document freezes the **CLI surface** and **read-only guarantees** for
Quarry. It defines the command topology, required invariants, and the
request/response shapes exposed by the runtime.

This is a contract document. Implementations must conform.

---

## Scope

- Defines CLI invariants and non-goals.
- Defines command topology and required subcommands.
- Defines output format selection and rendering rules.
- Defines request/response shapes for CLI-visible operations.

Non-goals:
- Does not define executor behavior.
- Does not define ingestion policy implementation details.
- Does not define storage layouts (see CONTRACT_LODE.md).

---

## Invariants

- The CLI is the **only execution entrypoint**.
- All commands except `run` are **read-only** with respect to execution,
  datasets, and persisted state.
- A small set of `debug` commands may mutate **ephemeral runtime mechanics**
  only when explicitly requested and documented.
- The CLI talks **only** to the Go runtime.
- The executor never exposes CLI surfaces.
- No command starts a long-running background process.
- No scheduler or control-plane commands are permitted.

---

## Command Topology

```
quarry
├─ run
├─ inspect
│  ├─ run <run-id>
│  ├─ job <job-id>
│  ├─ task <task-id>
│  ├─ proxy <pool-name>
│  └─ executor <executor-id>
├─ stats
│  ├─ runs
│  ├─ jobs
│  ├─ tasks
│  ├─ proxies
│  ├─ executors
│  └─ metrics
├─ list
│  ├─ runs
│  ├─ jobs
│  ├─ pools
│  └─ executors
├─ debug
│  ├─ resolve proxy <pool>
│  └─ ipc
└─ version
```

Forbidden commands:
- `status`
- `admin`
- `control`
- `serve`

---

## Output and Rendering

All `inspect`, `stats`, and `list` commands share the same output contract.

### Supported Formats

- `json` (canonical, stable)
- `table` (human-readable)
- `yaml` (debugging and config inspection)

### Format Selection Rules

- If output is a TTY, default to `table`.
- If output is not a TTY, default to `json`.
- A user-specified `--format` always overrides defaults.
- Invalid formats are errors.

### Shared Flags

- `--format=json|table|yaml`
- `--no-color` (affects table output only, not TUI)
- `--tui` (inspect and stats commands only)

Rendering is centralized. Command handlers must not implement custom
formatting logic.

### TUI Mode

TUI mode (`--tui`) provides interactive views for select read-only commands.

**TUI Policy:**

- **Opt-in only**: TUI is never the default. Users must explicitly request it.
- **Read-only only**: TUI is only available for `inspect` and `stats` commands.
- **Same data payloads**: TUI renders the same data structures as non-TUI output.
  There is no TUI-exclusive data.
- **Presentation only**: Bubble Tea is a presentation layer. It does not fetch
  additional data or perform mutations.
- **Clean exit**: TUI must exit cleanly on `q` or `Ctrl+C`.

Commands that do not support TUI must error if `--tui` is provided.

Supported TUI commands:
- `inspect run|job|task|proxy|executor`
- `stats runs|jobs|tasks|proxies|executors|metrics`

Unsupported (must error with `--tui`):
- `list` commands
- `debug` commands
- `version`
- `run`

---

## `run` (execution)

`quarry run` is the **only** command that executes work.
It must conform to CONTRACT_RUN.md and emit observability defined by
CONTRACT_EMIT.md and CONTRACT_POLICY.md.

No other command may initiate or control execution.

---

## `inspect` (single-entity introspection)

`inspect` commands return a deep view of a single entity and are read-only.
Data is sourced from the runtime’s read path (see CONTRACT_LODE.md).

### `inspect run <run-id>`

Request:
```
InspectRunRequest:
  run_id: string
```

Response:
```
InspectRunResponse:
  run_id: string
  job_id: string
  state: string
  attempt: number
  parent_run: string | null
  policy: string
  started_at: time
  ended_at: time | null
```

### `inspect job <job-id>`

Response must include:
- `job_id`
- `state`
- `run_ids` (if any)

### `inspect task <task-id>`

Response must include:
- `task_id`
- `state`
- `run_id` (if any)

### `inspect proxy <pool-name>`

Response:
```
InspectProxyPoolResponse:
  name: string
  strategy: string
  endpoint_cnt: number
  sticky: ProxySticky | null
  runtime:
    round_robin_index: number
    sticky_entries: number
    last_used_at: time | null
```

This response reinforces that **proxy selection is runtime-owned** and
the executor only applies resolved endpoints (see CONTRACT_PROXY.md).

### `inspect executor <executor-id>`

Response must include:
- `executor_id`
- `state`
- `last_seen_at` (time | null)

---

## `stats` (aggregated facts)

`stats` commands return aggregated, derived facts. They must not expose
internal counters from executors. Stats are derived from Lode.

### `stats runs`

Response:
```
RunStats:
  total: number
  running: number
  succeeded: number
  failed: number
```

### `stats proxies`

Response:
```
ProxyStats:
  pool: string
  requests: number
  failures: number
  last_used_at: time | null
```

### `stats jobs`

Response must include:
- `total`
- `running`
- `succeeded`
- `failed`

### `stats tasks`

Response must include:
- `total`
- `running`
- `succeeded`
- `failed`

### `stats executors`

Response must include:
- `total`
- `running`
- `idle`
- `failed`

### `stats metrics`

Response:
```
MetricsSnapshot:
  runs_started_total: number
  runs_completed_total: number
  runs_failed_total: number
  runs_crashed_total: number
  events_received_total: number
  events_persisted_total: number
  events_dropped_total: number
  dropped_by_type: map[string]number (optional)
  executor_launch_success_total: number
  executor_launch_failure_total: number
  executor_crash_total: number
  ipc_decode_errors_total: number
  lode_write_success_total: number
  lode_write_failure_total: number
  lode_write_retry_total: number
  policy: string
  executor: string
  storage_backend: string
  run_id: string
  job_id: string (optional)
```

Metric names match CONTRACT_METRICS.md. Dimensions are included for
traceability. Data source progression per CONTRACT_METRICS.md §Exposure
Requirements: stub-backed in v0.3.0, Lode-backed post-v0.3.0.

---

## `list` (enumeration)

`list` commands return thin slices (not inspect-level detail).

### `list runs`

Supported filters:
- `--state=running|failed|succeeded`
- `--limit=<n>`

Response must include:
- `run_id`
- `state`
- `started_at`

### `list jobs`

Response must include:
- `job_id`
- `state`

### `list pools`

Response must include:
- `name`
- `strategy`

### `list executors`

Response must include:
- `executor_id`
- `state`

---

## `debug` (sharp tools, opt-in)

`debug` commands are opt-in diagnostic tools and **must not execute work**.
They are read-only by default. Any mutation must be:
- explicitly requested by a flag
- ephemeral and non-persistent
- non-executing (no runs affected or started)
- observable in logs and inspection surfaces
- limited to runtime mechanics (never datasets, runs, or policies)

### `debug resolve proxy <pool>`

Purpose:
- test selector logic
- validate configuration
- reason about rotation

Response:
```
ResolveProxyResponse:
  endpoint: ProxyEndpoint
  committed: boolean
```

The `committed` field indicates whether the resolution was committed (rotation
counters advanced). It is `false` by default and `true` only when `--commit`
is provided.

By default this command is read-only and **must not** mutate runtime state.

Optional flag:
- `--commit`

If `--commit` is provided, the runtime **may** advance in-memory rotation
counters. This mutation is:
- ephemeral
- non-persistent
- non-executing
- local to proxy selection mechanics

It must not write to Lode, change datasets, or affect runs or policies.
This is the only allowed mutation outside `run` and must be observable in
logs and inspection surfaces.

### `debug ipc`

Response:
```
IPCDebugResponse:
  transport: string
  encoding: string
  messages_sent: number
  errors: number
  last_error: string | null
```

No payload dumping unless `--verbose` is provided.

---

## `version`

`version` reports the canonical project version.
It must not contact the executor.

Response:
```
VersionResponse:
  version: string
  commit: string
```

### Lockstep Versioning

All Quarry components share a single canonical version defined in `types.Version`.
This includes:
- CLI version
- Emit contract version (CONTRACT_EMIT.md)
- IPC contract version (CONTRACT_IPC.md)

Version changes require updating the single source constant. There is no separate
"contract version" — the project version is the contract version.
