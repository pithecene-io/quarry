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

## Dataset Identity

Quarry writes to a fixed Lode dataset ID: `quarry`.

This dataset ID is a global container and is **not** the same as Quarry's
logical `category` partition key.

---

## Required Partition Keys

Lode must support partitioning by:
- `source` (origin system/provider; required)
- `category` (logical data type within a source; required; default `category=default`)
- `day` (derived from run start time; see below)
- `run_id`
- `event_type`

Additional keys are allowed, but the above must be present.

### Partition Key Semantics

- `category` is required. If no meaningful category exists for a source,
  use `category=default`.
- `day` is derived from the **run start time**, not individual event timestamps.
  Events may span dates, but must remain in the run's `day` partition.

### Recommended Layout Ordering

For Hive-style layouts, the preferred order is:

`source / category / day / run_id / event_type`

---

## Append-Only Semantics

- Events are **append-only**.
- No updates or deletes are required or expected.
- Event order within a run must be preserved.

---

## Record Types

All stored records MUST include a `record_kind` discriminator field.

| `record_kind`    | Description                          |
|------------------|--------------------------------------|
| `event`          | Standard event envelope              |
| `artifact_event` | Artifact commit (manifest) event     |
| `artifact_chunk` | Artifact binary chunk                |
| `metrics`        | Per-run metrics snapshot             |

The `record_kind` field enables downstream consumers to distinguish record
types without inspecting `event_type` or payload structure.

---

## Artifact Chunk Storage

Artifact binary data is stored as chunk records under `event_type=artifact`.

### Chunk Record Schema

| Field          | Type     | Required | Description                              |
|----------------|----------|----------|------------------------------------------|
| `record_kind`  | string   | yes      | `"artifact_chunk"`                       |
| `artifact_id`  | string   | yes      | Artifact identifier                      |
| `seq`          | int64    | yes      | Sequence number (starts at 1)            |
| `is_last`      | bool     | yes      | True if final chunk                      |
| `offset`       | int64    | yes      | Byte offset within artifact              |
| `length`       | int64    | yes      | Chunk data length in bytes               |
| `data`         | bytes    | yes      | Raw chunk data (base64 in JSON)          |
| `checksum`     | string   | no       | Chunk checksum (see Checksum section)    |
| `checksum_algo`| string   | no       | Checksum algorithm (must be `md5`)       |

### Commit Record Schema

The artifact commit event marks the artifact as complete.

| Field          | Type     | Required | Description                              |
|----------------|----------|----------|------------------------------------------|
| `record_kind`  | string   | yes      | `"artifact_event"`                       |
| `artifact_id`  | string   | yes      | Artifact identifier                      |
| `name`         | string   | yes      | Human-readable artifact name             |
| `content_type` | string   | yes      | MIME content type                        |
| `size_bytes`   | int64    | yes      | Total artifact size in bytes             |

### Ordering Invariant

> Chunk records MUST be written before the corresponding commit record.

The commit record is the commit boundary. Chunks written without a subsequent
commit are orphans and may be garbage collected.

---

## Checksum

The `checksum` field on chunk records is **optional**.

If present:
- `checksum_algo` MUST be set to `"md5"`.
- `checksum` contains the hex-encoded MD5 digest of the chunk data.

Checksum validation is a downstream consumer responsibility.

---

## Metrics Record Storage

A metrics snapshot is written at run completion under `event_type=metrics`.
Write is best-effort — failure produces a warning but does not change run outcome.

### Metrics Record Schema

| Field                     | Type              | Required | Description                              |
|---------------------------|-------------------|----------|------------------------------------------|
| `record_kind`             | string            | yes      | `"metrics"`                              |
| `event_type`              | string            | yes      | `"metrics"` (Hive partition key)         |
| `ts`                      | string (RFC3339)  | yes      | Completion timestamp                     |
| `runs_started`            | int64             | yes      | Run lifecycle counter                    |
| `runs_completed`          | int64             | yes      | Run lifecycle counter                    |
| `runs_failed`             | int64             | yes      | Run lifecycle counter                    |
| `runs_crashed`            | int64             | yes      | Run lifecycle counter                    |
| `events_received`         | int64             | yes      | Ingestion counter                        |
| `events_persisted`        | int64             | yes      | Ingestion counter                        |
| `events_dropped`          | int64             | yes      | Ingestion counter                        |
| `dropped_by_type`         | map[string]int64  | no       | Per-type drop breakdown                  |
| `executor_launch_success` | int64             | yes      | Executor counter                         |
| `executor_launch_failure` | int64             | yes      | Executor counter                         |
| `executor_crash`          | int64             | yes      | Executor counter                         |
| `ipc_decode_errors`       | int64             | yes      | Executor counter                         |
| `lode_write_success`      | int64             | yes      | Storage counter                          |
| `lode_write_failure`      | int64             | yes      | Storage counter                          |
| `lode_write_retry`        | int64             | yes      | Storage counter (reserved)               |
| `policy`                  | string            | yes      | Dimension: policy name                   |
| `executor`                | string            | yes      | Dimension: executor identity             |
| `storage_backend`         | string            | yes      | Dimension: storage backend               |
| `run_id`                  | string            | yes      | Dimension: run identifier                |
| `job_id`                  | string            | no       | Dimension: job identifier                |
| `source`                  | string            | yes      | Partition key                            |
| `category`                | string            | yes      | Partition key                            |
| `day`                     | string            | yes      | Partition key (YYYY-MM-DD)               |

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
