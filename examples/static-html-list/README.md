# static-html-list

Parses a local HTML fixture and emits item records.

Run:

```
quarry run \
  --script ./examples/static-html-list/script.ts \
  --run-id run-001 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --job '{"source":"demo"}'
```
