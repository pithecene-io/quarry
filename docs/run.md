# Runs, Jobs, and Attempts

This document explains Quarry’s run lifecycle at a user level.
The authoritative contract is `docs/contracts/CONTRACT_RUN.md`.

---

## Definitions

- **Job**: logical unit of work.
- **Run**: a single execution attempt of a job.
- **Attempt**: a counter for retries of the same job.

---

## Lineage

Retries create **new runs** with a parent run reference. The original job
ID stays the same across retries.

---

## Outcomes

Runs end in one of a few outcomes:
- success (normal completion)
- script error (emitted `run_error`)
- executor crash
- policy failure

Each outcome is observable in runtime metadata.
