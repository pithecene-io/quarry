# Changelog

All notable changes to Quarry will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

_No unreleased changes._

---

## [0.3.4] - 2026-02-07

### Fixed

- **Lode**: Fixed nil-map panic in S3 client constructor — `NewLodeS3Client` failed to initialize artifact offset and chunk-tracking maps, causing runtime panic on first `WriteChunks` call (#92)

### Changed

- **Lode**: Extracted shared `newClient()` constructor helper to prevent future initialization drift between FS and S3 client paths (#92)

### Added

- **Testing**: Regression test `TestNewClient_InitializesMaps` guards against nil-map constructor bugs (#92)

---

## [0.3.3] - 2026-02-07

### Added

- **Storage**: S3-compatible provider support — new `--storage-endpoint` and `--storage-s3-path-style` CLI flags for Cloudflare R2, MinIO, and other S3-compatible backends (#87)
- **Docs**: S3-compatible storage flags and R2 examples added to cli.md, configuration.md, and PUBLIC_API.md (#89)
- **Docs**: S3-compatible provider support section added to CONTRACT_LODE.md (#87)

### Fixed

- **Release**: JSR now publishes TypeScript source instead of compiled dist, preserving full type information (#88)
- **CLI**: Fixed duplicate run-id in usage text (#89)

---

## [0.3.2] - 2026-02-07

### Added

- **Release**: Cross-compiled platform binaries (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) included in GitHub Releases (#82)
- **Release**: mise install support — `mise install github:justapithecus/quarry@<version>` (#82)
- **Release**: JSR publishing via OIDC (zero-secret) — `npx jsr add @justapithecus/quarry-sdk` (#83)
- **Release**: Dual SDK distribution on JSR (public) and GitHub Packages (restricted) (#82, #83)
- **Release**: Checksums manifest (`checksums.txt`) included in every release (#82)
- **Docs**: Installation docs updated across README, PUBLIC_API.md, and SDK README (#82)

---

## [0.3.1] - 2026-02-06

### Added

- **Executor**: puppeteer-extra support with stealth and adblocker plugins (#75)
  - Stealth enabled by default (`QUARRY_STEALTH=0` to disable)
  - Adblocker opt-in via `QUARRY_ADBLOCKER=1`
  - No-sandbox mode for CI via `QUARRY_NO_SANDBOX=1`
- **Docs**: Consolidated configuration reference (`docs/guides/configuration.md`) (#81)

### Changed

- **Refactor**: Cleanup passes across SDK, executor, and Go runtime — extract helpers, reduce duplication, declarative validation (#76, #77, #78, #79)

### Fixed

- **Build**: Stabilized embedded executor bundle sync (#80)

---

## [0.3.0] - 2026-02-06

### Added

- **Metrics**: Runtime metrics surface per CONTRACT_METRICS.md — run lifecycle, ingestion drops, executor failures, storage write counters (#65)
- **Metrics**: Persist metrics snapshot to Lode at run end as `record_kind=metrics` record (#67)
- **CLI**: `stats metrics` subcommand with Lode-backed reader for querying persisted metrics (#68)
- **Lode**: Policy name recorded in event and artifact commit Lode records (#71)
- **Contracts**: CONTRACT_METRICS.md and CONTRACT_INTEGRATION.md added (#63)
- **Contracts**: Metrics stats persistence requirements defined (#66)
- **Docs**: Lode v0.2.0 → v0.4.1 compatibility guide (`docs/guides/lode-upgrade.md`) (#70)

### Changed

- **Lode**: Upgraded Lode dependency from v0.2.0 to v0.4.1 (#64)
- **Docs**: Phase 6 (dogfooding) clarified as post-release validation exercise (#70)
- **Docs**: v0.3.0 deliverables reworded — dogfooding is a prerequisite, not a gate (#70)

### Fixed

- **Go**: Prefer `errors.New` over `fmt.Errorf` for static error strings (#69)

---

## [0.2.2] - 2026-02-05

### Fixed

- **CI**: Fixed GitHub Packages publish auth failure (ENEEDAUTH) in release workflow. Root cause was missing `actions/setup-node` with registry configuration. Fix adds `packages: write` permission, proper registry-url setup, and `publishConfig` in SDK package.json.
- **CI**: Enforced pinned pnpm via Corepack in publish workflow. Replaced global `npm install -g pnpm` with Corepack-based setup that reads `packageManager` from root package.json. Added version verification step that fails fast on mismatch.

---

## [0.2.1] - 2026-02-05

### Fixed

- **IPC**: Fixed race condition where fast-completing scripts intermittently reported `executor_crash` outcome despite successful completion ([#56](https://github.com/justapithecus/quarry/issues/56)). Root cause was Go's `exec.Cmd.Wait()` closing stdout pipe before ingestion completed reading all data.

---

## [0.2.0] - 2026-02-05

### Added

- **CLI**: `--job-json <path>` flag to load job payload from file (alternative to inline `--job`)
- **Docs**: Downstream integration guide with event-bus and polling patterns
- **CI**: CLI/config parity checker validates flag documentation against implementation
- **CI**: Strict parity gate blocks merge on CLI documentation drift
- **Examples**: Integration pattern examples (SNS, handler, filesystem polling, S3 polling)

### Changed

- **CLI**: `--job` description clarified as "inline JSON object" to distinguish from `--job-json`
- **CLI**: Job payload (`--job` and `--job-json`) now **requires** a top-level JSON object; arrays, primitives, and null are rejected with actionable error messages
- **CLI**: Normalized help text for job, storage, and policy flags for consistency
- **Docs**: CLI parity artifact (`docs/CLI_PARITY.json`) added as machine-checkable source of truth

### Known Issues

- **IPC race condition** ([#56](https://github.com/justapithecus/quarry/issues/56)): Fast-completing scripts may intermittently report `executor_crash` outcome despite successful completion. **Fixed in v0.2.1.**

---

## [0.1.0] - 2026-02-04

### Added

- **CLI**: `quarry run` command for executing extraction scripts
- **CLI**: `quarry inspect` command for viewing run data (read-only)
- **CLI**: `quarry stats` command for run statistics
- **SDK**: `@justapithecus/quarry-sdk` npm package with emit API
- **Emit API**: `emit.item()`, `emit.artifact()`, `emit.checkpoint()`, `emit.log()`
- **Terminal events**: `emit.runComplete()`, `emit.runError()`
- **Storage**: Filesystem backend with Hive-partitioned layout
- **Storage**: S3 backend (experimental, requires explicit opt-in)
- **Policies**: Strict and Buffered ingestion policies
- **Proxy**: Runtime proxy configuration and rotation support
- **Executor**: Embedded Node.js executor in Go binary (~28MB)
- **Contracts**: IPC, Emit, Run, Policy, Lode, CLI, Proxy contracts frozen

### Changed

- N/A (initial release)

### Fixed

- N/A (initial release)

### Breaking Changes

- None (initial release)

### Known Limitations

1. **Single executor type**: Only Node.js executor supported
2. **No built-in retries**: Retry logic is caller's responsibility
3. **No streaming reads**: Artifacts must fit in memory
4. **S3 is experimental**: No transactional guarantees across writes
5. **No job scheduling**: Quarry is an execution runtime, not a scheduler
6. **Puppeteer required**: All scripts run in a browser context

### Upgrade Notes

**Runtime Requirements:**
- Go 1.25.6 or later (for building from source)
- Node.js 23+ or 22.6+ (for script execution; 22.6 requires `--experimental-strip-types`)
- pnpm 10.28.2 (for development)

**Puppeteer:**
- Puppeteer is a peer dependency; install it in your project:
  ```bash
  npm install puppeteer
  ```

**Storage:**
- `--storage-backend` and `--storage-path` are required flags
- FS backend path must exist before running

### References

- [PUBLIC_API.md](PUBLIC_API.md) — User-facing API documentation
- [SUPPORT.md](SUPPORT.md) — Support posture and maturity level
- [docs/contracts/](docs/contracts/) — Normative contract specifications
- [docs/guides/](docs/guides/) — User guides and explanations

---

[0.3.4]: https://github.com/justapithecus/quarry/releases/tag/v0.3.4
[0.3.3]: https://github.com/justapithecus/quarry/releases/tag/v0.3.3
[0.3.2]: https://github.com/justapithecus/quarry/releases/tag/v0.3.2
[0.3.1]: https://github.com/justapithecus/quarry/releases/tag/v0.3.1
[0.3.0]: https://github.com/justapithecus/quarry/releases/tag/v0.3.0
[0.2.2]: https://github.com/justapithecus/quarry/releases/tag/v0.2.2
[0.2.1]: https://github.com/justapithecus/quarry/releases/tag/v0.2.1
[0.2.0]: https://github.com/justapithecus/quarry/releases/tag/v0.2.0
[0.1.0]: https://github.com/justapithecus/quarry/releases/tag/v0.1.0
