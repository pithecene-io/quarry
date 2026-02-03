# Lode Expectations (User View)

This document explains the storage expectations Quarry places on Lode.
The authoritative contract is `docs/contracts/CONTRACT_LODE.md`.

---

## What Lode Guarantees

- Append-only storage for events.
- Stable partitioning by source, category, day, run, and event type.
- Consistent layout across different ingestion policies.

---

## Why This Matters

Users can query data without knowing which policy produced it, and runs
remain immutable once written.

## Partitioning Semantics (Informational)

- Lode dataset ID is fixed to `quarry` (global container).
- `source` is the origin system/provider (required).
- `category` is the logical data type within a source (required).
  For one-to-one sources, use `category=default`.
- `day` is derived from the **run start time** (not individual event timestamps).
  Events may span dates but remain in the run's `day` partition.
- Preferred Hive ordering: `source / category / day / run_id / event_type`.

---

## Record Types

Every stored record includes a `record_kind` field to identify its type:

| Kind             | Description                                    |
|------------------|------------------------------------------------|
| `event`          | Standard event (item, log, checkpoint, etc.)   |
| `artifact_event` | Artifact commit record (marks artifact complete)|
| `artifact_chunk` | Binary chunk of an artifact                    |

This allows queries to filter by record type without parsing payloads.

---

## Artifact Storage

Artifacts (binary files) are stored as a sequence of chunks followed by a
commit record.

**Flow:**
1. Chunks are emitted with sequential `seq` numbers starting at 1
2. The final chunk has `is_last: true`
3. An artifact commit event is emitted with total `size_bytes`
4. Chunks are written to storage first, then the commit record

The commit record is the durability boundary. If a run fails before the
commit is written, chunks are considered orphans.

---

## Checksum (Optional)

Chunk records may include an MD5 checksum for integrity verification.

- `checksum`: hex-encoded MD5 digest of chunk data
- `checksum_algo`: always `"md5"` when checksum is present

Checksum generation is optional and controlled by runtime configuration.
Validation is a downstream consumer responsibility.
