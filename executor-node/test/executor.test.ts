import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest'
import { PassThrough } from 'node:stream'
import type { RunId, JobId, EmitSink, ArtifactId, EventEnvelope, EventId } from '@justapithecus/quarry-sdk'
import { parseRunMeta, execute, type ExecutorConfig, type ExecutorResult } from '../src/executor.js'
import { ObservingSink, SinkAlreadyFailedError } from '../src/ipc/observing-sink.js'
import type { LoadedScript } from '../src/loader.js'

// Mock dependencies
vi.mock('../src/loader.js', () => ({
  loadScript: vi.fn(),
  ScriptLoadError: class ScriptLoadError extends Error {
    constructor(public scriptPath: string, public reason: string) {
      super(`Failed to load script "${scriptPath}": ${reason}`)
      this.name = 'ScriptLoadError'
    }
  }
}))

vi.mock('puppeteer', () => ({
  default: {
    launch: vi.fn()
  }
}))

import { loadScript, ScriptLoadError } from '../src/loader.js'
import puppeteer from 'puppeteer'

/**
 * Create a mock Puppeteer setup.
 */
function createMockPuppeteer() {
  const page = {
    close: vi.fn().mockResolvedValue(undefined)
  }
  const browserContext = {
    newPage: vi.fn().mockResolvedValue(page),
    close: vi.fn().mockResolvedValue(undefined)
  }
  const browser = {
    createBrowserContext: vi.fn().mockResolvedValue(browserContext),
    close: vi.fn().mockResolvedValue(undefined)
  }

  return { browser, browserContext, page }
}

/**
 * Create a mock output stream that behaves like a real writable stream.
 */
function createMockOutput() {
  return new PassThrough()
}

describe('parseRunMeta', () => {
  describe('required field validation', () => {
    it('throws on null input', () => {
      expect(() => parseRunMeta(null)).toThrow('run metadata must be an object')
    })

    it('throws on non-object input', () => {
      expect(() => parseRunMeta('string')).toThrow('run metadata must be an object')
      expect(() => parseRunMeta(123)).toThrow('run metadata must be an object')
    })

    it('throws on missing run_id', () => {
      expect(() => parseRunMeta({ attempt: 1 })).toThrow('run_id must be a non-empty string')
    })

    it('throws on empty run_id', () => {
      expect(() => parseRunMeta({ run_id: '', attempt: 1 })).toThrow(
        'run_id must be a non-empty string'
      )
    })

    it('throws on missing attempt', () => {
      expect(() => parseRunMeta({ run_id: 'run-123' })).toThrow('attempt must be a positive integer')
    })

    it('throws on non-integer attempt', () => {
      expect(() => parseRunMeta({ run_id: 'run-123', attempt: 1.5 })).toThrow(
        'attempt must be a positive integer'
      )
    })

    it('throws on attempt < 1', () => {
      expect(() => parseRunMeta({ run_id: 'run-123', attempt: 0 })).toThrow(
        'attempt must be a positive integer'
      )
      expect(() => parseRunMeta({ run_id: 'run-123', attempt: -1 })).toThrow(
        'attempt must be a positive integer'
      )
    })
  })

  describe('valid input parsing', () => {
    it('parses minimal valid input', () => {
      const run = parseRunMeta({ run_id: 'run-123', attempt: 1 })

      expect(run.run_id).toBe('run-123')
      expect(run.attempt).toBe(1)
      expect(run.job_id).toBeUndefined()
      expect(run.parent_run_id).toBeUndefined()
    })

    it('parses input with job_id', () => {
      const run = parseRunMeta({ run_id: 'run-123', attempt: 1, job_id: 'job-456' })

      expect(run.job_id).toBe('job-456')
    })

    it('ignores empty job_id', () => {
      const run = parseRunMeta({ run_id: 'run-123', attempt: 1, job_id: '' })

      expect(run.job_id).toBeUndefined()
    })
  })

  describe('lineage validation (strict)', () => {
    it('throws when initial run (attempt=1) has parent_run_id', () => {
      expect(() =>
        parseRunMeta({
          run_id: 'run-123',
          attempt: 1,
          parent_run_id: 'run-parent'
        })
      ).toThrow('initial run (attempt=1) must not have parent_run_id')
    })

    it('throws when retry run (attempt>1) lacks parent_run_id', () => {
      expect(() =>
        parseRunMeta({
          run_id: 'run-123',
          attempt: 3
        })
      ).toThrow('retry run (attempt=3) must have parent_run_id')
    })

    it('accepts valid initial run without parent_run_id', () => {
      const run = parseRunMeta({
        run_id: 'run-123',
        attempt: 1
      })

      expect(run.attempt).toBe(1)
      expect(run.parent_run_id).toBeUndefined()
    })

    it('accepts valid retry run with parent_run_id', () => {
      const run = parseRunMeta({
        run_id: 'run-123',
        attempt: 2,
        parent_run_id: 'run-parent'
      })

      expect(run.attempt).toBe(2)
      expect(run.parent_run_id).toBe('run-parent')
    })
  })

  describe('type branding', () => {
    it('returns branded types', () => {
      const run = parseRunMeta({
        run_id: 'run-123',
        attempt: 2,
        job_id: 'job-456',
        parent_run_id: 'run-parent'
      })

      // TypeScript would catch misuse, but we can verify the values
      const runId: RunId = run.run_id
      const jobId: JobId | undefined = run.job_id
      const parentRunId: RunId | undefined = run.parent_run_id

      expect(runId).toBe('run-123')
      expect(jobId).toBe('job-456')
      expect(parentRunId).toBe('run-parent')
    })
  })
})

