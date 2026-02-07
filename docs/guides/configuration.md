# Configuration Reference

This document is a consolidated reference for all Quarry configuration surfaces.
For command details, see `docs/guides/cli.md`. For proxy setup, see `docs/guides/proxy.md`.

---

## Configuration Layers

Quarry configuration lives in three distinct layers:

| Layer | Where | What it controls |
|-------|-------|------------------|
| **CLI flags** | `quarry run --flag` | Run execution, storage, policy, proxy selection |
| **Config files** | JSON files on disk | Proxy pool definitions |
| **Environment variables** | Process environment | Executor/browser behavior |

These layers are intentionally separate. CLI flags control the Go runtime
(orchestration, storage, policy). Environment variables control the Node
executor (browser hardening, plugin behavior). Config files define static
resources referenced by flags.

---

## CLI Flags

All flags are passed to `quarry run`. See `docs/guides/cli.md` for full
command reference.

### Required

| Flag | Type | Purpose |
|------|------|---------|
| `--script` | path | Script to execute |
| `--run-id` | string | Unique run identifier |
| `--source` | string | Source identifier (Lode partition key) |
| `--storage-backend` | `fs` or `s3` | Storage backend |
| `--storage-path` | string | `fs`: directory path; `s3`: `bucket/prefix` |

### Run Metadata

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--attempt` | int | `1` | Attempt number (1 = initial, >1 = retry) |
| `--job-id` | string | — | Optional job identifier |
| `--parent-run-id` | string | — | Required when `--attempt > 1` |
| `--job` | JSON string | `{}` | Inline job payload (must be a JSON object) |
| `--job-json` | path | — | Job payload from file (mutually exclusive with `--job`) |
| `--category` | string | `"default"` | Category identifier (Lode partition key) |

### Storage

| Flag | Type | Purpose |
|------|------|---------|
| `--storage-backend` | `fs` or `s3` | Backend type |
| `--storage-path` | string | `fs`: local directory; `s3`: `bucket/optional-prefix` |
| `--storage-region` | string | AWS region (S3 only; uses default credential chain) |
| `--storage-endpoint` | string | Custom S3 endpoint URL (for R2, MinIO, etc.) |
| `--storage-s3-path-style` | bool | Force path-style addressing (required by R2, MinIO) |

### Policy

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--policy` | `strict` or `buffered` | `strict` | Ingestion policy |
| `--flush-mode` | `at_least_once`, `chunks_first`, `two_phase` | `at_least_once` | Buffered flush semantics |
| `--buffer-events` | int | `0` | Max events to buffer (buffered policy) |
| `--buffer-bytes` | int | `0` | Max buffer bytes (buffered policy) |

Buffered policy requires at least one of `--buffer-events` or `--buffer-bytes` to be set (> 0).

### Proxy

| Flag | Type | Purpose |
|------|------|---------|
| `--proxy-config` | path | Path to JSON proxy pools config |
| `--proxy-pool` | string | Pool name to select from (requires `--proxy-config`) |
| `--proxy-strategy` | `round_robin`, `random`, `sticky` | Override pool's default strategy |
| `--proxy-sticky-key` | string | Explicit sticky key (overrides derivation) |
| `--proxy-domain` | string | Domain for sticky derivation (scope=domain) |
| `--proxy-origin` | string | Origin for sticky derivation (scope=origin, format: `scheme://host:port`) |

See `docs/guides/proxy.md` for pool configuration format and selection behavior.

### Output

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--format` | `table`, `json`, `yaml` | auto (TTY=table, pipe=json) | Output format |
| `--no-color` | bool | `false` | Disable color in table output |
| `--tui` | bool | `false` | Interactive TUI (inspect/stats only) |
| `--quiet` | bool | `false` | Suppress run result output |

### Advanced

| Flag | Type | Purpose |
|------|------|---------|
| `--executor` | path | Override executor binary (auto-resolved by default) |

---

## Environment Variables

Environment variables control executor and browser behavior. They are read
by the Node executor process, not the Go runtime.

| Variable | Default | Purpose |
|----------|---------|---------|
| `QUARRY_STEALTH` | enabled | Puppeteer stealth plugin (evades bot detection). Set to `0` to disable. |
| `QUARRY_ADBLOCKER` | disabled | Puppeteer adblocker plugin. Set to `1` to enable. |
| `QUARRY_NO_SANDBOX` | disabled | Disable Chromium sandbox (required in containers/CI). Set to `1` to enable. |

### Usage

```bash
# Container/CI: disable sandbox
QUARRY_NO_SANDBOX=1 quarry run ...

