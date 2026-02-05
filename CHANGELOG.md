# Changelog

All notable changes to Quarry will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- **CLI**: `--job-json <path>` flag to load job payload from file (alternative to inline `--job`)
- **Docs**: Downstream integration guide with event-bus and polling patterns
- **CI**: CLI/config parity checker validates flag documentation against implementation
- **CI**: Strict parity gate blocks merge on CLI documentation drift
- **Examples**: Integration pattern examples (SNS, handler, filesystem polling, S3 polling)

### Changed

- **CLI**: `--job` description clarified as "inline JSON" to distinguish from `--job-json`
- **CLI**: Job payload (`--job` and `--job-json`) now **requires** a top-level JSON object; arrays, primitives, and null are rejected with actionable error messages
- **CLI**: Normalized help text for job, storage, and policy flags for consistency

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

[0.1.0]: https://github.com/justapithecus/quarry/releases/tag/v0.1.0
