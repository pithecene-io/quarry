# Quarry

**A CLI-first web extraction runtime for browser-driven crawling and durable ingestion**

Quarry is a web extraction runtime for imperative, browser-backed scraping workflows. It is designed for adversarial sites, bespoke extraction logic, and long-lived ETL pipelines where correctness, observability, and durability matter more than convenience abstractions.

Quarry executes user-authored Puppeteer scripts under a strict runtime contract, streams observations incrementally, and hands off persistence to an external substrate (typically Lode). It is intentionally *not* a crawler framework, workflow engine, or low-code platform.

---

## What Quarry Is

Quarry is:

- A **runtime**, not a framework
- **CLI-first**, not embedded
- Designed for **imperative Puppeteer scripts**
- Explicit about **ordering, backpressure, and failure**
- Agnostic to storage, retries, scheduling, and downstream processing

Quarry’s responsibility ends at **observing and emitting what happened**.

---

## What Quarry Is Not

Quarry is **not**:

- A crawling DSL
- A workflow orchestrator
- A distributed task scheduler
- A SaaS scraper or low-code pipeline
- A storage engine

Those concerns are intentionally left to other layers.

---

## Conceptual Model

Quarry enforces a clean boundary between extraction logic and ingestion mechanics:

```
User Script (Puppeteer, imperative)
        ↓
emit.*  (stable event contract)
        ↓
Quarry Runtime
        ↓
Ingestion Policy (strict, buffered, etc.)
        ↓
Persistence Substrate (e.g. Lode)
```

Scripts **emit observations**.  
Policies decide how those observations are handled.  
Persistence decides what survives.

---

## Using Quarry in a Larger Pipeline

Quarry is designed to be composed **around**, not extended **from**.

A typical pipeline might look like:

```bash
# Extract
quarry run \
  --script streeteasy.ts \
  --job jobs/streeteasy.json \
  --dataset nyc-rent/streeteasy/raw \
  --policy buffered

# Transform (outside Quarry)
nyc-rent-transform \
  --input nyc-rent/streeteasy/raw \
  --output nyc-rent/streeteasy/normalized

# Index / analyze (outside Quarry)
nyc-rent-index \
  --input nyc-rent/streeteasy/normalized
```

Quarry owns **only** the extraction step.

---

## Quarry Scripts

Quarry scripts are **freestanding programs**, not libraries.

They should:
- Accept all inputs via the job payload
- Use real Puppeteer objects (`page`, `browser`)
- Emit all outputs via `emit.*`
- Avoid shared global state
- Remain agnostic to durability and retries

### Example

```ts
import type { QuarryScript } from 'quarry/sdk'

export default async function run(ctx) {
  await ctx.page.goto(ctx.job.url)

  const listings = await ctx.page.evaluate(() => {
    // scrape DOM
  })

  for (const listing of listings) {
    await ctx.emit.item({
      item_type: 'listing',
      data: listing
    })
  }

  await ctx.emit.runComplete()
}
```

Scripts are imperative, explicit, and boring by design.

---

## Emission Model

Scripts do not return values.

All output flows through `emit.*`:

- `emit.item(...)` — structured records
- `emit.artifact(...)` — binary artifacts (screenshots, files)
- `emit.checkpoint(...)` — progress markers
- `emit.log(...)` — structured logs
- `emit.run_error(...)` — terminal failure
- `emit.run_complete(...)` — successful completion

Emission is:
- **ordered**
- **append-only**
- **backpressure-aware**
- **observable**

---

## Backpressure and Policies

Quarry does not hide backpressure.

If downstream ingestion is slow, `emit.*` **blocks**.

Ingestion behavior is controlled via **policies**:

- **Strict** — synchronous writes, no loss
- **Buffered** — bounded buffering, explicit drops allowed

Scripts are policy-agnostic.

---

## Durability and Lode

Quarry does not persist data itself.

It is commonly paired with **Lode**, which provides:
- append-only object storage
- partitioned datasets
- recovery and replay
- lineage visibility

Quarry guarantees consistent emission semantics so that Lode can remain simple.

---

## Design Principles

- **Contracts before code**
- **No silent loss**
- **No hidden retries**
- **No framework magic**
- **Explicit failure boundaries**

If a behavior matters, it is documented and observable.

---

## Documentation

User-facing guides live in `docs/guides/` for a deeper dive into concepts,
configuration, and usage.

---

## Status

Quarry is under active development.

- Contracts are frozen
- SDK surface is stabilizing
- Executor and runtime are evolving

Breaking changes are gated by contract versioning.

---

## License

Apache 2.0
