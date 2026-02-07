# Quarry Event-Bus Integration Contract

This document defines the **event-bus adapter boundary** and the **selection
and delivery semantics** used by the Quarry runtime.

This is a contract document. Implementations must conform.

---

## Scope

- Defines the adapter boundary and ownership.
- Defines CLI/config selection rules.
- Defines delivery semantics and required observability.

Non-goals:
- Does not define executor behavior.
- Does not define user script behavior.
- Does not define provider-specific configuration details.

---

## Adapter Model

- Adapters are in-repo modules under `quarry/adapter/`.
- The runtime owns adapter lifecycle and selection.
- Users do not write runtime code; users only provide configuration.
- Adapter notification is **best-effort**: failures are logged to stderr
  but do not fail the run. Run data is already persisted before the
  adapter is invoked.

### Runtime Adapters (v0.5.0+)

| Adapter | Package | Status |
|---------|---------|--------|
| Webhook (HTTP POST) | `quarry/adapter/webhook` | Available |
| Temporal | — | Planned |
| NATS | — | Planned |
| SNS | — | Planned |

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
| `--adapter <type>` | Adapter type (`webhook`) |
| `--adapter-url <url>` | Endpoint URL (required when `--adapter` is set) |
| `--adapter-header <key=value>` | Custom HTTP header (repeatable) |
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

## Versioning

- Additive changes only during 0.x.
- Renames or semantic changes are breaking and forbidden in 0.x.
