# Downstream Integration Guide

This guide describes patterns for triggering downstream ETL or processing systems
after Quarry runs complete. It is **non-normative**; the authoritative contracts
remain in `docs/contracts/`.

---

## Overview

Quarry is an **extraction runtime**, not an end-to-end pipeline. After a run
completes, downstream systems typically need to:

1. Detect that new data is available
2. Read the emitted events and artifacts
3. Transform and load data into target systems

This guide describes two patterns for detecting run completion:

| Pattern | Default | Latency | Complexity | Guarantees |
|---------|---------|---------|------------|------------|
| Event-bus | ✅ Recommended | Low | Medium | At-least-once delivery |
| Polling | Fallback | Higher | Lower | Eventual consistency |

---

## Event-Bus Pattern (Recommended)

The event-bus pattern triggers downstream processing immediately when Quarry
publishes a run completion signal.

### Architecture

```
Quarry run completes
        ↓
Storage commit (Lode)
        ↓
Event published to bus (SNS, Kafka, etc.)
        ↓
Downstream consumer receives event
        ↓
Consumer reads from storage path
        ↓
Transform / Load
```

### Visibility Boundary

The event should be published **after** the storage commit is durable.
This ensures consumers can read the data referenced in the event.

Per `CONTRACT_RUN.md`, the visibility boundary is:

- `run_complete` or `run_error` is emitted
- All preceding events are persisted
- Storage sink is flushed and closed

Only publish to the event bus after this boundary is crossed.

### Event Payload

A minimal event payload should include:

```json
{
  "event_type": "run_completed",
  "run_id": "run-001",
  "source": "my-source",
  "category": "default",
  "day": "2026-02-04",
  "outcome": "success",
  "storage_path": "s3://bucket/source=my-source/category=default/day=2026-02-04/run_id=run-001",
  "timestamp": "2026-02-04T12:00:00Z"
}
```

Include enough information for consumers to:
- Filter events by source/category
- Locate data in storage
- Decide whether to process (based on outcome)

### Failure and Retry Considerations

1. **Publisher failures**: If publishing fails after storage commit, the run
   data exists but consumers won't be notified. Implement retry with backoff.

2. **Consumer failures**: Consumers should be idempotent. Processing the same
   run twice should produce the same result (or be a no-op).

3. **Out-of-order delivery**: Events may arrive out of order. Consumers should
   handle this gracefully (e.g., by checking timestamps or sequence numbers).

4. **Dead letter queue**: Route failed events to a DLQ for manual inspection.

### Example: AWS SNS + SQS

```bash
# After successful run, publish to SNS
aws sns publish \
  --topic-arn arn:aws:sns:us-east-1:123456789012:quarry-runs \
  --message '{"run_id":"run-001","source":"my-source","outcome":"success"}'
```

Downstream Lambda or ECS task subscribes to SQS queue backed by the SNS topic.

### Example: Kafka

```bash
# Publish to Kafka topic
echo '{"run_id":"run-001","source":"my-source","outcome":"success"}' | \
  kafka-console-producer --topic quarry-runs --bootstrap-server localhost:9092
```

Downstream consumer group processes messages with automatic offset management.

---

## Polling Pattern (Fallback)

The polling pattern periodically scans storage for new runs. Use this when:

- Event infrastructure is unavailable
- Simplicity is preferred over latency
- Batch processing windows are acceptable

### Architecture

```
Poller runs on schedule (cron, etc.)
        ↓
Scan storage for new/changed runs
        ↓
Compare against checkpoint (last processed)
        ↓
Process new runs
        ↓
Update checkpoint
```

### Checkpoint Management

Maintain a checkpoint to track the last processed state:

```json
{
  "last_processed_timestamp": "2026-02-04T11:00:00Z",
  "last_processed_run_ids": ["run-001", "run-002"]
}
```

The checkpoint should be **durable** (stored in a database or file) to survive
poller restarts.

### Idempotent Processing

Because polling may re-scan the same runs, processors must be idempotent:

- Check if a run has already been processed before starting
- Use upserts instead of inserts for target systems
- Track processed run IDs in the checkpoint

### Example: Filesystem Polling

```bash
#!/bin/bash
# Poll for new runs and process them

STORAGE_PATH="/var/quarry/data"
CHECKPOINT_FILE="/var/quarry/.checkpoint"

# Get last processed timestamp
LAST_TS=$(cat "$CHECKPOINT_FILE" 2>/dev/null || echo "1970-01-01T00:00:00Z")

# Find runs newer than checkpoint
find "$STORAGE_PATH" -name "event_type=run_complete" -newer "$CHECKPOINT_FILE" \
  -exec process_run.sh {} \;

# Update checkpoint
date -u +%Y-%m-%dT%H:%M:%SZ > "$CHECKPOINT_FILE"
```

### Example: S3 Polling with Listing

```python
import boto3
from datetime import datetime, timedelta

s3 = boto3.client('s3')
bucket = 'my-bucket'
prefix = 'source=my-source/'

# List objects modified in the last hour
response = s3.list_objects_v2(Bucket=bucket, Prefix=prefix)

for obj in response.get('Contents', []):
    if obj['LastModified'] > datetime.now() - timedelta(hours=1):
        # Process this run
        process_run(obj['Key'])
```

---

## Choosing a Pattern

| Factor | Event-Bus | Polling |
|--------|-----------|---------|
| Latency requirement | Real-time | Batch OK |
| Infrastructure | Event bus available | Minimal |
| Ordering needs | Strict | Relaxed |
| Retry complexity | Higher | Lower |
| Cost at scale | Per-event | Per-poll |

**Recommendation**: Start with event-bus for production workloads. Use polling
for development, testing, or when event infrastructure is unavailable.

---

## Anti-Patterns

### Watching for file changes directly

Do not use filesystem watchers (inotify, fswatch) on Quarry storage paths.
These tools may trigger before the storage commit is complete, leading to
partial reads.

### Assuming immediate availability

After `quarry run` exits, data may not be immediately visible to consumers
due to storage replication delays (especially with S3). Add a small delay
or use strong consistency guarantees.

### Processing without idempotency

Without idempotent processing, duplicate events or re-polled runs will
create duplicate records in target systems.

---

## Related Documents

- `CONTRACT_RUN.md` — Run lifecycle and terminal states
- `CONTRACT_LODE.md` — Storage interaction semantics
- `guides/run.md` — User-facing run lifecycle guide
