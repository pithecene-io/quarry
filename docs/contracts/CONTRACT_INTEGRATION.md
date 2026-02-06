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

- Adapters are in-repo modules.
- The runtime owns adapter lifecycle and selection.
- Users do not write runtime code; users only provide configuration.

---

## Selection and Configuration

### Selection
- Adapter selection is runtime-owned and CLI/config-driven.
- Per-run selection via `quarry run` flags is the baseline.
- Global defaults via config are optional and additive.
- No silent fallback to a different adapter is permitted.

### Configuration
- Adapters must accept configuration only from CLI/config inputs.
- Sensitive fields must be redacted from logs and output.

---

## Delivery Semantics

- Delivery is **at-least-once**.
- Dedupe guidance: consumers should use `run_id + seq` as the idempotency key.

Adapters must not:
- reorder events within a run,
- alter the emit envelope,
- drop non-droppable events.

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

## Failure and Backpressure

- Adapter failures must be observable via metrics and CLI stats.
- Backpressure must block or fail explicitly; no silent loss is permitted.

---

## Security and Redaction

- Credentials must never be emitted in events, logs, or CLI output.
- Any adapter-specific secrets must be redacted at the boundary.

---

## Versioning

- Additive changes only during 0.x.
- Renames or semantic changes are breaking and forbidden in 0.x.
