/**
 * Terminal exclusivity tests.
 *
 * Goal: Ensure exactly one terminal event can be persisted.
 * Invariant: runComplete and runError are mutually exclusive terminals.
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI, SinkFailedError, TerminalEventError } from '../../../src/emit-impl'
import type { CheckpointId } from '../../../src/types/events'
import { createRunMeta, FakeSink } from '../_harness'

describe('terminal event exclusivity', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  describe('runComplete then runError', () => {
    it('runError after runComplete throws', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.runComplete()

      await expect(emit.runError({ error_type: 'script_error', message: 'Error' })).rejects.toThrow(
        TerminalEventError
      )
    })

    it('only runComplete is persisted', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.runComplete()
      await emit.runError({ error_type: 'error', message: 'Error' }).catch(() => {})

      expect(sink.envelopes).toHaveLength(1)
      expect(sink.envelopes[0].type).toBe('run_complete')
    })
  })

  describe('runError then runComplete', () => {
    it('runComplete after runError throws', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.runError({ error_type: 'script_error', message: 'Error' })

      await expect(emit.runComplete()).rejects.toThrow(TerminalEventError)
    })

    it('only runError is persisted', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.runError({ error_type: 'error', message: 'Error' })
      await emit.runComplete().catch(() => {})

      expect(sink.envelopes).toHaveLength(1)
      expect(sink.envelopes[0].type).toBe('run_error')
    })
  })

  describe('double terminal attempts', () => {
    it('double runComplete - only first persisted', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.runComplete()
      await emit.runComplete().catch(() => {})

      expect(sink.envelopes).toHaveLength(1)
      expect(sink.envelopes[0].type).toBe('run_complete')
    })

    it('double runError - only first persisted', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.runError({ error_type: 'first', message: 'First' })
      await emit.runError({ error_type: 'second', message: 'Second' }).catch(() => {})

      expect(sink.envelopes).toHaveLength(1)
      expect(sink.envelopes[0].payload).toMatchObject({ error_type: 'first' })
    })
  })

  describe('concurrent terminal attempts', () => {
    it('concurrent runComplete - exactly one persisted', async () => {
      const emit = createEmitAPI(run, sink)

      const results = await Promise.allSettled([
        emit.runComplete({ summary: { first: true } }),
        emit.runComplete({ summary: { second: true } })
      ])

      // One should succeed, one should fail
      const fulfilled = results.filter((r) => r.status === 'fulfilled')
      const rejected = results.filter((r) => r.status === 'rejected')

      expect(fulfilled).toHaveLength(1)
      expect(rejected).toHaveLength(1)

      // Only one event persisted
      expect(sink.envelopes).toHaveLength(1)
      expect(sink.envelopes[0].type).toBe('run_complete')
    })

    it('concurrent runError - exactly one persisted', async () => {
      const emit = createEmitAPI(run, sink)

      const results = await Promise.allSettled([
        emit.runError({ error_type: 'first', message: 'First' }),
        emit.runError({ error_type: 'second', message: 'Second' })
      ])

      const fulfilled = results.filter((r) => r.status === 'fulfilled')
      const rejected = results.filter((r) => r.status === 'rejected')

      expect(fulfilled).toHaveLength(1)
      expect(rejected).toHaveLength(1)
      expect(sink.envelopes).toHaveLength(1)
    })

    it('concurrent runComplete and runError - exactly one persisted', async () => {
      const emit = createEmitAPI(run, sink)

      const results = await Promise.allSettled([
        emit.runComplete(),
        emit.runError({ error_type: 'error', message: 'Error' })
      ])

      const fulfilled = results.filter((r) => r.status === 'fulfilled')
      const rejected = results.filter((r) => r.status === 'rejected')

      expect(fulfilled).toHaveLength(1)
      expect(rejected).toHaveLength(1)
      expect(sink.envelopes).toHaveLength(1)
      // The type should be one of the terminal types
      expect(['run_complete', 'run_error']).toContain(sink.envelopes[0].type)
    })
  })

  describe('terminal state is final', () => {
    it('state persists across many attempts', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.runComplete()

      // Try many times
      for (let i = 0; i < 10; i++) {
        await emit.item({ item_type: 'test', data: {} }).catch(() => {})
        await emit.runComplete().catch(() => {})
        await emit.runError({ error_type: 'error', message: 'Error' }).catch(() => {})
      }

      // Still only one event
      expect(sink.envelopes).toHaveLength(1)
    })

    it('all post-terminal methods blocked', async () => {
      const emit = createEmitAPI(run, sink)
      await emit.runComplete()

      // First emit after terminal throws TerminalEventError
      await expect(emit.item({ item_type: 'a', data: {} })).rejects.toThrow(TerminalEventError)

      // Subsequent emits throw SinkFailedError (wrapping TerminalEventError)
      // because serialize() sets sinkFailed on any error including TerminalEventError
      await expect(emit.log({ level: 'info', message: 'b' })).rejects.toThrow(SinkFailedError)
      await expect(
        emit.artifact({ name: 'c', content_type: 'text/plain', data: Buffer.from('') })
      ).rejects.toThrow(SinkFailedError)
      await expect(emit.checkpoint({ checkpoint_id: 'd' as CheckpointId })).rejects.toThrow(
        SinkFailedError
      )
      await expect(emit.enqueue({ target: 'e', params: {} })).rejects.toThrow(SinkFailedError)
      await expect(emit.rotateProxy()).rejects.toThrow(SinkFailedError)
      await expect(emit.debug('f')).rejects.toThrow(SinkFailedError)
      await expect(emit.info('g')).rejects.toThrow(SinkFailedError)
      await expect(emit.warn('h')).rejects.toThrow(SinkFailedError)
      await expect(emit.error('i')).rejects.toThrow(SinkFailedError)
      await expect(emit.runError({ error_type: 'j', message: 'k' })).rejects.toThrow(
        SinkFailedError
      )
      await expect(emit.runComplete()).rejects.toThrow(SinkFailedError)
    })
  })
})
