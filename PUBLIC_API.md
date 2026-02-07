# Quarry Public API

User-facing guide for Quarry v0.3.4.
Normative behavior is defined by contracts under `docs/contracts/`.

---

## Prerequisites

| Tool | Version | Source |
|------|---------|--------|
| Go | 1.25.6 | `quarry/go.mod` |
| Node.js | 23+ or 22.6+ | Native TS support (see note) |
| pnpm | 10.28.2 | `package.json#packageManager` |

**Node.js version note:** Quarry scripts are TypeScript and use Node's native
type-stripping. Node 23+ enables this by default. Node 22.6–22.x requires
`--experimental-strip-types` flag (set via `NODE_OPTIONS`).

Quarry is **TypeScript-first** and **ESM-only**.

---

## Installation

### Via mise (recommended)

```bash
mise install github:justapithecus/quarry@0.3.4
```

Or pin in your `mise.toml`:

```toml
[tools]
"github:justapithecus/quarry" = "0.3.4"
```

### SDK

Install the SDK in your script project:

```bash
npx jsr add @justapithecus/quarry-sdk
```

Or via pnpm with GitHub Packages:

```bash
pnpm add @justapithecus/quarry-sdk
```

### From source

```bash
git clone https://github.com/justapithecus/quarry.git
cd quarry
pnpm install
task build
```

---

## Quick Start

### 1. Clone and Build

```bash
git clone https://github.com/justapithecus/quarry.git
cd quarry
pnpm install
task build
```

### 2. Run an Example

```bash
mkdir -p ./quarry-data

./quarry/quarry run \
  --script ./examples/demo.ts \
  --run-id my-first-run \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data
```

Expected output:
```
run_id=my-first-run, attempt=1, outcome=success, duration=...
```

---

## Script Authoring

### Export Shape

Scripts **must** export a default async function:

```typescript
import type { QuarryContext } from "@justapithecus/quarry-sdk";

export default async function run(ctx: QuarryContext): Promise<void> {
  // Your extraction logic here
}
```

This is the **only** supported entry point shape.

### Context Object

The `QuarryContext` provides:

| Property | Type | Description |
|----------|------|-------------|
| `job` | `unknown` | Job payload (from `--job` or `--job-json`) |
| `run` | `RunMeta` | Run metadata (run_id, attempt, job_id, etc.) |
| `page` | `Page` | Puppeteer page instance |
| `browser` | `Browser` | Puppeteer browser instance |
| `browserContext` | `BrowserContext` | Puppeteer browser context |
| `emit` | `EmitAPI` | Output emission interface |

### Terminal Behavior

A script terminates in one of these ways:

1. **Normal return** — Executor emits `run_complete` automatically
2. **Throw error** — Executor emits `run_error` automatically
3. **Explicit `emit.runComplete()`** — Script signals success
4. **Explicit `emit.runError()`** — Script signals failure

After a terminal event, further emit calls are undefined behavior.

---

## Emit API

`emit.*` is the **sole output mechanism** for scripts.

### Primary Methods

```typescript
// Emit structured data (primary output)
await ctx.emit.item({
  item_type: "product",
  data: { name: "Widget", price: 9.99 }
});

// Emit binary artifact (screenshot, PDF, etc.)
const artifactId = await ctx.emit.artifact({
  name: "screenshot.png",
  content_type: "image/png",
  data: buffer  // Buffer or Uint8Array
});

// Emit progress checkpoint
await ctx.emit.checkpoint({
  checkpoint_id: "page-2",
  note: "Completed page 2 of 10"
});
```

### Logging

```typescript
await ctx.emit.debug("Debug message", { field: "value" });
await ctx.emit.info("Info message");
await ctx.emit.warn("Warning message");
await ctx.emit.error("Error message");
```

### Advisory Methods (Not Guaranteed)

```typescript
// Suggest enqueueing additional work
await ctx.emit.enqueue({
  target: "detail-page",
  params: { url: "https://example.com/item/123" }
});

// Suggest proxy rotation
await ctx.emit.rotateProxy({ reason: "rate limited" });
```

---

## CLI Reference

### Run Command

```bash
quarry run [options]
```

**Required flags:**

| Flag | Description |
|------|-------------|
| `--script <path>` | Path to TypeScript script |
| `--run-id <id>` | Unique run identifier |
| `--source <name>` | Source identifier for partitioning |
| `--storage-backend <fs\|s3>` | Storage backend |
| `--storage-path <path>` | Storage location |

