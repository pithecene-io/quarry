# Quarry Lode Expectations

This document freezes Quarry's minimal expectations of Lode.

This is a contract document. Implementations must conform.

---

## Scope

- Defines required partition keys and layout invariants.
- Defines append-only semantics.
- Defines meaning of "consistency across policies".

Non-goals:
- Does not define Lode internal schema or storage engine.
- Does not define deduplication (downstream responsibility).

---

## Required Partition Keys

Lode must support partitioning by:
- `dataset` (or logical collection)
- `run_id`
- `event_type`

Additional keys are allowed, but the above must be present.

---

## Append-Only Semantics

- Events are **append-only**.
- No updates or deletes are required or expected.
- Event order within a run must be preserved.

---

## Policy-Independent Layout Invariants

Storage layout must remain consistent across policies so that:
- A strict policy run and a buffered policy run produce data in the
  **same partition layout**.
- Downstream consumers can query without knowing policy choice.

---

## Consistency Across Policies

"Consistency across policies" means:
- Identical partition keys.
- Identical event envelopes.
- Identical event ordering within a run.
- Explicit visibility if events were dropped (via policy observability).

---

## Lineage and Metadata

Lode must surface run metadata alongside stored events:
- `run_id`
- `job_id` (if known)
- `parent_run_id` (if applicable)
- `attempt` (if applicable)
- run outcome status

Deduplication is explicitly out of scope and left to downstream consumers.
