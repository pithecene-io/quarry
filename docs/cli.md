# CLI Overview

This document describes the Quarry CLI at a user level.
The authoritative contract is `docs/contracts/CONTRACT_CLI.md`.

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
