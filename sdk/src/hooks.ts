import type { QuarryContext, RunMeta } from './types/context'

/**
 * Result of a `prepare` hook invocation.
 *
 * - `{ action: 'continue' }` — proceed with the original job.
 * - `{ action: 'continue', job }` — proceed with a transformed job.
 * - `{ action: 'skip', reason? }` — skip the run entirely.
 */
export type PrepareResult<Job = unknown> =
  | { action: 'continue'; job?: Job }
  | { action: 'skip'; reason?: string }

/**
 * Hook called before browser launch to inspect or transform the job payload.
 *
 * Returning `{ action: 'skip' }` short-circuits the run: the executor emits
 * `run_complete` with `{ skipped: true }` and returns without launching a browser.
 * No other hooks (beforeRun, afterRun, onError, beforeTerminal, cleanup) are
 * called on the skip path — no browser or context exists.
 *
 * Returning `{ action: 'continue', job }` replaces the job for all downstream hooks.
 *
 * Must return a valid `PrepareResult`. Non-object or missing `action` returns
 * are treated as a crash with a diagnostic message.
 */
export type PrepareHook<Job = unknown> = (
  job: Job,
  run: RunMeta
) => Promise<PrepareResult<Job>> | PrepareResult<Job>

/**
 * Signal passed to `beforeTerminal` describing how the script finished.
 */
export type TerminalSignal =
  | { outcome: 'completed' }
  | { outcome: 'error'; error: unknown }

/**
 * Hook called after script execution but before the executor emits the
 * terminal event. Emit is still open — the hook may emit items or logs.
 *
 * If the hook throws, the error is swallowed (consistent with onError/cleanup).
 */
export type BeforeTerminalHook<Job = unknown> = (
  signal: TerminalSignal,
  ctx: QuarryContext<Job>
) => Promise<void> | void

/**
 * Hook called before the main script function executes.
 * Can be used for setup, logging, or validation.
 */
export type BeforeRunHook<Job = unknown> = (ctx: QuarryContext<Job>) => Promise<void> | void

/**
 * Hook called after the main script function completes successfully.
 * Not called if the script throws or emits run_error.
 */
export type AfterRunHook<Job = unknown> = (ctx: QuarryContext<Job>) => Promise<void> | void

/**
 * Hook called when an error occurs during script execution.
 * Receives the error object and context.
 * Can be used for cleanup or error reporting.
 */
export type OnErrorHook<Job = unknown> = (
  error: unknown,
  ctx: QuarryContext<Job>
) => Promise<void> | void

/**
 * Hook called during cleanup, regardless of success or failure.
 * Similar to a finally block.
 *
 * Not called when `prepare` returns `{ action: 'skip' }` — the run
 * short-circuits before browser acquisition, so no context exists.
 */
export type CleanupHook<Job = unknown> = (ctx: QuarryContext<Job>) => Promise<void> | void

/**
 * Optional lifecycle hooks that scripts can export.
 * All hooks are optional.
 */
export interface QuarryHooks<Job = unknown> {
  prepare?: PrepareHook<Job>
  beforeRun?: BeforeRunHook<Job>
  afterRun?: AfterRunHook<Job>
  onError?: OnErrorHook<Job>
  beforeTerminal?: BeforeTerminalHook<Job>
  cleanup?: CleanupHook<Job>
}

/**
 * A script module with optional hooks.
 * Scripts can export these alongside their default function.
 */
export interface QuarryScriptModule<Job = unknown> extends QuarryHooks<Job> {
  default: (ctx: QuarryContext<Job>) => Promise<void>
}
