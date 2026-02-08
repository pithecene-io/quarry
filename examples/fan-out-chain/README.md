# fan-out-chain

Demonstrates fan-out derived work execution. The root script parses a
product listing and enqueues a detail script per product. Each detail
script extracts product data and emits an item.

**Note:** Child runs inherit the root run's `--source` and `--category`
by default. Per-child overrides are supported via
`emit.enqueue({ source, category })`.

**Important:** If child scripts use `ctx.storage.put()`, the root run
must have storage properly configured (`--storage-backend`,
`--storage-path`). A missing FileWriter will fail the child run
immediately rather than silently discarding data.

Run:

```
quarry run \
  --script ./examples/fan-out-chain/listing.ts \
  --run-id fanout-001 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --job '{}' \
  --depth 1 \
  --max-runs 10 \
  --parallel 3
```
