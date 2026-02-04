# CLI Overview

This document is a user guide for the Quarry CLI.
The authoritative contract is `docs/contracts/CONTRACT_CLI.md`.

---

## Quick Start

Run a script (the only execution entrypoint):

```
quarry run \
  --script ./examples/demo.ts \
  --run-id run-001 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --job '{"source":"demo"}'
```

Inspect a run:

```
quarry inspect run run-001
```

List recent runs (no limit by default):

```
quarry list runs
```

Show JSON output:

```
quarry stats runs --format json
```

Interactive TUI for inspect/stats (opt-in only):

```
quarry inspect run run-001 --tui
```

---

## Command Groups

- `run`: execute work (the only command that runs jobs)
- `inspect`: deep view of a single entity (run, job, task, proxy, executor)
- `stats`: aggregated facts (runs, jobs, tasks, proxies, executors)
- `list`: thin enumerations (runs, jobs, pools, executors)
- `debug`: opt-in diagnostics (read-only by default)
- `version`: CLI and contract versions

---

## Output Formats

Commands support multiple formats:
- `table` for TTY output
- `json` for non-TTY output
- `yaml` for debugging and config inspection

The `--format` flag overrides defaults.

Supported formats:
- `table`: human-readable (default for TTY)
- `json`: canonical (default for non-TTY)
- `yaml`: debugging and config inspection

### Color

`--no-color` affects **table output only**. It does not disable TUI styling.

### TUI Mode

`--tui` is **opt-in** and **read-only**. It is supported only for:
- `inspect` subcommands
- `stats` subcommands

If `--tui` is provided for other commands, the CLI returns an error.

---

## Command Reference

### `run`

Executes a single script run. This is the only command that performs work.

Required flags:
- `--script <path>`
- `--run-id <id>`
- `--source <id>`
- `--storage-backend <fs|s3>`
- `--storage-path <path>`

Optional flags:
- `--attempt <n>` (default: 1)
- `--job-id <id>`
- `--parent-run-id <id>`
- `--job <json>` (inline JSON payload)
- `--job-json <path>` (load payload from file; mutually exclusive with `--job`)
- `--quiet`
- `--policy strict|buffered`
- `--flush-mode at_least_once|chunks_first|two_phase`
- `--buffer-events <n>`
- `--buffer-bytes <n>`
- `--proxy-config <path>` (JSON pool config)
- `--proxy-pool <name>`
- `--proxy-strategy round_robin|random|sticky`
- `--proxy-sticky-key <key>`
- `--proxy-domain <domain>` (when sticky scope = domain)
- `--proxy-origin <origin>` (when sticky scope = origin, format: scheme://host:port)

Advanced flags:
- `--executor <path>` (auto-resolved by default; override for troubleshooting)

Exit codes (per CONTRACT_RUN.md):
- `0`: success (run_complete)
- `1`: script error (run_error)
- `2`: executor crash
- `3`: policy failure

Example:

```
quarry run \
  --script ./examples/demo.ts \
  --run-id run-001 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --policy strict \
  --job '{"source":"demo"}'
```

Proxy selection example:

```
quarry run \
  --script ./examples/demo.ts \
  --run-id run-001 \
  --source demo \
  --storage-backend fs \
  --storage-path ./quarry-data \
  --proxy-config ./proxies.json \
  --proxy-pool residential_sticky \
  --proxy-domain example.com \
  --job '{"source":"demo"}'
```

### `inspect`

Deep view of a single entity.

Subcommands:
- `inspect run <run-id>`
- `inspect job <job-id>`
- `inspect task <task-id>`
- `inspect proxy <pool-name>`
- `inspect executor <executor-id>`

Examples:

```
quarry inspect run run-001
quarry inspect proxy default
quarry inspect run run-001 --tui
```

### `stats`

Aggregated facts derived from the read path.

Subcommands:
- `stats runs`
- `stats jobs`
- `stats tasks`
- `stats proxies`
- `stats executors`

Examples:

```
quarry stats runs
quarry stats proxies --format json
quarry stats runs --tui
```

### `list`

Thin enumerations (not inspect-level detail).

Subcommands:
- `list runs [--state running|failed|succeeded] [--limit <n>]`
- `list jobs`
- `list pools`
- `list executors`

Notes:
- `--limit` defaults to `0` (no limit).
- If output is large, the CLI may warn and suggest `--limit`.

Examples:

```
quarry list runs
quarry list runs --state running --limit 25
```

### `debug`

Opt-in diagnostics. Read-only by default.

Subcommands:
- `debug resolve proxy <pool> [--commit]`
- `debug ipc [--verbose]`

Examples:

```
quarry debug resolve proxy default --proxy-config ./proxies.json
quarry debug resolve proxy default --proxy-config ./proxies.json --commit
quarry debug ipc --verbose
```

### `version`

Reports the canonical project version (lockstep across all components).

Example:

```
quarry version
```
