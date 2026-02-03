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
