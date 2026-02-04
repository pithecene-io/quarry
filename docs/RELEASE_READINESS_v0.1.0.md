# Release Readiness Checklist — v0.1.0

This document tracks release gates for Quarry v0.1.0.
All gates must be satisfied before tagging a release.

---

## Hard Gates (Blocking)

| Gate | Status | Evidence |
|------|--------|----------|
| CI green on main | ⬜ | [CI Run](#) |
| Nightly green (3+ consecutive) | ⬜ | [Nightly Runs](#) |
| Release dry-run passes | ⬜ | [Dry-run Log](#) |
| Examples validated end-to-end | ⬜ | [Example Logs](#) |
| Version lockstep verified | ⬜ | `quarry/types/version.go` == `sdk/package.json` |

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
| Partial flush + new events matrix | ✅ | TwoPhase_NewEventsAfterEventFailure (existing) |
| No silent data-loss paths | ✅ | TestRunOrchestrator_NoSilentDataLoss_* |

### Phase 5 — Storage Backend Hardening

| Item | Status | Notes |
|------|--------|-------|
| FS: permissions/path validation | ⬜ | |
| FS: disk-full/error behavior tests | ⬜ | |
| S3: config validation tests | ⬜ | |
| S3: auth failure handling | ⬜ | |
| S3: consistency caveats documented | ⬜ | |
| Checksum internal-only and off | ⬜ | |
| Backpressure/retry behavior documented | ⬜ | Or explicit non-goals |

### Phase 6 — CI/Nightly/Release Reliability

| Item | Status | Notes |
|------|--------|-------|
| CI: lint/test/build/examples required | ⬜ | |
| Nightly: test:race meaningful | ⬜ | |
| Nightly: artifacts/logs retained | ⬜ | |
| Release: semver tag check | ⬜ | |
| Release: version lockstep check | ⬜ | |
| Release: package + hold + release flow | ⬜ | |
| Release: GitHub Packages publish | ⬜ | |
| Release: dry-run workflow added | ⬜ | |
| Release: missing checks block release | ⬜ | |
| Release: pre-publish validation | ⬜ | |
| Successful full dry-run completed | ⬜ | |

### Phase 7 — Go/No-Go Review

| Item | Status | Notes |
|------|--------|-------|
| All phase exit criteria complete | ⬜ | |
| No open P0/P1 defects | ⬜ | |
| Docs/examples/CI green on main | ⬜ | |
| Known limitations documented | ⬜ | |
| Support posture documented | ⬜ | |
| Release decision doc complete | ⬜ | |

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
