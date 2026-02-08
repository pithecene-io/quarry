# Configuration Reference

This document is a consolidated reference for all Quarry configuration surfaces.
For command details, see `docs/guides/cli.md`. For proxy setup, see `docs/guides/proxy.md`.

---

## Configuration Layers

Quarry configuration lives in four distinct layers:

| Layer | Where | What it controls |
|-------|-------|------------------|
| **CLI flags** | `quarry run --flag` | Run execution, storage, policy, proxy selection |
| **YAML config file** | `--config quarry.yaml` | Project-level defaults for all run flags |
| **JSON config files** | `--proxy-config` (deprecated) | Proxy pool definitions |
| **Environment variables** | Process environment | Executor/browser behavior |

**Precedence:** CLI flags > YAML config file > flag defaults.

CLI flags control the Go runtime (orchestration, storage, policy).
Environment variables control the Node executor (browser hardening, plugin
behavior). The YAML config file provides reusable project-level defaults
to reduce flag repetition across invocations.

---

## CLI Flags

All flags are passed to `quarry run`. See `docs/guides/cli.md` for full
command reference.

### Config File

| Flag | Type | Purpose |
|------|------|---------|
| `--config` | path | Path to YAML config file with project-level defaults |

See [YAML Config File](#yaml-config-file) below for schema and examples.

### Required

| Flag | Type | Purpose |
|------|------|---------|
| `--script` | path | Script to execute |
| `--run-id` | string | Unique run identifier |
| `--source` | string | Source identifier (Lode partition key) |
| `--storage-backend` | `fs` or `s3` | Storage backend |
| `--storage-path` | string | `fs`: directory path; `s3`: `bucket/prefix` |

> **Note:** `--source`, `--storage-backend`, and `--storage-path` can be
> provided via `--config` file instead of CLI flags. They are required at
> runtime but do not need to appear on the CLI if set in the config.

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
| `--storage-dataset` | string | Lode dataset ID (default: `"quarry"`) |
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
| `--proxy-config` | path | Path to JSON proxy pools config (**deprecated**: use `proxies:` in YAML config) |
| `--proxy-pool` | string | Pool name to select from |
| `--proxy-strategy` | `round_robin`, `random`, `sticky` | Override pool's default strategy |
| `--proxy-sticky-key` | string | Explicit sticky key (overrides derivation) |
| `--proxy-domain` | string | Domain for sticky derivation (scope=domain) |
| `--proxy-origin` | string | Origin for sticky derivation (scope=origin, format: `scheme://host:port`) |

See `docs/guides/proxy.md` for pool configuration format and selection behavior.

### Adapter (Event-Bus Notification)

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--adapter` | `webhook`, `redis` | | Adapter type |
| `--adapter-url` | string | | Endpoint URL (required when `--adapter` set) |
| `--adapter-header` | string (repeatable) | | Custom header as `key=value` (webhook only) |
| `--adapter-channel` | string | `quarry:run_completed` | Pub/sub channel name (redis only) |
| `--adapter-timeout` | duration | `10s` (webhook) / `5s` (redis) | Per-request/publish timeout |
| `--adapter-retries` | int | `3` | Retry attempts |

See `docs/guides/integration.md` for adapter usage patterns.

### Output

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--format` | `table`, `json`, `yaml` | auto (TTY=table, pipe=json) | Output format |
| `--no-color` | bool | `false` | Disable color in table output |
| `--tui` | bool | `false` | Interactive TUI (inspect/stats only) |
| `--quiet` | bool | `false` | Suppress run result output |

### Advanced (development only)

| Flag | Type | Purpose |
|------|------|---------|
| `--executor` | path | Override executor binary (auto-resolved in bundled binary; only needed for local development builds) |

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

## YAML Config File

The `--config` flag loads a YAML config file providing project-level defaults
for `quarry run`. This reduces repetition when the same source, storage,
policy, and proxy settings are used across multiple invocations.

### Quick Start

```yaml
# quarry.yaml
source: my-source
storage:
  backend: fs
  path: ./quarry-data
```

```bash
quarry run --config quarry.yaml --script ./script.ts --run-id run-001
```

### Full Schema

```yaml
# quarry.yaml — project defaults for quarry run
# All values are overridden by explicit CLI flags.

source: my-source
category: default

# Executor path override (development only).
# The bundled binary auto-resolves the executor; only needed for local dev builds.
# executor: ./executor-node/dist/bin/executor.js

storage:
  dataset: quarry
  backend: s3
  path: my-bucket/quarry-data
  region: us-east-1
  endpoint: https://ACCOUNT_ID.r2.cloudflarestorage.com
  s3_path_style: true

policy:
  name: buffered
  flush_mode: at_least_once
  buffer_events: 1000
  buffer_bytes: 10485760

proxies:
  iproyal_nyc:
    strategy: round_robin
    endpoints:
      - protocol: https
        host: geo.iproyal.com
        port: 12321
        username: ${IPROYAL_USER}
        password: ${IPROYAL_PASS}

  residential_sticky:
    strategy: sticky
    sticky:
      scope: domain
      ttl_ms: 3600000
    endpoints:
      - protocol: http
        host: proxy.example.com
        port: 8080

proxy:
  pool: iproyal_nyc
  strategy: round_robin

adapter:
  type: webhook
  url: https://hooks.example.com/quarry
  headers:
    Authorization: Bearer ${WEBHOOK_TOKEN}
  timeout: 10s
  retries: 3
```

### Environment Variable Expansion

The config file supports `${VAR}` and `${VAR:-default}` syntax. Expansion
is applied to the raw YAML text before parsing.

- `${VAR}` — expands to the value of `VAR`, or empty string if unset
- `${VAR:-default}` — expands to `default` if `VAR` is unset or empty

Unset variables without defaults are not errors. Required secrets will
fail at downstream validation (e.g., proxy endpoint auth pair validation).

### Proxy Pools in Config

Proxy pools are defined inline under `proxies:`, keyed by pool name. This
replaces the deprecated `--proxy-config` JSON file. Using both
`--proxy-config` and config `proxies:` in the same invocation is an error.

### No Auto-Discovery

Config files are loaded only via explicit `--config <path>`. There is no
implicit search for `quarry.yaml` in the working directory.

---

## Config Files (Legacy)

### Proxy Pools (deprecated)

Proxy pool definitions were previously static JSON files referenced via
`--proxy-config`. This method still works but emits a deprecation warning.
Prefer defining pools in the YAML config file under `proxies:`.

```json
[
  {
    "name": "residential",
    "strategy": "round_robin",
    "endpoints": [
      { "protocol": "http", "host": "proxy1.example.com", "port": 8080 },
      { "protocol": "http", "host": "proxy2.example.com", "port": 8080 }
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
  ├── CLI flags (highest precedence)
  ├── YAML config file (--config, project defaults)
  ├── Flag defaults (lowest precedence)
  └── Spawns executor subprocess
        ├── stdin JSON → run_id, attempt, job, proxy endpoint
        └── env vars → QUARRY_STEALTH, QUARRY_ADBLOCKER, QUARRY_NO_SANDBOX
              └── Controls Puppeteer browser behavior
```

The Go runtime merges CLI flags and config file values (CLI wins), resolves
all configuration, selects a proxy endpoint (if configured), and passes a
fully resolved run context to the executor via stdin. The executor reads
only its own environment variables for browser hardening — it does not
access CLI flags or config files.

---

## Common Recipes

### Minimal local run (with config)

```yaml
# quarry.yaml
source: my-source
storage:
  backend: fs
  path: ./data
```

```bash
quarry run --config quarry.yaml --script ./my-script.ts --run-id run-001
```

### Minimal local run (flags only)

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
