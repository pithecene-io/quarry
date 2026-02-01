/**
 * Executor: orchestrates script execution with proper lifecycle management.
 *
 * Responsibilities:
 * - Initialize Puppeteer browser/page
 * - Create context with EmitSink
 * - Execute script with lifecycle hooks
 * - Auto-emit terminal events (run_complete/run_error) if not already emitted
 * - Determine outcome based on actually written terminal events
 * - Clean up resources
 *
 * Outcome determination (in precedence order):
 * 1. Sink failure at any point → crash
 * 2. Terminal event successfully written → match that event
 * 3. Script threw without terminal → error (auto-emit run_error)
 * 4. Script completed without terminal → completed (auto-emit run_complete)
 *
 * @module
 */
import type { Writable } from 'node:stream'
import {
  type CreateContextOptions,
  createContext,
  type JobId,
  type ProxyEndpoint,
  type RunId,
  type RunMeta,
  TerminalEventError
} from '@justapithecus/quarry-sdk'
import type { Browser, BrowserContext, LaunchOptions, Page } from 'puppeteer'
import puppeteer from 'puppeteer'
import { ObservingSink, SinkAlreadyFailedError, type SinkState } from './ipc/observing-sink.js'
import { StdioSink } from './ipc/sink.js'
import { type LoadedScript, loadScript, ScriptLoadError } from './loader.js'

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
  /** Optional resolved proxy endpoint per CONTRACT_PROXY.md */
  readonly proxy?: ProxyEndpoint
}

/**
 * Execution outcome for reporting.
 */
export type ExecutionOutcome =
  | { readonly status: 'completed'; readonly summary?: Record<string, unknown> }
  | {
      readonly status: 'error'
      readonly errorType: string
      readonly message: string
      readonly stack?: string
    }
  | { readonly status: 'crash'; readonly message: string }

/**
 * Result of executor run.
 */
export interface ExecutorResult {
  readonly outcome: ExecutionOutcome
  /** True if a terminal event was successfully written to the sink (by script or executor) */
  readonly terminalEmitted: boolean
}

/**
 * Parse and validate run metadata from raw input.
 *
 * Lineage rules per CONTRACT_RUN.md (strictly enforced):
 * - attempt must be >= 1
 * - If attempt === 1, parent_run_id must be absent (initial run)
 * - If attempt > 1, parent_run_id must be present (retry run)
 *
 * @param input - Raw metadata object
 * @returns Validated RunMeta
 * @throws Error if required fields are missing, invalid, or lineage rules are violated
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

  const hasParentRunId = typeof obj.parent_run_id === 'string' && obj.parent_run_id !== ''

  // Strict lineage validation per CONTRACT_RUN.md
  if (obj.attempt === 1 && hasParentRunId) {
    throw new Error('initial run (attempt=1) must not have parent_run_id')
  }
  if (obj.attempt > 1 && !hasParentRunId) {
    throw new Error(`retry run (attempt=${obj.attempt}) must have parent_run_id`)
  }

  // Build RunMeta
  const run: RunMeta = {
    run_id: obj.run_id as RunId,
    attempt: obj.attempt,
    ...(typeof obj.job_id === 'string' && obj.job_id !== '' && { job_id: obj.job_id as JobId }),
    ...(hasParentRunId && { parent_run_id: obj.parent_run_id as RunId })
  }

  return run
}

/**
 * Build Puppeteer launch options with proxy configuration.
 * Per CONTRACT_PROXY.md: Apply proxy host/port/protocol at browser launch.
 *
 * @param baseOptions - Base Puppeteer launch options
 * @param proxy - Optional proxy endpoint
 * @returns Merged launch options with proxy args
 */
function buildPuppeteerLaunchOptions(
  baseOptions: LaunchOptions | undefined,
  proxy: ProxyEndpoint | undefined
): LaunchOptions {
  if (!proxy) {
    return baseOptions ?? {}
  }

  // Build proxy URL (without credentials - those are applied via page.authenticate)
  const proxyUrl = `${proxy.protocol}://${proxy.host}:${proxy.port}`

  // Merge args, preserving existing args from baseOptions
  const existingArgs = baseOptions?.args ?? []
  const proxyArgs = [`--proxy-server=${proxyUrl}`]

  return {
    ...baseOptions,
    args: [...existingArgs, ...proxyArgs]
  }
}

/**
 * Determine if an error is a sink failure (vs expected TerminalEventError).
 * SinkAlreadyFailedError is also a sink failure - it wraps the original cause.
 */
function isSinkFailure(err: unknown): boolean {
  if (err instanceof TerminalEventError) {
    return false
  }
  if (err instanceof SinkAlreadyFailedError) {
    return true
  }
  // Any other error from emit is a sink failure
  return true
}

