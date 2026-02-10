# Quarry Integration Contract

This document defines the **integration boundary** for downstream notification
adapters and orchestration integrations used by the Quarry runtime.

This is a contract document. Implementations must conform.

---

## Scope

- Defines the distinction between notification adapters and orchestration integrations.
- Defines the adapter boundary and ownership.
- Defines CLI/config selection rules.
- Defines delivery semantics and required observability.

Non-goals:
- Does not define executor behavior.
- Does not define user script behavior.
- Does not define provider-specific configuration details.
- Does not define Temporal workflow semantics (see `guides/temporal.md`).

---

## Integration Paradigms

Quarry supports two distinct integration paradigms:

| Paradigm | Relationship to Runtime | Direction | Examples |
|----------|------------------------|-----------|----------|
| Notification Adapter | Invoked **by** runtime after a run | Runtime → External | Webhook, Redis, NATS, SNS |
| Orchestration Integration | **Wraps** the runtime as embedded activity | External → Runtime | Temporal |

**Notification adapters** are in-process modules that fire after a run
completes. They implement the `Adapter` interface, are selected via
`--adapter` flags, and follow best-effort delivery semantics. The runtime
owns their lifecycle.

**Orchestration integrations** embed the runtime as a unit of work within
an external orchestrator. The orchestrator owns scheduling, retries, and
delivery guarantees. Orchestration integrations do NOT implement the
`Adapter` interface and are NOT selected via `--adapter` flags — they
are separate binaries that depend on the runtime as a library.

---

## Adapter Model

- Adapters are in-repo modules under `quarry/adapter/`.
- The runtime owns adapter lifecycle and selection.
- Users do not write runtime code; users only provide configuration.
- Adapter notification is **best-effort**: failures are logged to stderr
  but do not fail the run. Run data is already persisted before the
  adapter is invoked.

### Notification Adapters (v0.5.0+)

| Adapter | Package | Status |
|---------|---------|--------|
| Webhook (HTTP POST) | `quarry/adapter/webhook` | Available |
| Redis (Pub/Sub) | `quarry/adapter/redis` | Available |
| NATS | — | Planned |
| SNS | — | Planned |

> Temporal is an orchestration integration, not a notification adapter.
> See [Orchestration Integration Semantics](#orchestration-integration-semantics)
> and `guides/temporal.md`.

---

## Selection and Configuration

### Selection
- Adapter selection is runtime-owned and CLI/config-driven.
- Per-run selection via `quarry run --adapter <type>` flags is the baseline.
- Global defaults via config are optional and additive.
- No silent fallback to a different adapter is permitted.
- If `--adapter` is not set, no notification is sent.

### Configuration
- Adapters must accept configuration only from CLI/config inputs.
- Sensitive fields must be redacted from logs and output.

### CLI Flags

| Flag | Description |
|------|-------------|
| `--adapter <type>` | Adapter type (`webhook`, `redis`) |
| `--adapter-url <url>` | Endpoint URL (required when `--adapter` is set) |
| `--adapter-header <key=value>` | Custom HTTP header (repeatable, webhook only) |
| `--adapter-channel <name>` | Pub/sub channel name (redis only, default `quarry:run_completed`) |
| `--adapter-timeout <duration>` | Notification timeout (default `10s`) |
| `--adapter-retries <n>` | Retry attempts (default `3`) |

---

## Delivery Semantics

- Delivery is **best-effort with retries**. The adapter retries on
  transient failures (5xx, network errors) but may ultimately fail.
  A failed publish is logged to stderr; the run outcome is unaffected.
- On success, delivery may be duplicated (retries after ambiguous
  failure). Consumers should use `run_id` as the idempotency key.

Adapters must not:
- alter the event payload,
- silently drop events without observable failure.

---

## Strategy Surface (v0.3.0+)

Strategy is a contract-defined enum limited to batching and retries.

### Batching
Allowed values:
- `none`
- `fixed_count`
- `fixed_time`

### Retries
Allowed values:
- `none`
- `bounded`
- `infinite` (if supported)

Ordering and fan-out strategies are explicitly out of scope for now.

---

## Invocation Ordering

The adapter is invoked **after** all of the following:
1. Run execution completes (success or failure)
2. Policy flush completes
3. Metrics are persisted to Lode

This ensures consumers can read the data referenced in the event payload.
Adapter publish is the last step before CLI output and exit.

---

## Failure and Backpressure

- Adapter failures must be observable via stderr warnings.
- Adapter failure does not change the run exit code.
- Backpressure must block or fail explicitly; no silent loss is permitted.

---

## Security and Redaction

- Credentials must never be emitted in events, logs, or CLI output.
- Any adapter-specific secrets must be redacted at the boundary.

---

## Orchestration Integration Semantics

This section defines normative requirements for orchestration integrations
that embed the Quarry runtime as a unit of work.

### Activity Boundary

The orchestrator invokes `runtime.NewRunOrchestrator(config).Execute(ctx)`
as an atomic unit of work. The runtime owns everything inside this boundary
(executor lifecycle, IPC, policy, Lode persistence). The orchestrator owns
everything outside (scheduling, retries, fan-out, notification).

### Outcome Mapping

Orchestration integrations must map `OutcomeStatus` (from `types/lineage.go`)
to orchestrator-native error semantics:

| OutcomeStatus | Orchestrator Behavior |
|---------------|----------------------|
| `success` | Activity completes normally |
| `script_error` | Retryable error |
| `executor_crash` | Retryable error |
| `policy_failure` | Non-retryable error |
| `version_mismatch` | Non-retryable error |

`policy_failure` and `version_mismatch` are non-retryable because they
indicate systemic configuration problems that retries cannot resolve.

### Lineage Preservation

The orchestrator must set `RunMeta` fields per `RunMeta.Validate()` rules:

- Each execution gets a unique `run_id`.
- `attempt` starts at 1 and increments on each retry.
- `parent_run_id` links retries to the previous execution's `run_id`.
- `attempt == 1` must have `parent_run_id == nil` (initial run).
- `attempt > 1` must have `parent_run_id != nil` (retry run).

### Heartbeat Contract

Orchestration integrations should heartbeat at a recommended interval of
30 seconds. Context cancellation from the orchestrator must trigger:

1. Executor kill (best-effort).
2. Policy flush (best-effort).

### Data Flow

Only orchestration metadata flows through the orchestrator (~10KB: config
in, result summary out). Run data (events, artifacts, metrics) stays in
Lode. This keeps orchestrator payloads small and avoids serialization
limits.

---

## Versioning

- Additive changes only during 0.x.
- Renames or semantic changes are breaking and forbidden in 0.x.
