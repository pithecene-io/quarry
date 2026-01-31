# Quarry Roadmap (User-Facing)

This document summarizes Quarry’s implementation phases at a high level.
It is informational; the authoritative plan lives in `docs/IMPLEMENTATION_PLAN.md`.

---

## Phases (High Level)

- **Contracts**: freeze interfaces and invariants so components can evolve independently.
- **SDK**: provide the stable script authoring API.
- **Executor**: run scripts and stream events faithfully.
- **Runtime**: supervise runs, retries, and ingestion policy.
- **Policies**: implement strict and buffered ingestion modes.
- **CLI**: expose runtime functionality and inspection surfaces.

---

## What This Means For Users

- Contracts stabilize external behavior early.
- SDK and CLI mature before policy and storage nuances.
- Feature work that changes contracts is deliberate and rare.
