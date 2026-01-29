import type { QuarryContext } from './types/context'

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
 */
export type CleanupHook<Job = unknown> = (ctx: QuarryContext<Job>) => Promise<void> | void

/**
 * Optional lifecycle hooks that scripts can export.
 * All hooks are optional.
 */
export interface QuarryHooks<Job = unknown> {
  beforeRun?: BeforeRunHook<Job>
  afterRun?: AfterRunHook<Job>
  onError?: OnErrorHook<Job>
  cleanup?: CleanupHook<Job>
}

/**
 * A script module with optional hooks.
 * Scripts can export these alongside their default function.
 */
export interface QuarryScriptModule<Job = unknown> extends QuarryHooks<Job> {
  default: (ctx: QuarryContext<Job>) => Promise<void>
}
