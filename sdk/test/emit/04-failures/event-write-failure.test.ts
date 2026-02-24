/**
 * Failure semantics tests for event write failures.
 *
 * Goal: Define behavior under sink failure (fail-fast).
 * Invariant: After first sink failure, Emit is permanently failed.
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI, SinkFailedError } from '../../../src/emit-impl'
import type { CheckpointId } from '../../../src/types/events'
import { createRunMeta, FakeSink } from '../_harness'

describe('event write failure semantics', () => {
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    run = createRunMeta()
  })

  it('throws original error on first failure', async () => {
    const injectedError = new Error('Sink connection lost')
    const sink = new FakeSink({ failOnEventWrite: 1, failureError: injectedError })
    const emit = createEmitAPI(run, sink)

    await expect(emit.item({ item_type: 'test', data: {} })).rejects.toThrow(injectedError)
  })

  it('throws SinkFailedError on subsequent emits after failure', async () => {
    const injectedError = new Error('Connection lost')
    const sink = new FakeSink({ failOnEventWrite: 1, failureError: injectedError })
    const emit = createEmitAPI(run, sink)

    // First emit fails with original error
    await expect(emit.item({ item_type: 'first', data: {} })).rejects.toThrow(injectedError)

    // Subsequent emit fails with SinkFailedError
    await expect(emit.item({ item_type: 'second', data: {} })).rejects.toThrow(SinkFailedError)
  })

  it('SinkFailedError contains original error as cause', async () => {
    const injectedError = new Error('Original failure')
    const sink = new FakeSink({ failOnEventWrite: 1, failureError: injectedError })
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'first', data: {} }).catch(() => {})

    try {
      await emit.item({ item_type: 'second', data: {} })
      expect.fail('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(SinkFailedError)
      expect((err as SinkFailedError).cause).toBe(injectedError)
    }
  })

  it('no seq increment on failed write', async () => {
    const sink = new FakeSink({ failOnEventWrite: 2 })
    const emit = createEmitAPI(run, sink)

    // First write succeeds (seq = 1)
    await emit.item({ item_type: 'success', data: {} })
    expect(sink.envelopes[0].seq).toBe(1)

    // Second write fails - no envelope persisted
    await emit.item({ item_type: 'fail', data: {} }).catch(() => {})

    // Only one envelope persisted
    expect(sink.envelopes).toHaveLength(1)
  })

  it('all emit methods fail after sink failure', async () => {
    const sink = new FakeSink({ failOnEventWrite: 1 })
    const emit = createEmitAPI(run, sink)

    // Trigger failure
    await emit.item({ item_type: 'fail', data: {} }).catch(() => {})

    // All methods should throw SinkFailedError
    await expect(emit.item({ item_type: 'test', data: {} })).rejects.toThrow(SinkFailedError)
    await expect(emit.log({ level: 'info', message: 'test' })).rejects.toThrow(SinkFailedError)
    await expect(emit.checkpoint({ checkpoint_id: 'cp' as CheckpointId })).rejects.toThrow(
      SinkFailedError
    )
    await expect(emit.enqueue({ target: 'next', params: {} })).rejects.toThrow(SinkFailedError)
    await expect(emit.rotateProxy()).rejects.toThrow(SinkFailedError)
    await expect(emit.debug('test')).rejects.toThrow(SinkFailedError)
    await expect(emit.runComplete()).rejects.toThrow(SinkFailedError)
    await expect(emit.runError({ error_type: 'error', message: 'Error' })).rejects.toThrow(
      SinkFailedError
    )
  })

  it('failure mid-sequence prevents further writes', async () => {
    const sink = new FakeSink({ failOnEventWrite: 3 })
    const emit = createEmitAPI(run, sink)

    // First two succeed
    await emit.item({ item_type: 'a', data: {} })
    await emit.item({ item_type: 'b', data: {} })

    // Third fails
    await emit.item({ item_type: 'c', data: {} }).catch(() => {})

    // Fourth throws SinkFailedError
    await expect(emit.item({ item_type: 'd', data: {} })).rejects.toThrow(SinkFailedError)

    // Only two envelopes persisted
    expect(sink.envelopes).toHaveLength(2)
    expect(sink.envelopes.map((e) => e.payload.item_type)).toEqual(['a', 'b'])
  })

  it('SinkFailedError has correct name', async () => {
    const sink = new FakeSink({ failOnEventWrite: 1 })
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'fail', data: {} }).catch(() => {})

    try {
      await emit.item({ item_type: 'test', data: {} })
      expect.fail('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(SinkFailedError)
      expect((err as Error).name).toBe('SinkFailedError')
    }
  })

  it('concurrent emits after failure all throw SinkFailedError', async () => {
    const sink = new FakeSink({ failOnEventWrite: 1 })
    const emit = createEmitAPI(run, sink)

    // Trigger failure
    await emit.item({ item_type: 'fail', data: {} }).catch(() => {})

    // All concurrent emits should fail
    const results = await Promise.allSettled([
      emit.item({ item_type: 'a', data: {} }),
      emit.item({ item_type: 'b', data: {} }),
      emit.item({ item_type: 'c', data: {} })
    ])

    for (const result of results) {
      expect(result.status).toBe('rejected')
      if (result.status === 'rejected') {
        expect(result.reason).toBeInstanceOf(SinkFailedError)
      }
    }
  })
})
