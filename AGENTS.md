# AGENTS.md — Quarry Guardrails

This file defines **non-negotiable guardrails** for working on Quarry.
It encodes *discipline and constraints*, not architecture.

---

## Core Principles

- Prefer **clarity over cleverness**
- Favor **explicit behavior** over implicit magic
- Keep abstractions **shallow and inspectable**
- Optimize for **debuggability and observability**, not elegance

---

## Scope Discipline

Agents must not:
- Invent new features unless explicitly requested
- Redesign core abstractions unprompted
- Introduce DSLs, frameworks, or configuration layers
- Optimize for scale or performance without evidence

If scope feels ambiguous or expanding, **pause and ask**.

---

## Change Discipline

- API changes are expensive; internal refactors are cheap
- Behavior changes must be observable (logs, events, metrics)
- Avoid silent fallbacks, hidden retries, or implicit recovery

---

## Structural Rules

- Small, single-purpose modules
- No premature generalization
- No “utility” dumping grounds
- Separate concerns explicitly:
  - execution vs orchestration
  - ingestion vs persistence
  - policy vs mechanism

## Proxy Discipline

- Proxy selection is runtime-owned; executor only applies resolved endpoints.
- No provider-specific proxy code in executor or SDK.

---

## TypeScript Rules (Strict)

Quarry is **TypeScript-first, ESM-only, modern by default**.

### Language & Modules
- `.ts` only
- ESM only (`import` / `export`)
- No `require`, `module`, `exports`
- Do **not** import `.js` files
- Extensionless imports where supported
- ES2022+ semantics

### Types
- Assume `strict: true`
- No `any` (use `unknown` + narrowing)
- Explicit return types on exported functions
- Prefer `type` aliases over `interface`
- Model domain concepts with types, not comments

### Imports & Exports
- Prefer named exports
- Default exports only for true entrypoints
- Import order:
  1. Node built-ins
  2. External deps
  3. Internal modules

### Files & Layout
- `kebab-case.ts`
- One primary concept per file
- Prefer small, composable modules

---

## Control Flow & Async

- Prefer pure functions
- Expression-oriented code
- Early returns over nesting
- No implicit mutation unless justified
- `async` / `await` only
- No floating promises
- Errors must be handled or propagated explicitly

---

## Errors

- Prefer explicit error types or result objects
- If throwing:
  - Throw `Error` subclasses only
  - Never throw strings

---

## Formatting & Comments

- Formatting is automated; do not hand-format
- Comments explain **why**, not **what**
- No commented-out code
- TSDoc only for exported APIs

---

## Barrel File Policy

- **Allowed**: One public entrypoint per package (e.g. `src/index.ts`)
- **Forbidden**: Internal barrel files
- Public entrypoints must import directly from source files

---

## Litmus Test

Before adding code, ask:

> Does this make the system easier to reason about for a future reader?

If not, reconsider.

## Agent Implementation Procedure

When given a task:

1. Read this file (`AGENTS.md`) in full.
2. Read only the files explicitly referenced by the task.
3. Do not infer architecture beyond what is visible in code.
4. Modify only files within the stated scope.
5. Do not introduce new dependencies unless explicitly requested.
6. Preserve existing public APIs unless the task explicitly permits changes.
7. Make all behavior changes observable.
8. Follow TypeScript and ESM rules strictly.
9. If an instruction is ambiguous, stop and ask before writing code.
10. If a change feels like scope expansion, stop and surface the concern.
11. Do not refactor unrelated code “for cleanliness.”
12. Output only the requested artifacts (code, diffs, or explanations).

