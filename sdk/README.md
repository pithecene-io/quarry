# @justapithecus/quarry-sdk

TypeScript SDK for writing Quarry extraction scripts.

## Installation

```bash
# via JSR (recommended)
npx jsr add @justapithecus/quarry-sdk

# via GitHub Packages
pnpm add @justapithecus/quarry-sdk
```

## Quick Start

```typescript
import type { QuarryScript } from '@justapithecus/quarry-sdk'

interface MyJob {
  url: string
  maxItems: number
}

const script: QuarryScript<MyJob> = async (ctx) => {
  const { page, emit, job } = ctx

  await emit.info('Starting extraction', { url: job.url })

  await page.goto(job.url)

  const items = await page.$$eval('.product', (els) =>
    els.map((el) => ({
      name: el.querySelector('.name')?.textContent,
      price: el.querySelector('.price')?.textContent
    }))
  )

  for (const item of items.slice(0, job.maxItems)) {
    await emit.item({
      item_type: 'product',
      data: item
    })
  }

  // Take a screenshot
  const screenshot = await page.screenshot()
  await emit.artifact({
    name: 'final-page.png',
    content_type: 'image/png',
    data: screenshot
  })

  await emit.runComplete({
    summary: { itemCount: items.length }
  })
}

export default script
```

## API

### QuarryContext\<Job\>

The context object passed to your script.

| Property | Type | Description |
|----------|------|-------------|
| `job` | `Job` | Your job payload (immutable) |
| `run` | `RunMeta` | Run metadata (run_id, job_id, attempt) |
| `page` | `Page` | Puppeteer Page |
| `browser` | `Browser` | Puppeteer Browser |
| `browserContext` | `BrowserContext` | Puppeteer BrowserContext |
| `emit` | `EmitAPI` | Sole output mechanism |
| `storage` | `StorageAPI` | Sidecar file upload interface |

### EmitAPI

All methods are async and may block on backpressure.

| Method | Description |
|--------|-------------|
| `item({ item_type, data })` | Emit a structured record |
| `artifact({ name, content_type, data })` | Emit binary data, returns `ArtifactId` |
| `checkpoint({ checkpoint_id, note? })` | Mark progress |
| `enqueue({ target, params })` | Suggest additional work (advisory) |
| `rotateProxy({ reason? })` | Suggest proxy rotation (advisory) |
| `log({ level, message, fields? })` | Emit structured log |
| `debug/info/warn/error(message, fields?)` | Convenience log methods |
| `runError({ error_type, message, stack? })` | Signal fatal error |
| `runComplete({ summary? })` | Signal completion |

### StorageAPI

Sidecar file upload interface available via `ctx.storage`.

| Method | Description |
|--------|-------------|
| `put({ filename, content_type, data })` | Write a file to Hive-partitioned storage (max **8 MiB**, flat filename only) |

Files land under the run's `files/` prefix. Use `emit.artifact()` for larger
payloads or when chunk-level progress tracking is needed.

### Lifecycle Hooks

Export these from your script module for lifecycle control:

```typescript
import type { QuarryScriptModule } from '@justapithecus/quarry-sdk'

const module: QuarryScriptModule<MyJob> = {
  async default(ctx) {
    // main script
  },
  async beforeRun(ctx) {
    // setup
  },
  async afterRun(ctx) {
    // teardown on success
  },
  async onError(error, ctx) {
    // error handling
  },
  async cleanup(ctx) {
    // always runs (finally)
  }
}

export default module.default
export const { beforeRun, afterRun, onError, cleanup } = module
```
