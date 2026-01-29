# Quarry — Implementation Plan (Updated with Phase 0.5)

This plan reflects Quarry’s clarified architecture and introduces an explicit **contract-freezing phase**
to prevent drift between SDK, executor, runtime, ingestion policies, and Lode.

Quarry’s core principle:

> **Quarry defines the runtime boundary.  
> Ingestion Policy defines tradeoffs.  
> Lode defines structure.**

Scripts and executors remain **policy-agnostic**.

---

## Conceptual Stack

```
User Script (Puppeteer, imperative)
        ↓
emit.*  (stable contract)
        ↓
Quarry Runtime (execution + observability)
        ↓
Ingestion Policy (configurable behavior)
        ↓
Lode (partitioned persistence substrate)
```

---

## Phase 0 — Foundations & Guardrails

### Goal
Establish development discipline before any architectural commitments harden.

### Deliverables
- Repo skeleton with internal modules:
  - `runtime/`
  - `executor-node/`
  - `sdk/`
  - `docs/`
  - `examples/`
- `AGENTS.md` defining style and collaboration guardrails
- Taskfile scaffolding for lint / test / build

### Mini-milestones
- [x] Repo structure committed
- [ ] Taskfile targets exist (even if stubbed)
- [ ] Guardrails reviewed and accepted

---

## Phase 0.5 — Contract Definitions (NO CODE)

### Goal
Freeze **interfaces and invariants** required for parallel implementation,
without committing to internal implementations.

This phase produces **authoritative documents**, not code.

### Deliverables

#### 0.5.1 Emit Contract (`docs/CONTRACT_EMIT.md`)
Defines:
- Event envelope fields (e.g. `event_id`, `run_id`, `seq`, `type`, `payload`, `ts`)
- Initial event types:
  - `item`
  - `artifact`
  - `checkpoint`
  - `enqueue` (optional advisory; not required or guaranteed)
  - `rotate_proxy` (optional advisory; not required or guaranteed)
  - `log`
  - `run_error`
  - `run_complete`
- Ordering guarantees:
  - Total order **per run**
  - No reordering across event types
- Versioning rules:
  - Contract version in envelope
  - Backward-compatible additive changes only
- Error semantics:
  - Script abort vs executor crash vs policy failure

#### 0.5.2 IPC & Streaming Contract (`docs/CONTRACT_IPC.md`)
Defines:
- Framing format (length prefix + payload)
- Payload encoding (single choice, e.g. msgpack)
- Maximum frame size
- Artifact chunking rules:
  - chunk size
  - ordering
  - reassembly expectations
- Backpressure semantics:
  - blocking writes (emit blocks on backpressure)
  - no executor-side buffering
  - dropping is policy-layer only
- Failure behavior on partial frames or pipe closure

#### 0.5.3 Run Identity & Lineage (`docs/CONTRACT_RUN.md`)
Defines:
- Canonical `run_id`
- Relationship: job → run → retry run
- Correlation fields:
  - `job_id`
  - `parent_run_id`
  - `attempt`
- Idempotency expectations:
  - runs are append-only
  - deduplication is downstream responsibility

#### 0.5.4 Ingestion Policy Semantics (`docs/CONTRACT_POLICY.md`)
Defines:
- What constitutes a “drop”
- Which event types may be dropped
- What “buffered” means (bounded memory, flush guarantees)
- Required observability:
  - counters
  - logs
  - events
- Invariants:
  - no silent loss
  - policy does not alter event shapes

#### 0.5.5 Lode Expectations (`docs/CONTRACT_LODE.md`)
Defines Quarry’s **minimal expectations** of Lode:
- Required partition keys (e.g. dataset / run_id / event_type)
- Append-only semantics
- Policy-independent layout invariants
- Concrete meaning of “consistency across policies”
- Lineage/metadata surfaced; dedup elimination is downstream and out of scope

### Exit criteria
- All five contract documents exist
- SDK, executor, runtime, and policy work can proceed independently
- Any ambiguity is resolved by pointing to a contract doc

---

## Phase 1 — TypeScript SDK (Script Contract)

### Goal
Define the stable authoring surface for extraction scripts.

### Deliverables
- `QuarryContext<Job>`
- `EmitAPI`
- Optional hooks
- Minimal SDK README with example

### Mini-milestones
- [ ] Context exposes real Puppeteer objects
- [ ] `emit.*` is the sole output mechanism
- [ ] No ingestion or durability concepts leak into the SDK

---

## Phase 2 — Node Executor (Minimal Host)

### Goal
Execute scripts without distortion and stream events immediately.

### Deliverables
- Executor entrypoint (`stdio` mode)
- Script loader
- IPC implementation conforming to contracts
- Emit forwarding implementation

### Mini-milestones
- [ ] Events streamed incrementally and in order
- [ ] Large artifacts supported via chunking
- [ ] Executor has zero knowledge of ingestion policy

---

## Phase 3 — Go Runtime Core

### Goal
Supervise execution and route events to an ingestion policy.

### Deliverables
- Executor lifecycle management
- Event ingestion loop
- Retry boundaries (retry = new run)
- Run metadata and logging

### Mini-milestones
- [ ] One job runs end-to-end
- [ ] Executor crashes detected and recorded
- [ ] Runtime delegates ingestion exclusively to policy layer

---

## Phase 4 — Ingestion Policy Layer

### Goal
Make tradeoffs explicit and swappable.

### Required policies

#### Strict Policy
- Synchronous writes
- Immediate backpressure
- No drops

#### Buffered Policy
- Bounded in-memory buffering
- Batched writes
- Explicit, observable drops allowed

### Mini-milestones
- [ ] Common `IngestionPolicy` interface
- [ ] Policies selectable per run
- [ ] Drops and buffering visible in stats/logs

---

## Phase 5 — CLI & Configuration

### Goal
Expose policy selection without leaking complexity.

### Deliverables
- `quarry run`
- Policy selection flags or config
- Clear run summaries

### Mini-milestones
- [ ] Policy recorded in run metadata
- [ ] Policy effects visible in output

---

## Phase 6 — Dogfooding (Two Postures)

### Goal
Validate Quarry across different ETL philosophies.

### Mandatory runs
1. Precision-first ETL (strict policy)
2. Volume-first ETL (buffered/loss-tolerant policy)

### Mini-milestones
- [ ] Same script structure used in both
- [ ] Same emission contract used in both
- [ ] Only ingestion policy differs

---

## Exit Criteria

Quarry is ready to expand when:
- Contracts prevent cross-component drift
- Scripts are policy-agnostic
- Loss (if any) is intentional and observable
- Lode partitions are consistent across policies
- No escape hatches were required during dogfooding

---

## Contract Change Protocol

Any change to SDK, executor, runtime, or policy that affects contract behavior
must follow this protocol.

### Pre-merge checklist

**For SDK envelope/event type changes:**
- [ ] Update `docs/CONTRACT_EMIT.md`
- [ ] Update impacted contract docs (if any)
- [ ] Verify SDK types match contract definitions

**For IPC/streaming changes:**
- [ ] Update `docs/CONTRACT_IPC.md`
- [ ] Verify executor and runtime implementations align

**For run identity/lineage changes:**
- [ ] Update `docs/CONTRACT_RUN.md`

### PR requirements

PRs that modify contract behavior must:
1. Reference the specific contract section changed.
2. Include a **compatibility note** (breaking vs additive).
3. Update `contract_version` if the change is breaking.
