# Lode Upgrade: v0.2.0 → v0.4.1

This document records compatibility analysis for the Lode dependency
upgrade shipped in Quarry v0.3.0.

---

## Upgrade Path

Quarry previously pinned `github.com/pithecene-io/lode v0.2.0`.
The upgrade skips v0.3.0 and v0.4.0, landing on v0.4.1.

### Lode v0.3.0 (docs/test only)
- No API changes.
- Documentation, examples, and streaming failure test coverage.

### Lode v0.4.0 (compression + streaming API change)
- **Breaking**: `StreamWriteRecords` parameter order changed to `(ctx, records, metadata)`.
- Adds `NewZstdCompressor()` for zstd compression support.
- New transitive dependency: `klauspost/compress v1.18.3`.

### Lode v0.4.1 (S3 multipart hardening)
- Conditional `CompleteMultipartUpload` for large S3 uploads (closes TOCTOU window).
- No API changes.

---

## Impact on Quarry

**Zero code changes required.**

Quarry uses `Dataset.Write` (batch), not `StreamWriteRecords` (streaming).
The v0.4.0 breaking change to `StreamWriteRecords` does not affect Quarry.

| Surface | v0.2.0 → v0.4.1 Change | Quarry Impact |
|---------|------------------------|---------------|
| `lode.Store` interface | Identical (7 methods) | None |
| `lode.Dataset` interface | Expanded (+StreamWrite, +StreamWriteRecords) | None — Quarry only uses `Write` |
| `NewDataset` / `StoreFactory` / options | Identical signatures | None |
| S3 `API` interface | Expanded (+4 multipart methods) | None — `*s3.Client` satisfies all |
| S3 `New()` + `Config` | Identical | None |
| HiveLayout / JSONL codec | Identical | None |
| Partition layout | Unchanged (source/category/day/run_id/event_type) | None |

---

## CONTRACT_LODE.md Invariants

All invariants verified post-upgrade:

- Partition keys respected (source, category, day, run_id, event_type)
- Append-only write semantics preserved
- Chunks-before-commit ordering preserved
- Offset tracking across batches preserved
- Error classification (typed `StorageError` assertions) preserved
- Record kinds (event, artifact_event, artifact_chunk, metrics) unchanged

---

## New Transitive Dependencies

| Dependency | Version | Reason |
|------------|---------|--------|
| `klauspost/compress` | v1.18.3 | Zstd support in Lode (used internally for compression codec) |

---

## Rollback

If issues arise, revert `go.mod` to pin `v0.2.0`. No data migration
is needed — Lode's on-disk format is unchanged between v0.2.0 and v0.4.1.
