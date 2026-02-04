# Quarry PUBLIC API (User Guide)

This document explains the public surface for Quarry users: how to set up
projects, write scripts, and run jobs. Normative behavior is defined by
contracts under `docs/contracts/`.

---

## Quick Start

Run a script:

```
quarry run \
  --script ./examples/demo.ts \
  --run-id run-001 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --job '{"source":"demo"}'
```

---

## Project Setup

Quarry scripts are TypeScript and must export a default async function.
The function receives a `QuarryContext` and emits output via `emit.*`.

Minimal script shape:

```
import type { QuarryContext } from "@justapithecus/quarry-sdk";

export default async function run(ctx: QuarryContext): Promise<void> {
  await ctx.emit.item({
    item_type: "example",
    data: { hello: "world" }
  });
}
```

---

## Storage Configuration (Quarry-level)

Storage is a Quarry concern. Lode is internal and not user-configured.

Required flags:
- `--storage-backend` (`fs` or `s3`)
- `--storage-path` (fs: directory, s3: bucket/prefix)

Optional:
- `--storage-region` (s3 only, uses default AWS chain if omitted)

---

## Run Identity

Required:
- `--run-id <id>`

Optional:
- `--attempt <n>` (default: 1)
- `--job-id <id>`
- `--parent-run-id <id>`

Lineage and retry rules follow `CONTRACT_RUN.md`.

---

## Emit API (Script Output)

`emit.*` is the sole output mechanism:
- `emit.item` — primary structured output
- `emit.artifact` — binary artifacts (screenshots, files)
- `emit.checkpoint` — progress markers
- `emit.log` — structured logs
- `emit.runError` / `emit.runComplete` — terminal events

Event ordering is preserved per run.

---

## Artifacts (Chunking and Commit)

Artifacts are written as chunks and committed by an `artifact` event.
Chunks are written before the commit record. Orphaned chunks are possible
on failure and are acceptable.

---

## Examples

Examples live under `examples/`:
- `examples/demo.ts` — minimal item emission
- `examples/static-html-list/` — parse static HTML list
- `examples/toy-pagination/` — pagination over fixtures
- `examples/artifact-snapshot/` — screenshot artifact

---

## Support Notes

Quarry is TypeScript-first and ESM-only. Scripts should be `.ts` and use
`export default async function`.
