# fan-out-chain

Demonstrates fan-out derived work execution. The root script parses a
product listing and enqueues a detail script per product. Each detail
script extracts product data and emits an item.

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