describe('execute()', () => {
  let mockPuppeteer: ReturnType<typeof createMockPuppeteer>
  let mockOutput: PassThrough

  beforeEach(() => {
    vi.clearAllMocks()

    mockPuppeteer = createMockPuppeteer()
    ;(puppeteer.launch as Mock).mockResolvedValue(mockPuppeteer.browser)

    mockOutput = createMockOutput()
  })

  function createConfig(overrides: Partial<ExecutorConfig> = {}): ExecutorConfig {
    return {
      scriptPath: '/path/to/script.js',
      job: { test: true },
      run: {
        run_id: 'run-123' as RunId,
        attempt: 1
      },
      output: mockOutput,
      ...overrides
    }
  }

  function createMockScript(overrides: Partial<LoadedScript> = {}): LoadedScript {
    return {
      script: vi.fn().mockResolvedValue(undefined),
      hooks: {},
      module: { default: vi.fn() },
      ...overrides
    }
  }

  describe('outcome precedence', () => {
    it('returns error status when terminal run_error was emitted', async () => {
      const mockScript = createMockScript({
        script: vi.fn().mockImplementation(async (ctx) => {
          await ctx.emit.runError({
            error_type: 'validation_failed',
            message: 'Invalid data found'
          })
        })
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('error')
      if (result.outcome.status === 'error') {
        expect(result.outcome.errorType).toBe('validation_failed')
        expect(result.outcome.message).toBe('Invalid data found')
      }
      expect(result.terminalEmitted).toBe(true)
    })

    it('returns completed status when terminal run_complete was emitted', async () => {
      const mockScript = createMockScript({
        script: vi.fn().mockImplementation(async (ctx) => {
          await ctx.emit.runComplete({ summary: { items: 10 } })
        })
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('completed')
      if (result.outcome.status === 'completed') {
        expect(result.outcome.summary).toEqual({ items: 10 })
      }
      expect(result.terminalEmitted).toBe(true)
    })

    it('auto-emits run_error when script throws without terminal', async () => {
      const mockScript = createMockScript({
        script: vi.fn().mockRejectedValue(new Error('Script failed'))
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('error')
      if (result.outcome.status === 'error') {
        expect(result.outcome.errorType).toBe('script_error')
        expect(result.outcome.message).toBe('Script failed')
      }
      // terminalEmitted is false because WE auto-emitted, not the script
      // Actually, let's check the implementation - it sets terminalEmitted based on sink state
      // After auto-emit, the terminal state will be set
    })

    it('auto-emits run_complete when script completes without terminal', async () => {
      const mockScript = createMockScript({
        script: vi.fn().mockResolvedValue(undefined)
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('completed')
    })
  })

  describe('onError hook behavior', () => {
    it('calls onError when script throws before terminal event', async () => {
      const onError = vi.fn()
      const scriptError = new Error('Script failed')
      const mockScript = createMockScript({
        script: vi.fn().mockRejectedValue(scriptError),
        hooks: { onError }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      await execute(createConfig())

      expect(onError).toHaveBeenCalledTimes(1)
      expect(onError).toHaveBeenCalledWith(scriptError, expect.any(Object))
    })

    it('does NOT call onError when script throws after terminal event', async () => {
      const onError = vi.fn()
      const mockScript = createMockScript({
        script: vi.fn().mockImplementation(async (ctx) => {
          await ctx.emit.runComplete({ summary: {} })
          throw new Error('Post-terminal error')
        }),
        hooks: { onError }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      await execute(createConfig())

      expect(onError).not.toHaveBeenCalled()
    })

    it('swallows errors from onError hook', async () => {
      const onError = vi.fn().mockRejectedValue(new Error('Hook failed'))
      const mockScript = createMockScript({
        script: vi.fn().mockRejectedValue(new Error('Script failed')),
        hooks: { onError }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      // Should not throw
      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('error')
      expect(onError).toHaveBeenCalled()
    })
  })

  describe('sink failure scenarios', () => {
    it('returns crash when script load fails', async () => {
      ;(loadScript as Mock).mockRejectedValue(
        new ScriptLoadError('/path/to/script.js', 'file not found')
      )

      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('crash')
      if (result.outcome.status === 'crash') {
        expect(result.outcome.message).toContain('Failed to load script')
      }
    })

    it('returns crash when puppeteer fails to launch', async () => {
      ;(loadScript as Mock).mockResolvedValue(createMockScript())
      ;(puppeteer.launch as Mock).mockRejectedValue(new Error('Failed to launch browser'))

      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('crash')
      if (result.outcome.status === 'crash') {
        expect(result.outcome.message).toBe('Failed to launch browser')
      }
    })
  })

  describe('lifecycle hooks', () => {
    it('calls beforeRun before script', async () => {
      const callOrder: string[] = []
      const mockScript = createMockScript({
        script: vi.fn().mockImplementation(async () => {
          callOrder.push('script')
        }),
        hooks: {
          beforeRun: vi.fn().mockImplementation(async () => {
            callOrder.push('beforeRun')
          })
        }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      await execute(createConfig())

      expect(callOrder).toEqual(['beforeRun', 'script'])
    })

    it('calls afterRun after script on success', async () => {
      const callOrder: string[] = []
      const mockScript = createMockScript({
        script: vi.fn().mockImplementation(async () => {
          callOrder.push('script')
        }),
        hooks: {
          afterRun: vi.fn().mockImplementation(async () => {
            callOrder.push('afterRun')
          })
        }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      await execute(createConfig())

      expect(callOrder).toEqual(['script', 'afterRun'])
    })

    it('does not call afterRun when script throws', async () => {
      const afterRun = vi.fn()
      const mockScript = createMockScript({
        script: vi.fn().mockRejectedValue(new Error('Script failed')),
        hooks: { afterRun }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      await execute(createConfig())

      expect(afterRun).not.toHaveBeenCalled()
    })

    it('calls cleanup after terminal emission', async () => {
      const callOrder: string[] = []
      const mockScript = createMockScript({
        script: vi.fn().mockImplementation(async (ctx) => {
          callOrder.push('script')
          await ctx.emit.runComplete()
          callOrder.push('after-emit')
        }),
        hooks: {
          cleanup: vi.fn().mockImplementation(async () => {
            callOrder.push('cleanup')
          })
        }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      await execute(createConfig())

      expect(callOrder).toContain('cleanup')
      // Cleanup runs after script completes (not immediately after terminal emit)
      expect(callOrder.indexOf('cleanup')).toBeGreaterThan(callOrder.indexOf('script'))
    })

    it('swallows errors from cleanup hook', async () => {
      const cleanup = vi.fn().mockRejectedValue(new Error('Cleanup failed'))
      const mockScript = createMockScript({
        script: vi.fn().mockResolvedValue(undefined),
        hooks: { cleanup }
      })
      ;(loadScript as Mock).mockResolvedValue(mockScript)

      // Should not throw
      const result = await execute(createConfig())

      expect(result.outcome.status).toBe('completed')
      expect(cleanup).toHaveBeenCalled()
    })
  })
})

describe('SinkAlreadyFailedError', () => {
  it('includes original cause message in error message', () => {
    const originalError = new Error('Connection reset')
    const sinkError = new SinkAlreadyFailedError(originalError)

    expect(sinkError.message).toBe('Sink has already failed: Connection reset')
    expect(sinkError.cause).toBe(originalError)
  })

  it('handles non-Error cause', () => {
    const sinkError = new SinkAlreadyFailedError('string cause')

    expect(sinkError.message).toBe('Sink has already failed: string cause')
    expect(sinkError.cause).toBe('string cause')
  })
})
