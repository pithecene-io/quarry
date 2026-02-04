# ARCH_INDEX.md — Quarry Architecture Index

This file is a **navigation map** of Quarry’s subsystems.
It summarizes *what exists and where*, not how things are implemented.

Normative behavior is defined in `docs/contracts/CONTRACT_*.md`.

Maintenance rule:
Update this file only when a new subsystem boundary becomes real.
Do not update for internal refactors.

---

## Root

- `AGENTS.md` — development guardrails and agent constraints
- `PUBLIC_API.md` — user-facing API and setup guide
- `Taskfile.yaml` — task orchestration and developer workflows
- `biome.json` — formatting and linting configuration
- `pnpm-workspace.yaml` — monorepo layout

---

## docs/

### docs/guides/

User-facing guides (informational).

- `guides/overview.md` — user-facing system overview
- `guides/emit.md` — user-facing emit guide
- `guides/streaming.md` — user-facing streaming and artifact behavior
- `guides/run.md` — user-facing run lifecycle guide
- `guides/policy.md` — user-facing ingestion policy guide
- `guides/lode.md` — user-facing storage expectations
- `guides/cli.md` — user-facing CLI overview
- `guides/proxy.md` — user-facing proxy guide

### docs/contracts/

Normative contracts (authoritative). These documents define **system behavior**.

- `contracts/CONTRACT_IPC.md` — IPC framing, ordering, and transport semantics
- `contracts/CONTRACT_EMIT.md` — event and artifact emission rules
- `contracts/CONTRACT_RUN.md` — run lifecycle and terminal states
- `contracts/CONTRACT_POLICY.md` — policy-level constraints
- `contracts/CONTRACT_PROXY.md` — proxy configuration, selection, and executor application
- `contracts/CONTRACT_CLI.md` — CLI command topology and read-only guarantees
- `contracts/CONTRACT_LODE.md` — persistence and storage interaction
- `IMPLEMENTATION_PLAN.md` — staged implementation roadmap

Contracts are authoritative over code.

---

## executor-node/

Node-based executor implementation and IPC boundary.

### executor-node/src/

- `executor.ts` — core executor logic
- `bin/executor.ts` — CLI entrypoint
- `loader.ts` — runtime/bootstrap concerns
- `index.ts` — package entrypoint

#### executor-node/src/ipc/

IPC implementation details for the executor side.

- `frame.ts` — low-level frame encoding/decoding
- `sink.ts` — IPC sink abstraction
- `observing-sink.ts` — instrumentation/observation wrapper
- `index.ts` — local entrypoint

### executor-node/test/

Unit and IPC-focused tests for executor behavior.

---

## sdk/

Public SDK consumed by executors and integrations.
Defines **stable APIs** for emitting events, artifacts, and lifecycle signals.

### sdk/src/

- `emit.ts` — public emit API
- `emit-impl.ts` — internal implementation
- `context.ts` — execution context model
- `hooks.ts` — lifecycle hooks
- `index.ts` — SDK public entrypoint

#### sdk/src/types/

Shared domain types exposed by the SDK.

### sdk/test/

Contract, unit, ordering, and property-based tests validating SDK behavior.

---

## examples/

Example usage and integration references.
Non-normative.

---

## quarry/

Go module root. Contains runtime, CLI, and core types.

### Key files

- `types/version.go` — canonical version (all packages must match)
- `types/proxy.go` — proxy domain types
- `types/events.go` — event envelope and payload types
- `proxy/selector.go` — proxy pool selection/rotation
- `lode/sink.go` — Lode storage sink interface
- `cli/reader/` — CLI read-side data access layer

---

## Architectural Notes

- Contracts in `docs/` are **normative**
- `executor-node/` owns execution and IPC concerns
- `sdk/` is the public-facing API surface
- Generated artifacts (`dist/`, `node_modules/`) are non-authoritative
- IPC is treated as a hard boundary

This index is intentionally stable and low-detail.
