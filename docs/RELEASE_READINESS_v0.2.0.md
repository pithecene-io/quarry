# Release Readiness: v0.2.0

Objective go/no-go checklist for Quarry v0.2.0 release.

**Status**: Pre-release
**Target**: After all gates pass

---

## Release Gates

All gates must pass before version bump.

### Gate 1: Object-Only Payload Tests

**Requirement**: All payload contract tests pass.

| Check | Test Command | Status |
|-------|--------------|--------|
| Inline object accepted | `go test ./cli/cmd -run TestParseJobPayload/inline_object_accepted` | |
| File object accepted | `go test ./cli/cmd -run TestParseJobPayload/file_object_accepted` | |
| Inline array rejected | `go test ./cli/cmd -run TestParseJobPayload/inline_array_rejected` | |
| File array rejected | `go test ./cli/cmd -run TestParseJobPayload/file_array_rejected` | |
| Inline primitive rejected | `go test ./cli/cmd -run TestParseJobPayload/inline_primitive` | |
| File primitive rejected | `go test ./cli/cmd -run TestParseJobPayload/file_primitive_rejected` | |
| Inline null rejected | `go test ./cli/cmd -run TestParseJobPayload/inline_null_rejected` | |
| File null rejected | `go test ./cli/cmd -run TestParseJobPayload/file_null_rejected` | |
| Malformed inline rejected | `go test ./cli/cmd -run TestParseJobPayload/malformed_inline` | |
| Malformed file rejected | `go test ./cli/cmd -run TestParseJobPayload/malformed_file` | |
| Both flags rejected | `go test ./cli/cmd -run TestParseJobPayload/both_flags` | |
| Neither flag returns `{}` | `go test ./cli/cmd -run TestParseJobPayload/neither_flag` | |

**Validation command**:
```bash
go test ./cli/cmd -run TestParseJobPayload -v
```

---

### Gate 2: CLI Parity Checker

**Requirement**: Parity artifact is synchronized with CLI implementation.

| Check | Test Command | Status |
|-------|--------------|--------|
| Run command parity | `go test ./cli/cmd -run TestCLIParityRunCommand` | |
| List command parity | `go test ./cli/cmd -run TestCLIParityListCommand` | |
| Debug command parity | `go test ./cli/cmd -run TestCLIParityDebugCommand` | |
| Job payload contract | `go test ./cli/cmd -run TestCLIParityJobPayloadContract` | |

**Validation command**:
```bash
go test ./cli/cmd -run Parity -v
```

---

### Gate 3: Strict CI Parity Gate

**Requirement**: CI workflow includes parity check as blocking job.

| Check | Verification | Status |
|-------|--------------|--------|
| Parity job defined | `.github/workflows/ci.yml` contains `cli-parity` job | |
| Job runs `go test -run Parity` | Verify command in workflow | |
| Job fails on drift | Introduce intentional drift, verify CI fails | |

**Validation**: Push branch with parity drift, confirm CI blocks.

---

### Gate 4: Integration Documentation

**Requirement**: Integration guide and examples are complete.

| Check | Location | Status |
|-------|----------|--------|
| Integration guide exists | `docs/guides/integration.md` | |
| Event-bus pattern documented | Guide includes SNS/Kafka examples | |
| Polling pattern documented | Guide includes filesystem/S3 examples | |
| Idempotency covered | Guide addresses duplicate handling | |
| Link from PUBLIC_API.md | Downstream Integration section | |

---

### Gate 5: Integration Examples

**Requirement**: Runnable/readable integration examples exist.

| Check | Location | Status |
|-------|----------|--------|
| Example README | `examples/integration-patterns/README.md` | |
| Event-bus SNS example | `examples/integration-patterns/event-bus-sns.sh` | |
| Event-bus handler | `examples/integration-patterns/event-bus-handler.ts` | |
| Polling filesystem example | `examples/integration-patterns/polling-fs.sh` | |
| Polling S3 example | `examples/integration-patterns/polling-s3.py` | |

---

### Gate 6: CI/Nightly Thresholds

**Requirement**: CI and nightly jobs pass consistently.

| Check | Job | Threshold | Status |
|-------|-----|-----------|--------|
| CI tests pass | `test` job | 100% | |
| CI lint passes | `lint` job | 0 errors | |
| CI build passes | `build` job | Success | |
| CI examples pass | `examples` job | All examples | |
| Version lockstep | `version-lockstep` job | Versions match | |
| CLI parity | `cli-parity` job | No drift | |

**Validation command**:
```bash
task test && task lint && task build && task examples
```

---

### Gate 7: Open Issues

**Requirement**: No open P0 or P1 issues blocking release.

| Check | Verification | Status |
|-------|--------------|--------|
| P0 issues | `gh issue list --label P0` returns empty | |
| P1 issues | `gh issue list --label P1` reviewed and deferred | |

---

## Pre-Release Checklist

Complete after all gates pass:

- [ ] All gates verified passing
- [ ] CHANGELOG.md Unreleased section finalized
- [ ] README.md consistent with current behavior
- [ ] PUBLIC_API.md consistent with current behavior
- [ ] All guides consistent with current behavior
- [ ] No stale TODOs in code
- [ ] Version bump PR prepared:
  - [ ] Update `quarry/types/version.go` to `0.2.0`
  - [ ] Update `sdk/package.json` version to `0.2.0`
  - [ ] Rebuild SDK: `pnpm exec tsdown` in sdk/
  - [ ] Single commit: `chore(release): 🔖 bump version to v0.2.0`
- [ ] Tag created: `git tag v0.2.0`
- [ ] Release notes drafted

---

## Sign-Off

| Role | Name | Date | Approved |
|------|------|------|----------|
| Maintainer | | | |
| Reviewer | | | |

---

## References

- [CHANGELOG.md](../CHANGELOG.md)
- [PUBLIC_API.md](../PUBLIC_API.md)
- [AGENTS.md](../AGENTS.md) — Version Policy section
- [docs/guides/integration.md](guides/integration.md)
