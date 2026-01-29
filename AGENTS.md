# AGENTS.md — Quarry Development Guardrails

This file defines **style, discipline, and collaboration guardrails** for working on Quarry.
It intentionally avoids encoding detailed architecture or implementation specifics.

## General principles
- Prefer **clarity over cleverness**.
- Favor **explicit behavior** over implicit magic.
- Keep abstractions **shallow and inspectable**.
- Optimize for **debug-ability and observability**, not theoretical elegance.

## Code style & structure
- Keep modules small and single-purpose.
- Avoid premature generalization.
- Do not introduce new public interfaces without a concrete motivating use case.
- Separate concerns cleanly:
  - execution vs orchestration
  - ingestion vs persistence
  - policy vs mechanism

## Change discipline
- Treat API changes as expensive; treat internal refactors as cheap.
- When behavior changes, make it observable (logs, events, metrics).
- Avoid silent fallbacks and hidden retries.

## Agent-specific guidance
- Do not invent new features unless explicitly requested.
- If an instruction appears ambiguous, ask for clarification rather than guessing.
- If a proposed change feels like scope expansion, pause and surface the concern.

## Non-goals for agents
- Do not redesign core abstractions unprompted.
- Do not add DSLs, configuration languages, or framework-like layers.
- Do not optimize for scale or performance without evidence.

## Litmus test
Before adding code, ask:
> Does this make the system easier to reason about for a future reader?

If the answer is no, reconsider the change.

## TypeScript Code Style & Conventions (Quarry)

Quarry’s TypeScript codebase is **TypeScript-first, ESM-native, and modern by default**. Agents must follow these rules strictly.

### 1. Language & Module System

- **TypeScript only** (`.ts`, `.tsx` if ever needed)
- **ESM only**
  - Use `import` / `export`
  - Never use `require`, `module.exports`, or CommonJS patterns
- **Do not import `.js` files**
  - All imports must resolve to TypeScript sources
  - Use extensionless imports when supported by tooling
- Target **ES2022+** semantics (async/await, top-level await allowed where appropriate)

### 2. Strictness & Types

- Assume **`strict: true`**
- No `any`
  - Use `unknown` + narrowing if necessary
- Prefer **explicit return types** on exported functions
- Prefer **type aliases** over interfaces unless extension is intentional
- Model domain concepts with **rich types**, not comments

    // good
    export type JobId = string & { readonly __brand: "JobId" }

    // bad
    export type JobId = string

### 3. Imports & Exports

- Prefer **named exports**
- Avoid default exports except for:
  - Framework entrypoints
  - Single-purpose modules
- Group imports:
  1. Node built-ins
  2. External dependencies
  3. Internal modules

    import fs from "node:fs/promises"

    import { z } from "zod"

    import { TaskGraph } from "@/runtime/task-graph"

### 4. File & Directory Conventions

- `kebab-case.ts` for files
- One primary concept per file
- Avoid “utility dumping grounds”
- Prefer **small, composable modules**

    runtime/
      scheduler.ts
      task-graph.ts
      execution-context.ts

### 5. Functions & Control Flow

- Prefer **pure functions**
- Prefer **expression-oriented code**
- Early returns over deep nesting
- No implicit mutation unless performance-critical and justified

    // good
    export function isTerminal(state: State): boolean {
      return state.kind === "done" || state.kind === "failed"
    }

    // bad
    export function isTerminal(state: any) {
      if (state) {
        if (state.kind === "done") {
          return true
        } else {
          if (state.kind === "failed") {
            return true
          }
        }
      }
      return false
    }

### 6. Async & Concurrency

- Use `async` / `await`
- Never mix promise chains with `await`
- Errors must be explicitly handled or propagated
- No floating promises

    await queue.enqueue(task)
    queue.enqueue(task) // forbidden

### 7. Errors & Results

- Prefer explicit error types or result objects over exceptions for control flow
- If throwing:
  - Throw `Error` subclasses
  - Never throw raw strings

    throw new ExecutionError("task timed out", { taskId })

### 8. Formatting & Linting

- Formatting is automated (Biome / Prettier-equivalent)
- Agents must not hand-format
- Assume:
  - 2-space indentation
  - Trailing commas where valid
  - Semicolons optional but consistent

### 9. Comments & Documentation

- Comments explain **why**, not **what**
- Prefer types over comments
- No commented-out code
- Use TSDoc only for exported APIs

### 10. Forbidden Patterns

Agents must never introduce:

- JavaScript files (`.js`)
- `require`, `exports`, `module`
- `any`
- `// eslint-disable`
- Internal barrel files that hide dependency structure
- Framework magic without explicit wiring

### 11. Barrel File Policy

- **Allowed**: One public entrypoint per package (e.g., `sdk/src/index.ts`).
- **Forbidden**: Internal barrel files (e.g., `sdk/src/types/index.ts`).
- Public entrypoints must use **explicit imports** from source files,
  not re-export from internal barrels.

