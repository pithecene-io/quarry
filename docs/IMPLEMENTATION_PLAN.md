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

## Current Status (as of v0.5.1)
- Latest release: v0.5.1 (see CHANGELOG.md).
- Phases 0–5 complete. Phase 6 (dogfooding) in progress.
- v0.5.1 fixed published npm packages missing `dist/` directory (#110).
- v0.5.0 adds webhook and Redis pub/sub event-bus adapters for downstream notifications.
- v0.4.1 added `--config` for YAML project-level defaults and config package hardening.
- v0.4.0 added `ctx.storage.put()` for sidecar file uploads via Lode Store.
- Next priority: v0.6.0 crawl mode (`quarry crawl`) — see roadmap below.

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

## v0.4.0 Roadmap — Storage Mechanics

### Goal
Expose sidecar file upload capabilities to scripts via `ctx.storage.put()`,
enabling direct writes to addressable storage paths outside the event pipeline.

### Deliverables
- `StorageAPI` on `QuarryContext` with `put()` method
- `file_write` IPC frame type for executor→runtime file transfer
- Content type persistence via companion `.meta.json`
- Terminal guard enforcement on `storage.put()`
- Contract updates (CONTRACT_IPC.md, CONTRACT_EMIT.md)

### Mini-milestones
- [x] SDK `StorageAPI` type and `storage.put()` implementation
- [x] `file_write` IPC frame in executor and runtime
- [x] Lode file writer for sidecar persistence
- [x] Terminal guard prevents writes after run completion
- [x] Contract and guide updates
- [x] Tests: SDK, executor IPC, runtime ingestion

---

## v0.5.0 Roadmap — Event Bus Adapters

### Goal
Provide runtime-integrated downstream notification so consumers do not need
to poll Lode or wire external plumbing.

### Deliverables
- `Adapter` interface and `RunCompletedEvent` type (`quarry/adapter/`)
- Webhook adapter: HTTP POST with retries, custom headers, timeout (`quarry/adapter/webhook/`)
- Redis pub/sub adapter: PUBLISH with retries, configurable channel (`quarry/adapter/redis/`)
- CLI flags: `--adapter`, `--adapter-url`, `--adapter-header`, `--adapter-channel`, `--adapter-timeout`, `--adapter-retries`
- Hook in `runAction()` after metrics persist (best-effort, does not fail run)
- Contract and guide updates

### Mini-milestones
- [x] Adapter interface and event type (`quarry/adapter/adapter.go`)
- [x] Webhook adapter implementation (`quarry/adapter/webhook/webhook.go`)
- [x] Webhook tests with httptest (`quarry/adapter/webhook/webhook_test.go`)
- [x] CLI flags and hook in `quarry/cli/cmd/run.go`
- [x] CONTRACT_INTEGRATION.md updated with runtime adapter reference
- [x] CONTRACT_CLI.md updated with adapter flags
- [x] Integration guide updated with webhook example
- [x] CLI_PARITY.json updated

### Redis Pub/Sub Adapter (v0.5.0)
- [x] Redis adapter implementation (`quarry/adapter/redis/redis.go`)
- [x] Redis adapter tests with miniredis (`quarry/adapter/redis/redis_test.go`)
- [x] `--adapter-channel` CLI flag
- [x] Config YAML `channel` field in adapter stanza
- [x] CLI wiring in `run.go` (import, flag, parse, build)
- [x] CLI_PARITY.json updated
- [x] CONTRACT_INTEGRATION.md and CONTRACT_CLI.md updated

### Future adapters (separate PRs)
- NATS
- SNS

### Future: At-Least-Once Delivery (Outbox Pattern)

v0.5.0 delivery is best-effort with retries. If all retries exhaust, the
notification is lost. For workloads that require at-least-once guarantees,
a Lode-backed outbox pattern can be added without breaking the existing
contract (strengthening from best-effort to at-least-once is additive).

#### Design sketch
1. Write a `notification_pending` record to Lode **before** attempting
   publish (durable intent).
2. Attempt publish with retries (existing webhook logic).
3. On success, mark the record `delivered`.
4. Undelivered records are retryable via a CLI command
   (`quarry adapter drain`) or detected by the next run.

Outbox records would live at:
```
datasets/<dataset>/partitions/source=<s>/category=<c>/day=<d>/run_id=<r>/event_type=adapter_outbox/
```

#### Relationship to external orchestrators

The outbox pattern applies only to standalone/CLI usage where no external
orchestrator provides delivery guarantees. When Quarry runs inside an
orchestrator (Temporal, Airflow, etc.), the outbox is redundant — the
orchestrator owns durable execution and downstream notification becomes
a separate step with the orchestrator's built-in retry semantics.

Standalone CLI usage remains a first-class paradigm. The outbox pattern
is relevant for users running Quarry directly (cron, CI, shell scripts)
without an external orchestrator.

See `docs/guides/temporal.md` for the full Temporal integration design.

#### Open questions
- Retry ownership: CLI command, background process, or next-run piggyback?
- TTL for stale outbox entries (when to give up permanently).
- Whether `quarry adapter drain` should be a new CLI command or a
  subcommand of `quarry run`.

This is deferred until best-effort proves insufficient in standalone
production usage.

---

## v0.6.0 Roadmap — Crawl Mode

### Goal

Enable Quarry to follow its own `enqueue` signals, turning a single CLI
invocation into a bounded crawl that discovers and processes work without
external orchestration.

### Motivation

Crawling is the most common Quarry workload shape that the current CLI
cannot serve natively. A listing page emits `enqueue` events for detail
pages, but the runtime drops them — forcing users to build external
orchestrators that read items from storage, loop, and spawn `quarry run`
per target. Crawl mode closes this loop inside the runtime.

This fills the gap between `quarry run` (single target, zero infrastructure)
and external orchestrators like Temporal (complex DAGs, heavy infrastructure):

```
Complexity of work graph
──────────────────────────────────────────────────────
Simple               Discovered              Complex DAG
(one target)         (fan-out)               (multi-step, conditional)

quarry run           quarry crawl            External orchestrator
```

### Semantics

- `quarry crawl` is a new CLI command (does not change `quarry run`)
- `enqueue` events remain advisory in `quarry run` — crawl mode is opt-in
- Each `enqueue` event spawns a child run:
  - `target` → `--source` (script to execute)
  - `params` → `--job` (job payload)
- `parent_run_id` links child runs to their parent
- All runs in a crawl share the same Lode dataset
- The crawl exits when the run tree is exhausted or limits are reached

### Bounds and Controls

- `--max-depth N` — maximum generations from the root run
- `--concurrency N` — maximum in-flight child runs
- `--max-runs N` — hard cap on total runs in the crawl
- Deduplication by `(target, params)` hash — same work is not re-queued

### Outcome Semantics

A crawl's exit reflects aggregate outcome:
- Exit 0 if all runs completed successfully
- Non-zero if any run failed (with summary of failures)
- Partial success is the expected case for large crawls
- `quarry inspect` extended to show crawl tree with per-run outcomes

### Contract Impact

- No changes to CONTRACT_EMIT.md — `enqueue` stays advisory
- CONTRACT_RUN.md — additive: `parent_run_id` threading for crawl lineage
- CONTRACT_CLI.md — additive: `quarry crawl` command and flags

### Deliverables

- `quarry crawl` CLI command with depth/concurrency/max-runs flags
- Crawl scheduler in runtime: enqueue consumer, dedup, child run spawning
- Lineage tracking via `parent_run_id` across the run tree
- Crawl summary output (tree view, per-run outcomes, aggregate stats)
- CLI `inspect` support for crawl trees

### Mini-milestones

- [ ] Contract updates (CONTRACT_RUN.md lineage, CONTRACT_CLI.md command)
- [ ] Crawl scheduler: enqueue event consumer with dedup and depth tracking
- [ ] Child run spawning with concurrency limiter
- [ ] `parent_run_id` threading through run metadata
- [ ] `quarry crawl` CLI wiring (command, flags, config)
- [ ] Crawl summary output and exit code semantics
- [ ] `quarry inspect` crawl tree view
- [ ] Tests: depth bounds, concurrency limits, dedup, partial failure

### Scope boundary

Crawl mode handles bounded, discovered work graphs. It does **not** handle:
- Priority queuing (depth-first vs breadth-first is implementation-chosen)
- Cross-crawl dedup (don't re-scrape URLs seen in previous crawls)
- Per-URL rate limiting beyond proxy rotation
- Resume of interrupted crawls
- Conditional branching (if X then enqueue Y)

These belong in external orchestrators. If crawl mode starts accumulating
these features, it has exceeded its scope.

### Design exploration

See `docs/ingress-models.md` for the full analysis of ingress models
and how crawl mode (Model B) relates to other options.

---

## v0.7.0 Roadmap — Advanced Proxy Rotation

### Goal
Harden proxy selection for production workloads that require recency-aware
rotation to reduce endpoint reuse and improve scraping reliability.

### Phase 1: In-Memory Recency Window

Add an opt-in `recency_window` pool option that maintains a ring buffer of
recently-used endpoint indices and excludes them from random selection.

#### Deliverables
- `RecencyWindow *int` field on `ProxyPool` (Go types + SDK types)
- Ring buffer in selector `poolState`, recency-aware `selectRandom`
- LRU fallback when window >= endpoint count (never blocks)
- Peek/commit semantics (peek does not advance ring)
- Validation: hard-reject if <= 0; soft-warn if set on non-random strategy
- Contract, guide, and SDK validation updates

#### Implementation scope
- `quarry/types/proxy.go` — field, validation, warnings
- `quarry/proxy/selector.go` — ring buffer, modified `selectRandom`, stats
- `sdk/src/types/proxy.ts` — `recencyWindow` field
- `sdk/src/proxy.ts` — validation in `validateProxyPool`
- `quarry/cli/reader/types.go` — `RecencyWindow`/`RecencyFill` in `ProxyRuntime`

#### Mini-milestones
- [ ] Contract updated (`CONTRACT_PROXY.md` — recency semantics)
- [ ] Guide updated (`docs/guides/proxy.md` — user-facing docs)
- [ ] Go types and validation (`quarry/types/proxy.go`)
- [ ] Selector ring buffer and recency-aware random (`quarry/proxy/selector.go`)
- [ ] SDK types and validation (`sdk/src/types/proxy.ts`, `sdk/src/proxy.ts`)
- [ ] Tests: type validation, selector avoidance, LRU fallback, peek semantics

### Phase 2: Pluggable Recency Backend (Future)

Replace in-memory ring buffer with a pluggable backend interface to enable
cross-run, cross-process proxy coordination.

- Interface: `RecencyStore` with `Mark(idx)` / `Exclude() []int` / `LRU() int`
- In-memory implementation (default, matches Phase 1)
- Redis implementation (atomic choose+mark via Lua script)
- Enables concurrent workers sharing proxy state

Phase 2 is deferred until Phase 1 is validated in production.

---

## Additional Event Bus Adapters (Staggered)

Order of support:
- ~~Redis Pub/Sub~~ (shipped in v0.5.0)
- NATS
- SNS/SQS

Principles:
- Integrations must not change contracts.
- Event envelope remains stable across adapters.
- CLI/Stats/Metrics hardening is a prerequisite for each new adapter.

---

## Module Split — Multi-Module Restructure (Deferred)

> **Status: Deferred.** The module split was originally motivated by the
> Temporal integration (see below). With Temporal deprioritized, the split
> is not urgent. It remains a clean-up goal for dependency hygiene but is
> no longer on the critical path.

### Goal

Split the `quarry/` Go module into independent modules so that future
integrations can depend on core runtime types without pulling in CLI,
TUI, or unrelated dependencies.

### Agreed Layout

| Module | Go Module Path | Contains |
|--------|---------------|----------|
| `quarry-core/` | `github.com/justapithecus/quarry-core` | types, runtime, policy, lode, adapter, proxy, metrics, log |
| `quarry-cli/` | `github.com/justapithecus/quarry-cli` | CLI commands, TUI, config, reader |

### Prerequisites

- [x] Decouple lode from cli/reader dependency (PR #113)
- [ ] Zero cli/tui imports from core packages

### Sequencing

1. Extract `quarry-core/` — move types, runtime, policy, lode, adapter,
   proxy, metrics, log into standalone module.
2. Extract `quarry-cli/` — CLI, TUI, config, reader depend on
   `quarry-core/`.
3. Update CI — build matrix, release workflows, go.work for local dev.

---

## Temporal Orchestration Integration (Deferred)

> **Status: Deferred.** Temporal integration is the right answer for teams
> that already operate Temporal infrastructure and need complex DAG
> orchestration, conditional branching, or human-in-the-loop workflows.
> However, most Quarry users do not run Temporal, and the most common
> gap — crawling with discovered work — is better served by `quarry crawl`
> (v0.6.0) with zero infrastructure requirements.
>
> Temporal remains a valid future integration. When demand justifies it,
> the module split (above) is its prerequisite.

### Goal

Enable Quarry to run as a Temporal activity, with workflow-level retries,
lineage tracking, and durable execution guarantees.

### Prerequisites

- [ ] Module split complete (`quarry-core/` extracted)

### Deliverables

- Activity wrapper calling `runtime.NewRunOrchestrator(config).Execute(ctx)`
- Heartbeat goroutine for liveness reporting
- Outcome mapping: `OutcomeStatus` → Temporal error semantics
- Reference workflow with lineage-aware retry logic
- Worker binary for `quarry-temporal/`

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
