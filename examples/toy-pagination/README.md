# toy-pagination

Paginates across two local fixtures and emits item records.

Run:

```
quarry run \
  --script ./examples/toy-pagination/script.ts \
  --run-id run-002 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --job '{"source":"demo"}'
```
