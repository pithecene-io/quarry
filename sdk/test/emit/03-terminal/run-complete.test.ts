/**
 * Terminal semantics tests for runComplete.
 *
 * Goal: Lock down terminal behavior as a state machine.
 * Invariant: Exactly one logical terminal event may be persisted.
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI, TerminalEventError } from '../../../src/emit-impl'
import { createRunMeta, FakeSink } from '../_harness'

describe('runComplete terminal semantics', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('emitting runComplete once succeeds', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    expect(sink.envelopes).toHaveLength(1)
    expect(sink.envelopes[0].type).toBe('run_complete')
  })

  it('emitting after runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    await expect(emit.item({ item_type: 'test', data: {} })).rejects.toThrow(TerminalEventError)
  })

  it('emitting log after runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    await expect(emit.log({ level: 'info', message: 'test' })).rejects.toThrow(TerminalEventError)
  })

  it('emitting artifact after runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    await expect(
      emit.artifact({ name: 'test.txt', content_type: 'text/plain', data: Buffer.from('') })
    ).rejects.toThrow(TerminalEventError)
  })

  it('emitting checkpoint after runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runComplete()

    await expect(emit.checkpoint({ checkpoint_id: 'cp' as any })).rejects.toThrow(
      TerminalEventError
    )
  })

  it('emitting enqueue after runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runComplete()

    await expect(emit.enqueue({ target: 'next', params: {} })).rejects.toThrow(TerminalEventError)
  })

  it('emitting rotateProxy after runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runComplete()

    await expect(emit.rotateProxy()).rejects.toThrow(TerminalEventError)
  })

  it('second runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    await expect(emit.runComplete()).rejects.toThrow(TerminalEventError)
  })

  it('runError after runComplete throws TerminalEventError', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    await expect(emit.runError({ error_type: 'script_error', message: 'error' })).rejects.toThrow(
      TerminalEventError
    )
  })

  it('non-terminal emits before runComplete are allowed', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'item', data: {} })
    await emit.log({ level: 'info', message: 'log' })
    await emit.runComplete()

    expect(sink.envelopes).toHaveLength(3)
    expect(sink.envelopes[0].type).toBe('item')
    expect(sink.envelopes[1].type).toBe('log')
    expect(sink.envelopes[2].type).toBe('run_complete')
  })

  it('TerminalEventError has correct name', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.runComplete()

    try {
      await emit.item({ item_type: 'test', data: {} })
      expect.fail('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(TerminalEventError)
      expect((err as Error).name).toBe('TerminalEventError')
    }
  })
})
