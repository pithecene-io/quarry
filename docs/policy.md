# Ingestion Policies

This document describes ingestion policy behavior at a user level.
The authoritative contract is `docs/contracts/CONTRACT_POLICY.md`.

---

## What Policies Do

Policies govern how events are buffered, persisted, and (if allowed) dropped.
They do not change event shapes or ordering.

---

## Drop Rules (User View)

Only advisory or diagnostic events may be dropped:
- `log`
- `enqueue`
- `rotate_proxy`

All core output events (`item`, `artifact`, `checkpoint`, `run_error`,
`run_complete`) must not be dropped.

---

## Strict vs Buffered

- **Strict**: synchronous writes, no drops.
- **Buffered**: bounded buffers, batched writes, explicit drops allowed.

Both modes preserve per-run ordering and emit observability about drops.
