# Quarry IPC & Streaming Contract

This document freezes the IPC framing and streaming behavior between
executor and runtime. It defines framing, encoding, chunking, backpressure,
and failure behavior.

This is a contract document. Implementations must conform.

---

## Scope

- Applies to the executor-runtime pipe or stream.
- Defines wire framing for event envelopes and artifact data.
- Defines backpressure semantics.

Non-goals:
- Does not define event envelope structure (see CONTRACT_EMIT.md).
- Does not define ingestion policy behavior.

---

## Framing Format

- **Frame structure**: length prefix + payload
- **Length prefix**: 4 bytes, unsigned big-endian integer
- **Payload**: encoded bytes as specified below

The stream is a sequence of frames. Each frame is either:
- an **event frame** (encoded event envelope), or
- an **artifact chunk frame** (binary chunk)

---

## Payload Encoding

Single encoding choice for event envelopes:
- `msgpack`

Rationale: compact, stable, language-agnostic.

Artifact chunk frames contain raw bytes (no msgpack layer).

---

## Maximum Frame Size

- Maximum frame size: **16 MiB**
- Frames exceeding this limit are invalid and must be rejected.

Artifacts larger than 16 MiB must be chunked (see below).

---

## Artifact Chunking

Artifacts are emitted as:
1) An `artifact` event (metadata envelope).
2) One or more **artifact chunk frames**.

Chunk rules:
- **Chunk size**: up to 8 MiB of raw bytes.
- **Ordering**: strictly in order after the `artifact` event.
- **Reassembly**: consumer reassembles based on:
  - `artifact_id`
  - chunk sequence
  - total size bytes from metadata

Chunk frame payload layout (msgpack-encoded envelope):
- `type` = `artifact_chunk`
- `artifact_id` (string)
- `seq` (integer, starting at 1)
- `data` (bytes)

The `artifact_chunk` envelope is not a normal emit event and does not use
the standard event envelope. It is a stream-level construct.

---

## Backpressure Semantics

- **Emit calls must block on backpressure.**
- The executor **must not** buffer unboundedly.
- The executor **must not** drop frames.
- Dropping is a policy-layer decision only.

If the runtime applies backpressure, the executor stalls, and the script blocks.
This makes loss explicit and avoids hidden queues.

---

## Failure Behavior

### Partial Frame
- If a frame is truncated or malformed, the runtime must treat it as a fatal
  stream error and terminate the run.

### Pipe Closure
- If the pipe closes cleanly after a valid `run_complete`, the run is complete.
- If the pipe closes without `run_complete`, the runtime records a crash or
  premature termination (see CONTRACT_EMIT.md error semantics).

---

## Invariants

- Frame boundaries are authoritative; no frame may contain multiple envelopes.
- The runtime must not attempt to resynchronize after invalid framing.
- Event envelopes must be decoded before ingestion policy handling.
