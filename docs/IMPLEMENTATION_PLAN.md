# Quarry — Implementation Plan (Contract-First, Status-Tracked)

This plan reflects Quarry’s clarified architecture and introduces an explicit **contract-freezing phase**
to prevent drift between SDK, executor, runtime, ingestion policies, and Lode.
Completed items are checked off. Unchecked items are planned or unverified.
Status statements refer to released behavior where possible.

Quarry’s core principle:

> **Quarry defines the runtime boundary.  
> Ingestion Policy defines tradeoffs.  
> Lode defines structure.**

Scripts and executors remain **policy-agnostic**.

## Current Status (as of v0.3.4)
- Latest release: v0.3.4 (see CHANGELOG.md).
- Phases 0–5 complete. Phase 6 (dogfooding) in progress.
- v0.3.4 adds S3-compatible provider support (R2, MinIO).

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
- [x] Taskfile targets exist (even if stubbed)
- [x] Guardrails reviewed and accepted

---

## Phase 0.5 — Contract Definitions (NO CODE)

### Goal
Freeze **interfaces and invariants** required for parallel implementation,
without committing to internal implementations.

This phase produces **authoritative documents**, not code.

### Deliverables

#### 0.5.1 Emit Contract (`docs/contracts/CONTRACT_EMIT.md`)
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

#### 0.5.2 IPC & Streaming Contract (`docs/contracts/CONTRACT_IPC.md`)
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

#### 0.5.3 Run Identity & Lineage (`docs/contracts/CONTRACT_RUN.md`)
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

#### 0.5.4 Ingestion Policy Semantics (`docs/contracts/CONTRACT_POLICY.md`)
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

#### 0.5.5 Lode Expectations (`docs/contracts/CONTRACT_LODE.md`)
Defines Quarry’s **minimal expectations** of Lode:
- Required partition keys (source / category / day / run_id / event_type)
- Append-only semantics
- Policy-independent layout invariants
- Concrete meaning of “consistency across policies”
- Lineage/metadata surfaced; dedup elimination is downstream and out of scope

#### 0.5.6 Proxy Contract (`docs/contracts/CONTRACT_PROXY.md`)
Defines:
- Shared proxy data model (endpoint, pool, strategy, sticky scope)
- Runtime selection and rotation rules
- Executor application requirements (launch + auth)
- IPC payload fields and redaction rules
- Validation rules and observability expectations

#### 0.5.7 CLI Contract (`docs/contracts/CONTRACT_CLI.md`)
Defines:
- CLI invariants and command topology
- Read-only guarantees and non-goals
- Output format selection rules
- Request/response shapes for inspect/stats/list/debug

### Exit criteria
- [x] All five contract documents exist
- [x] SDK, executor, runtime, and policy work can proceed independently
- [x] Any ambiguity is resolved by pointing to a contract doc

---

## Phase 1 — TypeScript SDK (Script Contract)

### Goal
Define the stable authoring surface for extraction scripts.

### Deliverables
- `QuarryContext<Job>`
- `EmitAPI`
- Optional hooks
- Proxy config types + validation helpers
- Minimal SDK README with example

### Mini-milestones
- [x] Context exposes real Puppeteer objects
- [x] `emit.*` is the sole output mechanism
- [x] No ingestion or durability concepts leak into the SDK

---

## Phase 2 — Node Executor (Minimal Host)

### Goal
Execute scripts without distortion and stream events immediately.

### Deliverables
- Executor entrypoint (`stdio` mode)
- Script loader
- IPC implementation conforming to contracts
- Emit forwarding implementation
- Proxy application utilities (launch args + auth)

### Mini-milestones
- [x] Events streamed incrementally and in order
- [x] Large artifacts supported via chunking
- [x] Executor has zero knowledge of ingestion policy

---

## Phase 3 — Go Runtime Core

### Goal
Supervise execution and route events to an ingestion policy.

### Deliverables
- Executor lifecycle management
- Event ingestion loop
- Retry boundaries (retry = new run)
- Run metadata and logging
- Proxy selection + rotation state in runtime

### Mini-milestones
- [x] One job runs end-to-end
- [x] Executor crashes detected and recorded
- [x] Runtime delegates ingestion exclusively to policy layer

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
- [x] Common `IngestionPolicy` interface
- [x] Policies selectable per run
- [x] Drops and buffering visible in stats/logs

---

## Phase 5 — CLI & Configuration

### Goal
Expose policy selection without leaking complexity.

### Deliverables
- `quarry run`
- Read-only CLI surface (`inspect`, `stats`, `list`, `debug`, `version`)
- Output formatting layer (`json|table|yaml` with TTY defaults)
- Policy selection flags or config
- Clear run summaries

### Mini-milestones
- [x] Policy recorded in run metadata
- [x] Policy effects visible in output

---

## Phase 6 — Dogfooding (Two Postures)

> **Prerequisite**: v0.3.0 must be released before Phase 6 begins.
> Phase 6 is a post-release validation exercise — Quarry is used E2E
> on a real project, and feedback is captured as concrete follow-ups.

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

## v0.3.0 Roadmap — ETL Ingress Hardening

### Goal
Treat Quarry as the first ingress of an ETL pipeline and harden the operational surface
before adding event bus integrations.

### Deliverables
- Lode upgrade to v0.4.1 with compatibility notes
- CLI/Stats/Metrics hardening for run visibility and ingestion effects
- v0.3.0 release readiness checklist and acceptance criteria
- Dogfooding prerequisites validated (CLI surface functional for real-project use)

### Mini-milestones
- [x] Lode dependency updated and validated against CONTRACT_LODE.md
- [x] CLI stats output includes policy effects and run outcome clarity
- [x] Metrics coverage for run lifecycle, ingestion drops, and executor failures
- [x] Dogfooding prerequisites met (Phase 6 can begin immediately post-release)

---

## Post-v0.3.0 — Event Bus Integrations (Staggered)

Order of support:
- Temporal.io
- NATS
- Kafka
- Rabbit
- SNS/SQS

Principles:
- Integrations must not change contracts.
- Event envelope remains stable across adapters.
- CLI/Stats/Metrics hardening is a prerequisite for each new adapter.

---

## Contract Change Protocol

Any change to SDK, executor, runtime, or policy that affects contract behavior
must follow this protocol.

### Pre-merge checklist

**For SDK envelope/event type changes:**
- [ ] Update `docs/contracts/CONTRACT_EMIT.md`
- [ ] Update impacted contract docs (if any)
- [ ] Verify SDK types match contract definitions

**For IPC/streaming changes:**
- [ ] Update `docs/contracts/CONTRACT_IPC.md`
- [ ] Verify executor and runtime implementations align

**For run identity/lineage changes:**
- [ ] Update `docs/contracts/CONTRACT_RUN.md`

### PR requirements

PRs that modify contract behavior must:
1. Reference the specific contract section changed.
2. Include a **compatibility note** (breaking vs additive).
3. Update `contract_version` if the change is breaking.
