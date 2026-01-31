# Streaming & Artifacts

This document explains how events and artifacts flow between the executor
and runtime. It is user-facing; the authoritative contract is
`docs/contracts/CONTRACT_IPC.md`.

---

## Event Streaming

Executors stream events immediately as they occur. The runtime applies
ingestion policy on the receiving side.

---

## Artifacts and Chunking

Large artifacts are chunked into multiple frames. The artifact **event**
acts as the commit record that makes the artifact visible.

Key expectations:
- Chunks may arrive before the artifact event.
- The artifact event is the authoritative commit.
- Orphaned chunks may be garbage-collected if no event arrives.

---

## Backpressure

When the runtime applies backpressure, the executor stalls and the script
blocks. This avoids hidden buffers and makes loss explicit.
