# Emitting Data

This document explains how scripts emit data and metadata in Quarry.
It is user-facing; the authoritative contract is `docs/contracts/CONTRACT_EMIT.md`.

---

## The Emit Surface

Scripts emit all output through `emit.*`. There are no side channels.

Event types include:
- `item`: structured records
- `artifact`: binary or large payloads
- `checkpoint`: script-defined progress markers
- `log`: structured logs
- `run_error`: fatal errors
- `run_complete`: normal completion

Some advisory types exist (`enqueue`, `rotate_proxy`), but they are optional.

---

## When To Emit What

- **Records**: use `item` for the primary outputs of a run.
- **Large content**: use `artifact` for files, blobs, or large text.
- **Progress**: use `checkpoint` to mark milestones.
- **Errors**: emit `run_error` and then terminate.
- **Completion**: emit `run_complete` once the script finishes.

---

## Ordering Guarantees

Events are ordered within a run. The runtime expects strictly increasing
sequence numbers and does not reorder types.

---

## Versioning Notes

The emit envelope includes a contract version string. Contract changes are
additive only; breaking changes require explicit version bumps.
