/**
 * Failure semantics tests for terminal event write failures.
 *
 * Goal: Ensure terminalEmitted is not latched if terminal write fails.
 * Invariant: Terminal state only latched after successful persistence.
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { createEmitAPI, SinkFailedError, TerminalEventError } from '../../../src/emit-impl'
import { FakeSink, createRunMeta } from '../_harness'

describe('terminal write failure semantics', () => {
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    run = createRunMeta()
  })

  describe('runComplete write failure', () => {
    it('throws original error when runComplete write fails', async () => {
      const injectedError = new Error('Storage unavailable')
      const sink = new FakeSink({ failOnEventWrite: 1, failureError: injectedError })
      const emit = createEmitAPI(run, sink)

      await expect(emit.runComplete()).rejects.toThrow(injectedError)
    })

    it('terminalEmitted not latched if runComplete write fails', async () => {
      const sink = new FakeSink({ failOnEventWrite: 1 })
      const emit = createEmitAPI(run, sink)

      // runComplete fails
      await emit.runComplete().catch(() => {})

      // The error is SinkFailedError, not TerminalEventError
      // This proves terminal state was not latched
      try {
        await emit.item({ item_type: 'test', data: {} })
        expect.fail('should have thrown')
      } catch (err) {
        // Should be SinkFailedError (from sink failure), not TerminalEventError
        expect(err).toBeInstanceOf(SinkFailedError)
        expect(err).not.toBeInstanceOf(TerminalEventError)
      }
    })

    it('no runComplete event persisted on failure', async () => {
      const sink = new FakeSink({ failOnEventWrite: 1 })
      const emit = createEmitAPI(run, sink)

      await emit.runComplete().catch(() => {})

      expect(sink.envelopes).toHaveLength(0)
    })
  })

  describe('runError write failure', () => {
    it('throws original error when runError write fails', async () => {
      const injectedError = new Error('Network failure')
      const sink = new FakeSink({ failOnEventWrite: 1, failureError: injectedError })
      const emit = createEmitAPI(run, sink)

      await expect(
        emit.runError({ error_type: 'script_error', message: 'Error' })
      ).rejects.toThrow(injectedError)
    })

    it('terminalEmitted not latched if runError write fails', async () => {
      const sink = new FakeSink({ failOnEventWrite: 1 })
      const emit = createEmitAPI(run, sink)

      // runError fails
      await emit.runError({ error_type: 'error', message: 'Error' }).catch(() => {})

      // Subsequent emit fails with SinkFailedError, not TerminalEventError
      try {
        await emit.item({ item_type: 'test', data: {} })
        expect.fail('should have thrown')
      } catch (err) {
        expect(err).toBeInstanceOf(SinkFailedError)
        expect(err).not.toBeInstanceOf(TerminalEventError)
      }
    })

    it('no runError event persisted on failure', async () => {
      const sink = new FakeSink({ failOnEventWrite: 1 })
      const emit = createEmitAPI(run, sink)

      await emit.runError({ error_type: 'error', message: 'Error' }).catch(() => {})

      expect(sink.envelopes).toHaveLength(0)
    })
  })

  describe('terminal after successful events', () => {
    it('runComplete failure after items - terminal not latched', async () => {
      const sink = new FakeSink({ failOnEventWrite: 3 }) // Fail on 3rd write
      const emit = createEmitAPI(run, sink)

      // First two items succeed
      await emit.item({ item_type: 'a', data: {} })
      await emit.item({ item_type: 'b', data: {} })

      // runComplete fails (3rd write)
      await emit.runComplete().catch(() => {})

      // Subsequent emit fails with SinkFailedError
      await expect(emit.item({ item_type: 'c', data: {} })).rejects.toThrow(SinkFailedError)

      // Only first two items persisted
      expect(sink.envelopes).toHaveLength(2)
    })

    it('runError failure after items - terminal not latched', async () => {
      const sink = new FakeSink({ failOnEventWrite: 3 })
      const emit = createEmitAPI(run, sink)

      await emit.item({ item_type: 'a', data: {} })
      await emit.item({ item_type: 'b', data: {} })

      // runError fails
      await emit.runError({ error_type: 'error', message: 'Error' }).catch(() => {})

      // Only items persisted, no terminal event
      expect(sink.envelopes).toHaveLength(2)
      expect(sink.envelopes.every((e) => e.type === 'item')).toBe(true)
    })
  })

  describe('sink failure vs terminal state priority', () => {
    it('sink failure takes precedence over terminal state check', async () => {
      const sink = new FakeSink({ failOnEventWrite: 1 })
      const emit = createEmitAPI(run, sink)

      // First emit (runComplete) fails - sink is now failed
      await emit.runComplete().catch(() => {})

      // Second runComplete attempt fails with SinkFailedError, not TerminalEventError
      // because sink failure check happens before terminal check
      await expect(emit.runComplete()).rejects.toThrow(SinkFailedError)
    })

    it('successful terminal then subsequent emits throw TerminalEventError', async () => {
      const sink = new FakeSink()
      const emit = createEmitAPI(run, sink)

      // runComplete succeeds
      await emit.runComplete()

      // Subsequent emit throws TerminalEventError (not SinkFailedError)
      await expect(emit.item({ item_type: 'test', data: {} })).rejects.toThrow(TerminalEventError)
    })
  })

  describe('no seq increment on terminal write failure', () => {
    it('runComplete failure does not increment seq', async () => {
      const sink = new FakeSink({ failOnEventWrite: 2 })
      const emit = createEmitAPI(run, sink)

      // First item succeeds (seq = 1)
      await emit.item({ item_type: 'a', data: {} })
      expect(sink.envelopes[0].seq).toBe(1)

      // runComplete fails - no seq increment
      await emit.runComplete().catch(() => {})

      // Still only one envelope
      expect(sink.envelopes).toHaveLength(1)
    })

    it('runError failure does not increment seq', async () => {
      const sink = new FakeSink({ failOnEventWrite: 2 })
      const emit = createEmitAPI(run, sink)

      await emit.item({ item_type: 'a', data: {} })

      await emit.runError({ error_type: 'error', message: 'Error' }).catch(() => {})

      expect(sink.envelopes).toHaveLength(1)
    })
  })
})
