/**
 * Tests for sequence number monotonicity.
 *
 * Goal: Prove that seq increments strictly by 1 for each persisted event.
 * Invariant: seq represents persisted order, starting at 1.
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import { FakeSink, createRunMeta } from '../_harness'
import type { CheckpointId } from '../../../src/types/events'

describe('sequence number monotonicity', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('seq starts at 1 for first event', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'first', data: {} })

    expect(sink.envelopes[0].seq).toBe(1)
  })

  it('seq increments by 1 for each event', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'a', data: {} })
    await emit.item({ item_type: 'b', data: {} })
    await emit.item({ item_type: 'c', data: {} })

    expect(sink.envelopes.map((e) => e.seq)).toEqual([1, 2, 3])
  })

  it('seq increments across different event types', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'item', data: {} })
    await emit.log({ level: 'info', message: 'log' })
    await emit.checkpoint({ checkpoint_id: 'cp' as CheckpointId })
    await emit.enqueue({ target: 'next', params: {} })
    await emit.rotateProxy()

    expect(sink.envelopes.map((e) => e.seq)).toEqual([1, 2, 3, 4, 5])
  })

  it('seq continues through terminal event', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'item', data: {} })
    await emit.log({ level: 'info', message: 'log' })
    await emit.runComplete()

    expect(sink.envelopes.map((e) => e.seq)).toEqual([1, 2, 3])
    expect(sink.envelopes[2].type).toBe('run_complete')
  })

  it('seq is strictly increasing under concurrent emits', async () => {
    const emit = createEmitAPI(run, sink)

    await Promise.all([
      emit.item({ item_type: 'a', data: {} }),
      emit.item({ item_type: 'b', data: {} }),
      emit.item({ item_type: 'c', data: {} }),
      emit.item({ item_type: 'd', data: {} }),
      emit.item({ item_type: 'e', data: {} })
    ])

    const seqs = sink.envelopes.map((e) => e.seq)

    // Verify strictly increasing
    for (let i = 1; i < seqs.length; i++) {
      expect(seqs[i]).toBe(seqs[i - 1] + 1)
    }
  })

  it('seq never skips values', async () => {
    const emit = createEmitAPI(run, sink)

    // Emit many events
    for (let i = 0; i < 20; i++) {
      await emit.item({ item_type: `item-${i}`, data: {} })
    }

    const seqs = sink.envelopes.map((e) => e.seq)

    // Should be [1, 2, 3, ..., 20]
    expect(seqs).toEqual(Array.from({ length: 20 }, (_, i) => i + 1))
  })

  it('seq never duplicates', async () => {
    const emit = createEmitAPI(run, sink)

    await Promise.all([
      emit.item({ item_type: 'a', data: {} }),
      emit.item({ item_type: 'b', data: {} }),
      emit.item({ item_type: 'c', data: {} })
    ])

    const seqs = sink.envelopes.map((e) => e.seq)
    const uniqueSeqs = new Set(seqs)

    expect(uniqueSeqs.size).toBe(seqs.length)
  })

  it('artifact emit gets single seq (not one for data + one for event)', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'before', data: {} })
    await emit.artifact({ name: 'test.txt', content_type: 'text/plain', data: Buffer.from('data') })
    await emit.item({ item_type: 'after', data: {} })

    // Artifact data write doesn't get a seq, only the artifact event does
    const seqs = sink.envelopes.map((e) => e.seq)
    expect(seqs).toEqual([1, 2, 3])
  })

  it('seq is integer (not float)', async () => {
    const emit = createEmitAPI(run, sink)

    for (let i = 0; i < 10; i++) {
      await emit.item({ item_type: `item-${i}`, data: {} })
    }

    for (const envelope of sink.envelopes) {
      expect(Number.isInteger(envelope.seq)).toBe(true)
    }
  })

  it('seq is positive', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'test', data: {} })

    expect(sink.envelopes[0].seq).toBeGreaterThan(0)
  })
})
