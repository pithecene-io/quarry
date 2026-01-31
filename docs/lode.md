# Lode Expectations (User View)

This document explains the storage expectations Quarry places on Lode.
The authoritative contract is `docs/contracts/CONTRACT_LODE.md`.

---

## What Lode Guarantees

- Append-only storage for events.
- Stable partitioning by dataset, run, and event type.
- Consistent layout across different ingestion policies.

---

## Why This Matters

Users can query data without knowing which policy produced it, and runs
remain immutable once written.
