# Release Readiness: v0.12.0

**Status**: Draft
**Scope**: v0.12.0 release gating
**Basis**: v0.11.0 (2026-02-23) + PRs #196, #197
**Versions tested**: 0.11.0

---

## What's in this release

v0.12.0 delivers three feature areas that close all remaining "Important (not blocking)" items from the v1.0 readiness assessment:

### 1. Storage batching (`createStorageBatcher`)

PR #196. Addresses v1.0 readiness §3.

`createStorageBatcher(storage, { concurrency })` wraps `ctx.storage.put()` with bounded-concurrency pipelining. Callers `add()` storage operations and `flush()` to drain. Default concurrency is 16. Failure in any put rejects all queued entries and propagates through `flush()`.

**Public API surface:**

- `createStorageBatcher(storage, options?)` — factory (exported from `@pithecene-io/quarry-sdk`)
- `StorageBatcher` — `{ add, flush, pending }` return type
- `StorageBatcherOptions` — `{ concurrency?: number }`
- `PendingStoragePut` — `{ result: Promise<StoragePutResult> }`

### 2. Memory pressure API (`ctx.memory`)

PR #196. Addresses v1.0 readiness §4.

`ctx.memory.snapshot()` returns heap, browser, and cgroup memory usage with a composite pressure level. `ctx.memory.isAbove(level)` provides a boolean threshold check. Cgroup v2/v1 auto-detection enables container-aware memory management without DIY `/proc` parsing.

**Public API surface:**

- `ctx.memory.snapshot(options?)` — returns `MemorySnapshot`
- `ctx.memory.isAbove(level)` — returns `Promise<boolean>`
- `MemorySnapshot` — `{ node, browser, cgroup, pressure, ts }`
- `MemoryUsage` — `{ used, limit, ratio }`
- `MemoryPressureLevel` — `'low' | 'moderate' | 'high' | 'critical'`
- `MemoryThresholds` — custom threshold overrides (strictly monotonic, range (0,1])

### 3. End-to-end storage error propagation (`file_write_ack`)

PR #197. Addresses v1.0 readiness §5.

First bidirectional IPC frame. After the runtime processes a `file_write`, it sends a `file_write_ack` back to the executor via stdin. The executor's `AckReader` correlates acks by `write_id` and resolves/rejects the `storage.put()` promise accordingly.

- Storage write failures are **recoverable** — ingestion continues, and the script receives a rejected promise.
- Validation errors (empty filename, path traversal, oversize data) remain **fatal** stream errors.
- Backward compatible: if no acks arrive before stdin EOF, all pending puts resolve silently (fire-and-forget fallback for older runtimes).

**IPC surface:**

- `FileWriteAckFrame` — `{ type: 'file_write_ack', write_id, ok, error? }` (Go + TypeScript)

### Lode upgrade

Both PRs include a Lode upgrade to v0.8.0 (from v0.7.3).

---

## Updated v1.0 scorecard

All 5 items from the v1.0 readiness assessment are now resolved:

| # | Item | Severity | Resolved in |
|---|------|----------|-------------|
| 1 | `storage.put()` returns resolved key | Blocker | v0.11.0 (PR #190) |
| 2 | Structured exit reporting | Blocker | v0.11.0 (PR #189) |
| 3 | Storage batching / concurrency | High | v0.12.0 (PR #196) |
| 4 | Memory pressure awareness | High | v0.12.0 (PR #196) |
| 5 | Storage error surfacing | Medium | v0.12.0 (PR #197) |

### Re-graded dimensions

| Dimension | v1.0 doc | Post-0.12.0 | Notes |
|-----------|----------|-------------|-------|
| **Correctness** | B | A- | `storage.put()` returns key; `file_write_ack` surfaces backend errors; structured exit reporting enables audit |
| **Ergonomics** | B- | A- | `createStorageBatcher` eliminates serial put bottleneck; `ctx.memory` replaces DIY cgroup parsing |
| **Reliability** | B+ | A- | End-to-end error propagation closes the last silent-failure path in the storage pipeline |
| **Performance** | B+ | A- | Bounded-concurrency batching unblocks storage throughput; memory pressure API enables proactive backoff |

**Overall: A- — all identified gaps resolved. Minor friction only.**

---

## Contract changes

| Contract | Changes |
|----------|---------|
| `CONTRACT_IPC.md` | `file_write_ack` frame type, bidirectional framing on stdin, backward compatibility semantics |
| `CONTRACT_EMIT.md` | `StoragePutResult` propagation through ack, `createStorageBatcher` flush/failure semantics |

No changes to CONTRACT_RUN, CONTRACT_POLICY, CONTRACT_PROXY, CONTRACT_CLI, CONTRACT_LODE, CONTRACT_METRICS, or CONTRACT_INTEGRATION.

---

## Known limitations

1. **`file_write_ack` requires runtime support.** Older runtimes (< 0.12.0) that do not send acks will trigger the fire-and-forget fallback — `storage.put()` resolves without confirmation. Scripts cannot distinguish "no ack support" from "successful write" in mixed-version deployments.
2. **Cgroup v1 unlimited sentinel.** Cgroup v1 environments reporting memory limit ≥ 2^62 are treated as "unlimited" (`cgroup: null` in snapshot). This is a reasonable heuristic but not formally specified by the kernel.
3. **Browser memory requires a page.** `ctx.memory.snapshot({ browser: true })` returns `browser: null` if no Puppeteer page is available. Scripts must handle this case.

---

## Release gates

- [ ] CI green on main (`task lint`, `task test`, `task build`, `task examples`)
- [ ] Lockstep version bump (see checklist below)
- [ ] Contracts updated (CONTRACT_IPC.md, CONTRACT_EMIT.md) — already landed in PRs #196, #197
- [ ] CHANGELOG.md `[Unreleased]` promoted to `[0.12.0] - YYYY-MM-DD`
- [ ] Golden test fixtures updated (`sdk/test/emit/06-golden/*.json`)
- [ ] Executor bundle rebuilt (`task executor:bundle`)
- [ ] PUBLIC_API.md updated for new API surface
- [ ] Release workflow dry-run passes
- [ ] Release notes drafted

---

## Version bump checklist

Per AGENTS.md lockstep versioning:

1. [ ] `quarry/types/version.go` — `Version = "0.12.0"`
2. [ ] `sdk/package.json` — `"version": "0.12.0"`
3. [ ] `sdk/src/types/events.ts` — `CONTRACT_VERSION = "0.12.0"`
4. [ ] Golden fixtures — `contract_version` values in `sdk/test/emit/06-golden/*.json`
5. [ ] Rebuild SDK — `pnpm exec tsdown` in `sdk/`
6. [ ] Rebuild executor bundle — `task executor:bundle`
7. [ ] CHANGELOG.md — promote `[Unreleased]` to `[0.12.0]`, add link reference
8. [ ] Single commit

---

## Sign-off

| Role | Name | Date | Approved |
|------|------|------|----------|
| Maintainer | | | |
| Reviewer | | | |
