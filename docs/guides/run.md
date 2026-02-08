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

---

## Child Runs (Fan-Out)

When `--depth > 0` is set, scripts can trigger **child runs** via
`emit.enqueue()`. Child runs differ from retry runs:

| | Retry Run | Child Run |
|-|-----------|-----------|
| Trigger | Failure of previous run | `emit.enqueue()` from parent |
| Script | Same as parent | Specified by `target` field |
| `attempt` | Incremented | Always 1 |
| `job_id` | Same as parent | New (derived from parent) |

Child runs are tracked internally by the fan-out operator. The root run's
exit code determines the overall outcome; child run results appear in the
fan-out summary.
