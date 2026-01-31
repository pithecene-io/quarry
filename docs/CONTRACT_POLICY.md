# Quarry Ingestion Policy Semantics

This document freezes the semantics expected of ingestion policies.

This is a contract document. Implementations must conform.

---

## Scope

- Defines what a "drop" means.
- Defines which event types may be dropped.
- Defines buffering expectations.
- Defines required observability.

Non-goals:
- Does not define policy implementations.
- Does not define storage layout (see CONTRACT_LODE.md).

---

## Definitions

### Drop
An event is **dropped** when it is observed by the runtime but **not persisted**
or forwarded to storage, and will not be retried by the policy.

### Buffered
An event is **buffered** when it is held in memory by the policy before
being written to storage. Buffered data is **bounded**.

---

## Drop Rules

- Policies may only drop:
  - `log`
  - `enqueue`
  - `rotate_proxy`
- Policies must **not** drop:
  - `item`
  - `artifact`
  - `checkpoint`
  - `run_error`
  - `run_complete`

If a policy cannot accept non-droppable events, it must fail the run.

---

## Buffering Rules

- Buffers must be **bounded** and observable.
- Buffered events must be flushed on:
  - `run_complete`
  - `run_error`
  - runtime termination (best effort)
- Buffering must never reorder events.

---

## Flush Modes

Buffered policies support configurable flush semantics via `FlushMode`:

### `at_least_once` (default)

- Writes events, then chunks.
- On any failure, **preserves all buffers**.
- Retry may cause duplicate writes.
- Guarantees no data loss.

### `chunks_first`

- Writes chunks before events.
- If chunks fail, events are not attempted (no duplicates for events).
- If chunks succeed but events fail, chunks may duplicate on retry.
- Clears only successfully written buffers.

### `two_phase`

- Tracks per-buffer success to minimize duplicates.
- Events written successfully are marked; not re-written on retry.
- Events added after partial flush go to a secondary buffer.
- Most complex; requires internal state tracking.

All modes must satisfy the no-silent-loss invariant.

---

## Required Observability

Policies must surface:

- Counters
  - total events received
  - total events persisted
  - total events dropped (by type)
- Logs
  - drop reasons (if any)
  - buffer overflows or flush failures
- Events
  - optional policy-level stats event

---

## Invariants

- No silent loss.
- Policy does not alter event shapes.
- Policy must respect the per-run ordering guarantees.
