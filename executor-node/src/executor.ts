/**
 * Executor: orchestrates script execution with proper lifecycle management.
 *
 * Responsibilities:
 * - Initialize Puppeteer browser/page
 * - Create context with EmitSink
 * - Execute script with lifecycle hooks
 * - Auto-emit terminal events (run_complete/run_error)
 * - Clean up resources
 *
 * @module
 */
import type { Writable } from 'node:stream'
import type { Browser, Page, BrowserContext, LaunchOptions } from 'puppeteer'
import puppeteer from 'puppeteer'
import {
  createContext,
  type RunMeta,
  type CreateContextOptions,
  type RunId,
  type JobId
} from '@justapithecus/quarry-sdk'
import { StdioSink } from './ipc/sink.js'
import { loadScript, type LoadedScript, ScriptLoadError } from './loader.js'

/**
 * Executor configuration passed from the runtime.
 */
export interface ExecutorConfig<Job = unknown> {
  /** Path to the script file */
  readonly scriptPath: string
  /** Job payload for the script */
  readonly job: Job
  /** Run metadata */
  readonly run: RunMeta
  /** Output stream for IPC frames (defaults to process.stdout) */
  readonly output?: Writable
  /** Puppeteer launch options */
  readonly puppeteerOptions?: LaunchOptions
}

/**
 * Execution outcome for reporting.
 */
export type ExecutionOutcome =
  | { readonly status: 'completed'; readonly summary?: Record<string, unknown> }
  | { readonly status: 'error'; readonly errorType: string; readonly message: string; readonly stack?: string }
  | { readonly status: 'crash'; readonly message: string }

/**
 * Result of executor run.
 */
export interface ExecutorResult {
  readonly outcome: ExecutionOutcome
  /** True if a terminal event was emitted by the executor */
  readonly terminalEmitted: boolean
}

/**
 * Parse and validate run metadata from raw input.
 */
export function parseRunMeta(input: unknown): RunMeta {
  if (input === null || typeof input !== 'object') {
    throw new Error('run metadata must be an object')
  }

  const obj = input as Record<string, unknown>

  if (typeof obj.run_id !== 'string' || obj.run_id === '') {
    throw new Error('run_id must be a non-empty string')
  }

  if (typeof obj.attempt !== 'number' || !Number.isInteger(obj.attempt) || obj.attempt < 1) {
    throw new Error('attempt must be a positive integer')
  }

  const run: RunMeta = {
    run_id: obj.run_id as RunId,
    attempt: obj.attempt,
    ...(typeof obj.job_id === 'string' && obj.job_id !== '' && { job_id: obj.job_id as JobId }),
    ...(typeof obj.parent_run_id === 'string' &&
      obj.parent_run_id !== '' && { parent_run_id: obj.parent_run_id as RunId })
  }

  return run
}

/**
 * Execute a script with full lifecycle management.
 *
 * This function:
 * 1. Loads the script
 * 2. Launches Puppeteer
 * 3. Creates the context
 * 4. Runs lifecycle hooks + script
 * 5. Emits terminal event if script doesn't
 * 6. Cleans up resources
 *
 * Lifecycle ordering:
 * - beforeRun → script → afterRun (success path)
 * - beforeRun → script → onError (failure path)
 * - cleanup always runs after terminal emission
 *
 * @remarks
 * **Cleanup hook contract**: The cleanup hook receives the same context but
 * MUST NOT emit protocol events. Terminal events (run_complete/run_error)
 * are emitted before cleanup runs. Any emit calls in cleanup will throw
 * TerminalEventError.
 *
 * @param config - Executor configuration
 * @returns Execution result
 */
export async function execute<Job = unknown>(config: ExecutorConfig<Job>): Promise<ExecutorResult> {
  const output = config.output ?? process.stdout
  const sink = new StdioSink(output)

  let browser: Browser | null = null
  let browserContext: BrowserContext | null = null
  let page: Page | null = null
  let script: LoadedScript<Job> | null = null
  let ctx: ReturnType<typeof createContext<Job>> | null = null
  let terminalEmitted = false

  try {
    // 1. Load script
    try {
      script = await loadScript<Job>(config.scriptPath)
    } catch (err: unknown) {
      if (err instanceof ScriptLoadError) {
        return {
          outcome: { status: 'crash', message: err.message },
          terminalEmitted: false
        }
      }
      throw err
    }

    // 2. Launch Puppeteer
    browser = await puppeteer.launch(config.puppeteerOptions)
    browserContext = await browser.createBrowserContext()
    page = await browserContext.newPage()

    // 3. Create context (single instance, reused throughout lifecycle)
    ctx = createContext<Job>({
      job: config.job,
      run: config.run,
      page,
      browser,
      browserContext,
      sink
    })

    // 4. Execute with lifecycle hooks
    try {
      // beforeRun hook
      if (script.hooks.beforeRun) {
        await script.hooks.beforeRun(ctx)
      }

      // Main script
      await script.script(ctx)

      // afterRun hook (only on success)
      if (script.hooks.afterRun) {
        await script.hooks.afterRun(ctx)
      }

      // Auto-emit run_complete if script didn't emit terminal
      if (!terminalEmitted) {
        try {
          await ctx.emit.runComplete()
          terminalEmitted = true
        } catch {
          // TerminalEventError means script already emitted terminal
          terminalEmitted = true
        }
      }

      return {
        outcome: { status: 'completed' },
        terminalEmitted
      }
    } catch (err) {
      // Script error
      const message = err instanceof Error ? err.message : String(err)
      const stack = err instanceof Error ? err.stack : undefined

      // onError hook
      if (script.hooks.onError) {
        try {
          await script.hooks.onError(err, ctx)
        } catch {
          // Swallow hook errors to avoid masking original
        }
      }

      // Auto-emit run_error if not already emitted
      if (!terminalEmitted) {
        try {
          await ctx.emit.runError({
            error_type: 'script_error',
            message,
            stack
          })
          terminalEmitted = true
        } catch {
          // TerminalEventError or sink failure
          terminalEmitted = true
        }
      }

      return {
        outcome: { status: 'error', errorType: 'script_error', message, stack },
        terminalEmitted
      }
    } finally {
      // cleanup hook (always runs, after terminal emission)
      // NOTE: cleanup MUST NOT emit events; they will throw TerminalEventError
      if (script?.hooks.cleanup && ctx) {
        try {
          await script.hooks.cleanup(ctx)
        } catch {
          // Swallow cleanup errors
        }
      }
    }
  } catch (err) {
    // Executor-level crash (Puppeteer launch failure, etc.)
    const message = err instanceof Error ? err.message : String(err)
    return {
      outcome: { status: 'crash', message },
      terminalEmitted: false
    }
  } finally {
    // Resource cleanup
    if (page) {
      try {
        await page.close()
      } catch {
        // Ignore close errors
      }
    }
    if (browserContext) {
      try {
        await browserContext.close()
      } catch {
        // Ignore close errors
      }
    }
    if (browser) {
      try {
        await browser.close()
      } catch {
        // Ignore close errors
      }
    }
  }
}
