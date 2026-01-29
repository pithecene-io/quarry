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
  - Integer retry count.
  - Starts at 1 for the first run of a job.
  - Incremented for each retry run.

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
- outcome status (success, script error, executor crash, policy failure)

This metadata must be available to storage and logs.