/**
 * Execute a script with full lifecycle management.
 *
 * This function:
 * 1. Loads the script
 * 2. Launches Puppeteer
 * 3. Creates the context with ObservingSink
 * 4. Runs lifecycle hooks + script
 * 5. Determines outcome based on sink state and script behavior
 * 6. Emits terminal event if script didn't and sink is healthy
 * 7. Cleans up resources
 *
 * Outcome determination (in precedence order):
 * 1. Sink failure at any point → crash
 * 2. Terminal event successfully written → match that event
 * 3. Script threw without terminal → error (auto-emit run_error)
 * 4. Script completed without terminal → completed (auto-emit run_complete)
 *
 * Lifecycle ordering:
 * - beforeRun → script → afterRun (success path)
 * - beforeRun → script → onError (failure path)
 * - cleanup always runs after terminal emission attempt
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
  const stdioSink = new StdioSink(output)
  const sink = new ObservingSink(stdioSink)

  let browser: Browser | null = null
  let browserContext: BrowserContext | null = null
  let page: Page | null = null
  let script: LoadedScript<Job> | null = null
  let ctx: ReturnType<typeof createContext<Job>> | null = null
  let scriptThrew = false
  let scriptError: { message: string; stack?: string } | null = null

  /**
   * Determine final outcome based on sink state and script behavior.
   * Precedence: sink failure > terminal state > script error > completed
   */
  function determineOutcome(sinkState: SinkState): ExecutorResult {
    // 1. Sink failure at any point → crash
    if (sinkState.isSinkFailed()) {
      const failure = sinkState.getSinkFailure()
      const message = failure instanceof Error ? failure.message : String(failure)
      return {
        outcome: { status: 'crash', message },
        terminalEmitted: sinkState.getTerminalState() !== null
      }
    }

    // 2. Terminal event successfully written → match that event
    const terminalState = sinkState.getTerminalState()
    if (terminalState !== null) {
      if (terminalState.type === 'run_error') {
        return {
          outcome: {
            status: 'error',
            errorType: terminalState.errorType ?? 'unknown',
            message: terminalState.message ?? 'Unknown error'
          },
          terminalEmitted: true
        }
      }
      // run_complete
      return {
        outcome: { status: 'completed', summary: terminalState.summary },
        terminalEmitted: true
      }
    }

    // 3. Script threw without terminal → error
    if (scriptThrew && scriptError) {
      return {
        outcome: {
          status: 'error',
          errorType: 'script_error',
          message: scriptError.message,
          stack: scriptError.stack
        },
        terminalEmitted: false
      }
    }

    // 4. Script completed without terminal → completed
    return {
      outcome: { status: 'completed' },
      terminalEmitted: false
    }
  }

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

    // 2. Launch Puppeteer (with proxy if configured)
    const launchOptions = buildPuppeteerLaunchOptions(config.puppeteerOptions, config.proxy)
    browser = await puppeteer.launch(launchOptions)
    browserContext = await browser.createBrowserContext()
    page = await browserContext.newPage()

    // Apply proxy authentication if needed (per CONTRACT_PROXY.md)
    if (config.proxy?.username && config.proxy?.password) {
      await page.authenticate({
        username: config.proxy.username,
        password: config.proxy.password
      })
    }

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

      // afterRun hook (only on success path)
      if (script.hooks.afterRun) {
        await script.hooks.afterRun(ctx)
      }
    } catch (err) {
      // Script threw - capture for outcome determination
      scriptThrew = true
      scriptError = {
        message: err instanceof Error ? err.message : String(err),
        stack: err instanceof Error ? err.stack : undefined
      }

      // onError hook - only call if no terminal event was emitted.
      // If a terminal event exists, the outcome is already determined by that event,
      // and calling onError could cause confusing behavior (e.g., trying to emit
      // another terminal event, or performing recovery that's no longer meaningful).
      if (script.hooks.onError && sink.getTerminalState() === null) {
        try {
          await script.hooks.onError(err, ctx)
        } catch {
          // Swallow hook errors to avoid masking original
        }
      }
    }

    // 5. Auto-emit terminal if needed and sink is healthy
    if (!sink.isSinkFailed() && sink.getTerminalState() === null) {
      try {
        if (scriptThrew && scriptError) {
          await ctx.emit.runError({
            error_type: 'script_error',
            message: scriptError.message,
            stack: scriptError.stack
          })
        } else {
          await ctx.emit.runComplete()
        }
      } catch (err) {
        // If this is a TerminalEventError, script already emitted terminal
        // If this is a sink failure, determineOutcome will handle it
        if (isSinkFailure(err)) {
          // Sink failed during auto-emit - will be handled by determineOutcome
        }
        // TerminalEventError means script already emitted - that's fine
      }
    }

    // 6. Cleanup hook (always runs, after terminal emission attempt)
    // NOTE: cleanup MUST NOT emit events; they will throw TerminalEventError
    if (script.hooks.cleanup && ctx) {
      try {
        await script.hooks.cleanup(ctx)
      } catch {
        // Swallow cleanup errors
      }
    }

    // 7. Determine final outcome
    return determineOutcome(sink)
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
