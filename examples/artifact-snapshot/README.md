# artifact-snapshot

Loads a local fixture and emits a screenshot artifact.

Run:

```
quarry run \
  --script ./examples/artifact-snapshot/script.ts \
  --run-id run-003 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --job '{"source":"demo"}'
```