# Disable stealth (e.g., for debugging)
QUARRY_STEALTH=0 quarry run ...

# Enable adblocker
QUARRY_ADBLOCKER=1 quarry run ...

# Combine
QUARRY_NO_SANDBOX=1 QUARRY_ADBLOCKER=1 quarry run ...
```

### Why environment variables?

These control Puppeteer browser behavior inside the executor subprocess.
They are deployment-level knobs (container vs local, detection posture)
rather than per-run configuration, so they live outside the run protocol.

---

## Config Files

### Proxy Pools

Proxy pool definitions are static JSON files referenced via `--proxy-config`.

```json
[
  {
    "name": "residential",
    "strategy": "round_robin",
    "endpoints": [
      { "protocol": "http", "host": "proxy1.example.com", "port": 8080 },
      { "protocol": "http", "host": "proxy2.example.com", "port": 8080 }
    ]
  },
  {
    "name": "sticky-pool",
    "strategy": "sticky",
    "sticky": { "scope": "domain", "ttl_ms": 3600000 },
    "endpoints": [
      {
        "protocol": "http",
        "host": "proxy3.example.com",
        "port": 8080,
        "username": "user",
        "password": "pass"
      }
    ]
  }
]
```

See `docs/guides/proxy.md` for the full pool configuration schema.

### Job Payload

Job payloads can be provided inline (`--job`) or from a file (`--job-json`).
The payload must be a JSON object (not array, string, number, or null).

```bash
# Inline
quarry run --job '{"url": "https://example.com"}' ...

# From file
quarry run --job-json ./job.json ...
```

---

## AWS Credentials (S3 Backend)

When using `--storage-backend s3`, Quarry uses the AWS SDK v2 default
credential chain. No explicit credential flags exist.

Resolution order:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM instance role (EC2, ECS, Lambda)

Set `--storage-region` if the bucket is not in your SDK's default region.

For S3-compatible providers (R2, MinIO), set credentials via environment
variables and use `--storage-endpoint` to point at the provider's endpoint.

---

## Configuration Flow

```
quarry run (Go runtime)
  ├── CLI flags → run metadata, storage, policy, proxy selection
  ├── Config files → proxy pool definitions (--proxy-config)
  └── Spawns executor subprocess
        ├── stdin JSON → run_id, attempt, job, proxy endpoint
        └── env vars → QUARRY_STEALTH, QUARRY_ADBLOCKER, QUARRY_NO_SANDBOX
              └── Controls Puppeteer browser behavior
```

The Go runtime resolves all configuration, selects a proxy endpoint (if
configured), and passes a fully resolved run context to the executor via
stdin. The executor reads only its own environment variables for browser
hardening — it does not access CLI flags or config files.

---

## Common Recipes

### Minimal local run

```bash
quarry run \
  --script ./my-script.ts \
  --run-id run-001 \
  --source my-source \
  --storage-backend fs \
  --storage-path ./data
```

### S3 with buffered policy

```bash
quarry run \
  --script ./my-script.ts \
  --run-id run-001 \
  --source my-source \
  --storage-backend s3 \
  --storage-path my-bucket/quarry \
  --storage-region us-east-1 \
  --policy buffered \
  --buffer-events 500 \
  --buffer-bytes 10485760
```

### S3-compatible provider (Cloudflare R2)

```bash
quarry run \
  --script ./my-script.ts \
  --run-id run-001 \
  --source my-source \
  --storage-backend s3 \
  --storage-path my-bucket/quarry \
  --storage-endpoint https://ACCOUNT_ID.r2.cloudflarestorage.com \
  --storage-s3-path-style
```

Credentials via environment variables:
```bash
export AWS_ACCESS_KEY_ID=<r2-access-key>
export AWS_SECRET_ACCESS_KEY=<r2-secret-key>
```

### Proxy with stealth in CI

```bash
QUARRY_NO_SANDBOX=1 quarry run \
  --script ./my-script.ts \
  --run-id run-001 \
  --source my-source \
  --storage-backend fs \
  --storage-path ./data \
  --proxy-config ./proxies.json \
  --proxy-pool residential \
  --proxy-domain example.com
```

### Debug proxy resolution (dry run)

```bash
quarry debug resolve proxy residential \
  --proxy-config ./proxies.json \
  --strategy round_robin
```
