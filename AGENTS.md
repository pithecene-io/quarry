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
