# Container Usage

This document covers running Quarry via container images.
For CLI flags and configuration, see `docs/guides/cli.md` and `docs/guides/configuration.md`.

---

## Images

Quarry ships two container images via GHCR:

| Image | Tag | Includes |
|-------|-----|----------|
| Full | `ghcr.io/pithecene-io/quarry:0.7.1` | Quarry CLI, Node.js, Puppeteer, system Chromium, fonts |
| Slim | `ghcr.io/pithecene-io/quarry:0.7.1-slim` | Quarry CLI, Node.js, Puppeteer (no browser) |

The **full** image is recommended for standalone usage. The **slim** image is
for environments where Chromium is provided externally (e.g., via
`--browser-ws-endpoint` pointing at a sidecar or shared browser service).

Both images set `QUARRY_NO_SANDBOX=1` (required for containerized Chromium)
and run as a non-root `quarry` user.

---

## Standalone `docker run`

```bash
docker run --rm \
  -v ./scripts:/work/scripts:ro \
  -v ./data:/work/data \
  ghcr.io/pithecene-io/quarry:0.7.1 \
  run \
    --script ./scripts/my-script.ts \
    --run-id "run-$(date +%s)" \
    --source my-source \
    --storage-backend fs \
    --storage-path ./data
```

---

## Docker Compose

```yaml
services:
  quarry:
    image: ghcr.io/pithecene-io/quarry:0.7.1
    volumes:
      - ./scripts:/work/scripts:ro
      - ./data:/work/data
    command:
      - run
      - --script=./scripts/my-script.ts
      - --run-id=scheduled-run
      - --source=my-source
      - --storage-backend=fs
      - --storage-path=./data
      - --policy=strict
```

---

## Docker Compose with S3 storage

```yaml
services:
  quarry:
    image: ghcr.io/pithecene-io/quarry:0.7.1
    volumes:
      - ./scripts:/work/scripts:ro
    environment:
      - AWS_ACCESS_KEY_ID
      - AWS_SECRET_ACCESS_KEY
    command:
      - run
      - --script=./scripts/my-script.ts
      - --run-id=scheduled-run
      - --source=my-source
      - --storage-backend=s3
      - --storage-path=my-bucket/quarry-data
      - --storage-region=us-east-1
```

---

## Docker Compose with Redis adapter

```yaml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  quarry:
    image: ghcr.io/pithecene-io/quarry:0.7.1
    depends_on:
      - redis
    volumes:
      - ./scripts:/work/scripts:ro
      - ./data:/work/data
    command:
      - run
      - --script=./scripts/my-script.ts
      - --run-id=scheduled-run
      - --source=my-source
      - --storage-backend=fs
      - --storage-path=./data
      - --adapter=redis
      - --adapter-url=redis://redis:6379
      - --adapter-channel=quarry:run_completed
```

A `RunCompletedEvent` JSON message is published to the `quarry:run_completed`
channel after each successful run. Subscribe with any Redis client:

```bash
redis-cli SUBSCRIBE quarry:run_completed
```

---

## Slim image with external browser

```yaml
services:
  chrome:
    image: chromedp/headless-shell:latest
    ports:
      - "9222:9222"

  quarry:
    image: ghcr.io/pithecene-io/quarry:0.7.1-slim
    depends_on:
      - chrome
    volumes:
      - ./scripts:/work/scripts:ro
      - ./data:/work/data
    command:
      - run
      - --script=./scripts/my-script.ts
      - --run-id=scheduled-run
      - --source=my-source
      - --storage-backend=fs
      - --storage-path=./data
      - --browser-ws-endpoint=ws://chrome:9222
```
