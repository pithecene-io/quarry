# intentional-failure

Tests the error path by emitting one item and then throwing.
Verifies that `run_error` terminal state is correctly reported.

Expected exit code: **1**

Run:

```
quarry run \
  --script ./examples/intentional-failure/script.ts \
  --run-id run-fail-001 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data
```
