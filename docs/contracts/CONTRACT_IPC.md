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

All frames use msgpack encoding:
- `msgpack`

Rationale: compact, stable, language-agnostic.

Event frames contain the msgpack-encoded EventEnvelope directly.
Artifact chunk frames contain a msgpack-encoded chunk envelope (see Artifact Chunking).

Decoding discrimination: artifact chunk frames have `type: 'artifact_chunk'`;
event envelopes have event types like `'item'`, `'log'`, `'run_complete'`, etc.

---

## Run Control Payloads (Out of Band)

The runtime and executor may exchange **run control payloads** outside the
event stream (e.g., initial run request and final run result).
These payloads must:
- Use the ProxyEndpoint schema defined in `CONTRACT_PROXY.md` when a proxy is supplied.
- Never include proxy passwords in any result/summary payload.

### Run Request (Runtime → Executor)
If present, the run request includes an optional `proxy` field:
- `proxy` (optional): `ProxyEndpoint`

### Run Result (Executor → Runtime)
If present, the run result may include `proxy_used` metadata:
- `proxy_used` (optional): `ProxyEndpoint` **without** `password`

---

## Run Result Control Frame

The **run_result** frame is a control frame emitted by the executor after
determining run outcome. It provides structured outcome information and
optional proxy usage metadata.

### Frame Structure

The run_result frame is msgpack-encoded with the following fields:

```
RunResultFrame:
  type: "run_result"          # literal string discriminator
  outcome:
    status: string            # "completed" | "error" | "crash"
    message: string | null    # optional error/status message
    error_type: string | null # optional error type (e.g., "TypeError")
    stack: string | null      # optional stack trace
  proxy_used: ProxyEndpointRedacted | null  # optional, no password
```

Where `ProxyEndpointRedacted` is:
```
ProxyEndpointRedacted:
  protocol: string    # "http" | "https" | "socks5"
  host: string
  port: integer
  username: string | null
```

### Control Frame Semantics

- **Not an event**: The run_result frame does NOT affect event sequence ordering.
  It is a control frame, not counted in `seq`.
- **Single emission**: The executor emits exactly one run_result frame per run,
  after the terminal event.
- **Duplicate handling**: If multiple run_result frames are received, only the
  first is processed; subsequent frames are ignored.
- **Exit code authority**: Exit codes are authoritative for outcome classification.
  If exit code conflicts with run_result status, exit code wins. The run_result
  provides supplementary context (message, error_type, stack) but does not
  override exit code determination.

### Outcome Status Mapping

| run_result status | Exit code | Final outcome    |
|-------------------|-----------|------------------|
| completed         | 0         | success          |
| completed         | 1         | script_error     |
| completed         | 2         | executor_crash   |
| error             | 0         | success*         |
| error             | 1         | script_error     |
| error             | 2         | executor_crash   |
| crash             | any       | executor_crash   |

*Exit code 0 with run_result "error" logs a warning but trusts exit code.

---

## Maximum Frame Size

- Maximum frame size: **16 MiB**
- Frames exceeding this limit are invalid and must be rejected.

Artifacts larger than 16 MiB must be chunked (see below).

---

## Artifact Chunking

Artifacts are transmitted as:
1) One or more **artifact chunk frames** (bytes).
2) An `artifact` event (metadata envelope) — the **commit record**.

Chunk rules:
- **Chunk size**: up to 8 MiB of raw bytes.
- **Ordering**: strictly in order; chunk `seq` starts at 1.
- **Completion**: the final chunk has `is_last: true`.
- **Reassembly**: consumer reassembles using `artifact_id` and `seq`.
  Reassembly does not require the artifact event; `is_last` signals
  when all bytes have arrived.

Chunk frame payload layout (msgpack-encoded envelope):
- `type` = `artifact_chunk`
- `artifact_id` (string)
- `seq` (integer, starting at 1)
- `is_last` (boolean)
- `data` (bytes)

The `artifact_chunk` envelope is not a normal emit event and does not use
the standard event envelope. It is a stream-level construct.

---

## Artifact Commit Semantics

The **artifact event** is the authoritative **commit record** for an artifact.

### Ordering Guarantees

- Artifact bytes (chunk frames) **MAY** precede the artifact event.
- The runtime **MUST** accept artifact chunk frames keyed by `artifact_id`
  even if the corresponding artifact event has not yet been observed.
- The runtime **MUST NOT** treat an artifact as "existent" until the
  artifact event is received.

### Orphaned Blobs

- If artifact bytes arrive but no artifact event follows (e.g., script crash),
  the bytes are **orphaned** and eligible for garbage collection.
- GC strategy is implementation-defined (e.g., blobs older than N hours with
  no manifest reference).

### Size Constraints

- Maximum artifact size: implementation-defined (recommended: 1 GiB).
- Maximum chunk size: 8 MiB (per Artifact Chunking above).
- Artifacts exceeding the maximum size **MUST** be rejected by the runtime.

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