**Optional flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--attempt <n>` | 1 | Attempt number |
| `--job <json>` | `{}` | Job payload as inline JSON object |
| `--job-json <path>` | | Path to JSON file containing job payload (must be object) |
| `--category <name>` | `default` | Category for partitioning |
| `--policy <strict\|buffered>` | `strict` | Ingestion policy |

> **Note:** `--job` and `--job-json` are mutually exclusive. Using both is an error.
> The payload **must** be a top-level JSON object. Arrays, primitives, and null are rejected.

**Advanced flags:**

| Flag | Description |
|------|-------------|
| `--executor <path>` | Override executor path (auto-resolved by default) |

**Storage flags (S3 / S3-compatible):**

| Flag | Description |
|------|-------------|
| `--storage-region <region>` | AWS region (uses default chain if omitted) |
| `--storage-endpoint <url>` | Custom S3 endpoint URL (e.g. Cloudflare R2, MinIO) |
| `--storage-s3-path-style` | Force path-style addressing (required by R2, MinIO) |

### Inspect Command

```bash
quarry inspect run <run-id>     # Inspect a specific run
quarry inspect job <job-id>     # Inspect a specific job
```

### Stats Command

```bash
quarry stats runs               # Run statistics
quarry stats jobs               # Job statistics
quarry stats metrics            # Contract metrics (requires --storage-backend, --storage-path)
```

---

## Examples

All examples use local fixtures (no external network calls).

| Example | Description | Events |
|---------|-------------|--------|
| `examples/demo.ts` | Minimal item emission | 1 item |
| `examples/static-html-list/` | Parse HTML fixture | 3 items |
| `examples/toy-pagination/` | Multi-page pagination | 6 items |
| `examples/artifact-snapshot/` | Screenshot artifact | 1 artifact |
| `examples/intentional-failure/` | Error handling test | 1 item + error |

Run all examples:
```bash
task examples
```

### Integration Pattern Examples

See `examples/integration-patterns/` for downstream ETL trigger examples:
- Event-bus pattern (SNS/SQS, handler)
- Polling pattern (filesystem, S3)

### Example: Minimal Item Emission

```typescript
// examples/demo.ts
import type { QuarryContext } from "@justapithecus/quarry-sdk";

export default async function run(ctx: QuarryContext): Promise<void> {
  await ctx.emit.item({
    item_type: "demo",
    data: { message: "hello from quarry" }
  });
}
```

### Example: Screenshot Artifact

```typescript
// examples/artifact-snapshot/script.ts
import type { QuarryContext } from "@justapithecus/quarry-sdk";

export default async function run(ctx: QuarryContext): Promise<void> {
  await ctx.page.setContent("<h1>Hello</h1>");
  const data = await ctx.page.screenshot({ type: "png" });
  await ctx.emit.artifact({
    name: "snapshot.png",
    content_type: "image/png",
    data
  });
}
```

---

## Troubleshooting

### "Module not found" for SDK

Ensure SDK is built:
```bash
pnpm -C sdk run build
```

### "Executor not found"

The executor is auto-resolved but must be built first:
```bash
pnpm -C executor-node run build
```

If auto-resolution fails, specify the path manually:
```bash
./quarry/quarry run --executor ./executor-node/dist/bin/executor.js ...
```

### Chrome/Puppeteer sandbox errors (CI)

Set environment variable:
```bash
QUARRY_NO_SANDBOX=1 quarry run ...
```

### "Contract version mismatch"

SDK and runtime versions must match. Rebuild both:
```bash
task build
```

### Storage path errors

- **FS backend**: Path must be a writable directory
- **S3 backend**: Format is `bucket/prefix`, credentials via AWS default chain

---

## Known Limitations (v0.3.4)

1. **Single executor type**: Only Node.js executor supported
2. **No built-in retries**: Retry logic is caller's responsibility
3. **No streaming reads**: Artifacts must fit in memory
4. **S3 is experimental**: S3 and S3-compatible providers (R2, MinIO) are supported but experimental; no transactional guarantees across writes
5. **No job scheduling**: Quarry is an execution runtime, not a scheduler
6. **Puppeteer required**: All scripts run in a browser context

---

## Storage Configuration

Storage is a **Quarry-level** concern. The underlying Lode subsystem is internal.

### Filesystem Backend

```bash
--storage-backend fs \
--storage-path /var/quarry/data
```

Data is stored in Hive-partitioned layout:
```
/var/quarry/data/
  source=my-source/
    category=default/
      day=2026-02-03/
        run_id=run-001/
          event_type=item/
            data.jsonl
```

### S3 Backend (Experimental)

```bash
--storage-backend s3 \
--storage-path my-bucket/quarry-data \
--storage-region us-east-1
```

Uses AWS default credential chain. IAM permissions required:
- `s3:PutObject`
- `s3:GetObject`
- `s3:ListBucket`

### S3-Compatible Providers (R2, MinIO)

Quarry supports S3-compatible storage providers via custom endpoint and path-style flags:

```bash
--storage-backend s3 \
--storage-path my-bucket/quarry-data \
--storage-endpoint https://ACCOUNT_ID.r2.cloudflarestorage.com \
--storage-s3-path-style
```

Set credentials via environment variables:
```bash
export AWS_ACCESS_KEY_ID=<access-key>
export AWS_SECRET_ACCESS_KEY=<secret-key>
```

---

## Downstream Integration

Quarry is an extraction runtime, not a full pipeline. For triggering downstream
processing after runs complete, see [docs/guides/integration.md](docs/guides/integration.md).

**Recommended pattern**: Event-bus-driven triggers after storage commit.
**Fallback pattern**: Polling-based triggers with idempotent checkpoints.

---

## Version Information

```bash
quarry version
# 0.3.4 (commit: ...)
```

SDK and runtime versions must match (lockstep versioning).

### Distribution Channels

| Component | Channel | Install |
|-----------|---------|---------|
| CLI binary | GitHub Releases | `mise install github:justapithecus/quarry@0.3.4` |
| SDK | JSR | `npx jsr add @justapithecus/quarry-sdk` |
| SDK | GitHub Packages | `pnpm add @justapithecus/quarry-sdk` |
