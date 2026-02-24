/**
 * Terminal semantics tests for runError.
 *
 * Goal: Lock down terminal behavior as a state machine.
 * Invariant: Exactly one logical terminal event may be persisted.
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI, SinkFailedError, TerminalEventError } from '../../../src/emit-impl'
import type { CheckpointId } from '../../../src/types/events'
import { createRunMeta, FakeSink } from '../_harness'

describe('runError terminal semantics', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('emitting runError once succeeds', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'script_error', message: 'Something failed' })

    expect(sink.envelopes).toHaveLength(1)
    expect(sink.envelopes[0].type).toBe('run_error')
  })

  it('emitting after runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'timeout', message: 'Timed out' })

    await expect(emit.item({ item_type: 'test', data: {} })).rejects.toThrow(TerminalEventError)
  })

  it('emitting log after runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'blocked', message: 'Blocked' })

    await expect(emit.log({ level: 'info', message: 'test' })).rejects.toThrow(TerminalEventError)
  })

  it('emitting artifact after runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'abort', message: 'Aborted' })

    await expect(
      emit.artifact({ name: 'test.txt', content_type: 'text/plain', data: Buffer.from('') })
    ).rejects.toThrow(TerminalEventError)
  })

  it('emitting checkpoint after runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runError({ error_type: 'error', message: 'Error' })

    await expect(emit.checkpoint({ checkpoint_id: 'cp' as CheckpointId })).rejects.toThrow(
      TerminalEventError
    )
  })

  it('emitting enqueue after runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runError({ error_type: 'error', message: 'Error' })

    await expect(emit.enqueue({ target: 'next', params: {} })).rejects.toThrow(TerminalEventError)
  })

  it('emitting rotateProxy after runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runError({ error_type: 'error', message: 'Error' })

    await expect(emit.rotateProxy()).rejects.toThrow(TerminalEventError)
  })

  it('second runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'first_error', message: 'First' })

    await expect(emit.runError({ error_type: 'second_error', message: 'Second' })).rejects.toThrow(
      TerminalEventError
    )
  })

  it('runComplete after runError throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'script_error', message: 'Error' })

    await expect(emit.runComplete()).rejects.toThrow(TerminalEventError)
  })

  it('non-terminal emits before runError are allowed', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'item', data: {} })
    await emit.log({ level: 'info', message: 'log' })
    await emit.runError({ error_type: 'script_error', message: 'Failed' })

    expect(sink.envelopes).toHaveLength(3)
    expect(sink.envelopes[0].type).toBe('item')
    expect(sink.envelopes[1].type).toBe('log')
    expect(sink.envelopes[2].type).toBe('run_error')
  })

  it('convenience log methods after runError throw errors', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runError({ error_type: 'error', message: 'Error' })

    // First emit after terminal throws TerminalEventError
    await expect(emit.debug('test')).rejects.toThrow(TerminalEventError)

    // Subsequent emits throw SinkFailedError (wrapping TerminalEventError)
    // because the error handler in serialize() sets sinkFailed on any error
    await expect(emit.info('test')).rejects.toThrow(SinkFailedError)
    await expect(emit.warn('test')).rejects.toThrow(SinkFailedError)
    await expect(emit.error('test')).rejects.toThrow(SinkFailedError)
  })
})
