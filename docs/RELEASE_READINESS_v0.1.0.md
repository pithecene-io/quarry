# Release Readiness Checklist — v0.1.0

This document tracks release gates for Quarry v0.1.0.
All gates must be satisfied before tagging a release.

---

## Hard Gates (Blocking)

| Gate | Status | Evidence |
|------|--------|----------|
| CI green on main | ✅ | Local validation (task lint/test/build/examples) |
| Nightly green (3+ consecutive) | ⬜ | [Nightly Runs](#) |
| Release dry-run passes | ✅ | v0.1.0 dry-run passed |
| Examples validated end-to-end | ✅ | 5/5 examples pass (task examples) |
| Version lockstep verified | ✅ | 0.1.0 (task version:lockstep) |

---

## Phase Checklist

### Phase 1 — Example Correctness

| Item | Status | Notes |
|------|--------|-------|
| Single supported script format decided | ✅ | TypeScript (.ts) via Node native type-stripping |
| All examples executable as documented | ✅ | 5/5 pass |
| Per-example success assertions added | ✅ | manifest.json with expected outcomes |
| Negative example (failure path) added | ✅ | intentional-failure/ |
| No external websites; local fixtures only | ✅ | All use local HTML fixtures |
| Artifact example validates chunk+commit | ✅ | artifact-snapshot passes |
| Deterministic outputs (stable order) | ✅ | Event counts stable |
| `task examples` executes, not just checks | ✅ | Runs full suite |
| Example run logs in CI artifacts | ✅ | upload-artifact in ci.yml |

### Phase 2 — Public API Documentation

| Item | Status | Notes |
|------|--------|-------|
| PUBLIC_API.md rewritten for first-run clarity | ✅ | Complete rewrite |
| Prerequisites with exact versions | ✅ | Go 1.25.6 (exact), Node 23+ (minimum), pnpm 10.28.2 (exact) |
| Minimal setup documented | ✅ | Clone, build, run example |
| One canonical run command | ✅ | `quarry run` with required flags |
| Troubleshooting section | ✅ | 5 common issues |
| No Lode internals exposed | ✅ | Quarry-level storage terms only |
| Script authoring contract section | ✅ | Export shape, context, terminal behavior |
| Known limitations section | ✅ | 6 limitations documented |
| Doc snippets from tested examples | ✅ | demo.ts and artifact-snapshot |
| Every command has CI counterpart | ⚠️ | Core tasks covered; inspect/stats not exercised |
| Internal doc review sign-off | ⬜ | Pending review |

### Phase 3 — CLI Ergonomics

| Item | Status | Notes |
|------|--------|-------|
| Required/optional flags normalized | ✅ | script, run-id, source, storage-backend, storage-path required |
| Storage flags validated with clear errors | ✅ | Path existence, directory check, backend validation |
| `--help` examples match working examples | ✅ | UsageText with 4 examples (3 canonical + 1 advanced) |
| Run output includes metadata + summary | ✅ | Existing printRunResult is comprehensive |
| JSON/job payload validation | ✅ | Clear error with examples |
| Storage backend/path validation | ✅ | validateStorageConfig with actionable errors |
| No silent fallbacks | ✅ | Errors instead of warnings for invalid config |
| CLI UX tests for misconfigurations | ✅ | run_test.go with 25 test cases |
| Error messages include "what to do next" | ✅ | All errors include guidance |
| Exit codes semantic (config vs script) | ✅ | exitConfigError for CLI validation failures |
| Executor auto-resolution | ✅ | --executor optional, bundled/PATH lookup |
| Canonical examples no --executor | ✅ | Docs updated, advanced section for override |

### Phase 4 — Runtime & Ingestion Resilience

| Item | Status | Notes |
|------|--------|-------|
| Sink write failure (before chunks) tested | ✅ | TestRunOrchestrator_SinkWriteFailure_BeforeChunks |
| Sink write failure (after chunks) tested | ✅ | TestRunOrchestrator_SinkWriteFailure_OnChunks |
| Executor crash mid-stream tested | ✅ | TestRunOrchestrator_ExecutorCrashMidStream, _TruncatedFrame |
| Malformed frame tested | ✅ | TestIngestionEngine_FrameDecodeError (existing) |
| Policy flush failure tested | ✅ | TestRunOrchestrator_PolicyFlushFailure_OutcomeMapping |
| Outcome mapping verified per failure | ✅ | TestOutcomeMapping_ExitCodes, _DetermineOutcome |
| Buffered policy ordering invariants | ✅ | TestBufferedPolicy_EventsWrittenInSequenceOrder (existing) |
| Chunk/commit invariants upheld | ✅ | TestBufferedPolicy_AllEventsWrittenTogether (existing) |
| Partial flush + new events scenarios | ✅ | TwoPhase_NewEvents/Chunks_AfterEventFailure (existing) |
| No silent data-loss paths | ✅ | TestRunOrchestrator_NoSilentDataLoss_* |

### Phase 5 — Storage Backend Hardening

| Item | Status | Notes |
|------|--------|-------|
| FS: permissions/path validation | ✅ | validateStorageConfig() in run.go |
| FS: write error propagation tested | ✅ | Sink propagates client errors (TestSink_*Error) |
| S3: config validation tests | ✅ | TestS3Config_Validate (existing) |
| S3: auth failure handling | ✅ | Documented in guides/lode.md |
| S3: consistency caveats documented | ✅ | Documented in guides/lode.md |
| Checksum internal-only and off | ✅ | checksumEnabled = false (client.go) |
| Backpressure/retry behavior documented | ✅ | Explicit non-goals in guides/lode.md |

### Phase 6 — CI/Nightly/Release Reliability

| Item | Status | Notes |
|------|--------|-------|
| CI: lint/test/build/examples required | ✅ | All jobs in ci.yml; branch protection TBD |
| Nightly: test:race meaningful | ✅ | Runs at 4am daily |
| Nightly: artifacts/logs retained | ✅ | 14-day retention on failure |
| Release: semver tag check | ✅ | Regex match ^v[0-9]+\.[0-9]+\.[0-9]+$ |
| Release: version lockstep check | ✅ | Verifies Go/SDK/Tag versions match |
| Release: package + hold + release flow | ✅ | hold job with release environment |
| Release: GitHub Packages publish | ✅ | publish_npm job to npm.pkg.github.com |
| Release: dry-run workflow added | ✅ | release-dry-run.yml with full validation |
| Release: missing checks block release | ⚠️ | Branch protection config needed (manual) |
| Release: pre-publish validation | ✅ | lint/test/build/examples before package |
| Successful full dry-run completed | ✅ | v0.1.0 dry-run passed |

### Phase 7 — Go/No-Go Review

| Item | Status | Notes |
|------|--------|-------|
| All phase exit criteria complete | ⚠️ | See phase gaps below |
| No open P0/P1 defects | ✅ | Confirmed zero open issues |
| Docs/examples/CI green on main | ✅ | All tasks pass locally |
| Known limitations documented | ✅ | PUBLIC_API.md § Known Limitations |
| Support posture documented | ✅ | SUPPORT.md |
| Release decision doc complete | ✅ | This document |

**Phase gaps requiring resolution:**
- Phase 2: Internal doc review sign-off (⬜)
- Phase 6: Branch protection config (⚠️ manual)

### Phase 8 — Storage Failure Hardening

| Item | Status | Notes |
|------|--------|-------|
| FS: directory creation failure tested | ⬜ | Real mkdir failures (permissions, path) |
| FS: file write failure tested | ⬜ | Real write failures (disk full, quota) |
| FS: atomic write semantics validated | ⬜ | Partial write detection |
| S3: auth failure tested | ⬜ | Invalid/expired credentials |
| S3: bucket access denied tested | ⬜ | Missing permissions |
| S3: network timeout tested | ⬜ | Transient network failures |
| S3: throttling (429) tested | ⬜ | Rate limit handling |
| Error messages include storage context | ⬜ | Path/bucket in errors |
| Policy failure propagation verified | ⬜ | Storage errors → run outcome |
| No silent corruption paths | ⬜ | All partial writes detectable |

### Phase 9 — Bundle Executor with Go Distribution

| Item | Status | Notes |
|------|--------|-------|
| Executor embedded in quarry binary | ⬜ | Go embed for Node executor |
| Executor extraction to temp dir | ⬜ | Extract on first run |
| Executor version validation | ⬜ | Embedded matches expected |
| Fallback to PATH executor | ⬜ | If embedded extraction fails |
| Cross-platform extraction tested | ⬜ | Linux, macOS, Windows |
| Extraction permissions correct | ⬜ | Executable bit set |
| Temp dir cleanup on exit | ⬜ | No orphaned extractors |
| `--executor` override still works | ⬜ | Explicit path takes precedence |
| Build reproduces embedded content | ⬜ | Deterministic embed |
| Binary size impact documented | ⬜ | Size delta tracked |

---

## Scope Summary

### Supported (v0.1.0)

- Node.js executor with TypeScript/JavaScript scripts
- Filesystem storage backend
- Buffered ingestion policies (FlushAtLeastOnce, FlushTwoPhase)
- Event and artifact emission via SDK
- CLI: `run`, `inspect`, `stats` commands

### Experimental (v0.1.0)

- S3 storage backend (requires explicit opt-in)

### Out of Scope (Post-v0.1.0)

- TBD

---

## Sign-off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Maintainer | | | |
| Reviewer | | | |

---

*Last updated: 2026-02-03*
