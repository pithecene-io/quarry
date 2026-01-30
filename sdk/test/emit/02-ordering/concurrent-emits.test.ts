/**
 * Ordering tests for concurrent emits.
 *
 * Goal: Prove that Emit is strictly serialized.
 * Invariant: Emit behaves as a single-threaded log, regardless of caller concurrency.
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import type { CheckpointId } from '../../../src/types/events'
import { createRunMeta, FakeSink } from '../_harness'

describe('concurrent emits ordering', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('serializes concurrent emits via Promise.all', async () => {
    const emit = createEmitAPI(run, sink)

    // Fire multiple emits concurrently
    await Promise.all([
      emit.item({ item_type: 'a', data: {} }),
      emit.item({ item_type: 'b', data: {} }),
      emit.item({ item_type: 'c', data: {} })
    ])

    // All should be persisted
    expect(sink.envelopes).toHaveLength(3)

    // Seq should be strictly monotonic
    const seqs = sink.envelopes.map((e) => e.seq)
    expect(seqs).toEqual([1, 2, 3])
  })

  it('preserves call order under concurrent execution', async () => {
    const emit = createEmitAPI(run, sink)

    // Fire emits concurrently but in specific order
    const promises = [
      emit.item({ item_type: 'first', data: {} }),
      emit.item({ item_type: 'second', data: {} }),
      emit.item({ item_type: 'third', data: {} })
    ]

    await Promise.all(promises)

    // Order should match call order (first-in-first-out through chain)
    const types = sink.envelopes.map((e) => e.payload.item_type)
    expect(types).toEqual(['first', 'second', 'third'])
  })

  it('serializes mixed event types under concurrent execution', async () => {
    const emit = createEmitAPI(run, sink)

    await Promise.all([
      emit.item({ item_type: 'item', data: {} }),
      emit.log({ level: 'info', message: 'log' }),
      emit.checkpoint({ checkpoint_id: 'cp' as CheckpointId }),
      emit.enqueue({ target: 'next', params: {} }),
      emit.rotateProxy({ reason: 'test' })
    ])

    expect(sink.envelopes).toHaveLength(5)

    // All seqs should be strictly monotonic
    const seqs = sink.envelopes.map((e) => e.seq)
    expect(seqs).toEqual([1, 2, 3, 4, 5])
  })

  it('writeEvent calls never overlap (with delays)', async () => {
    // Use sink with delay to expose any interleaving
    sink = new FakeSink({ eventWriteDelayMs: 10 })
    const emit = createEmitAPI(run, sink)

    // Fire many emits concurrently
    await Promise.all([
      emit.item({ item_type: 'a', data: {} }),
      emit.item({ item_type: 'b', data: {} }),
      emit.item({ item_type: 'c', data: {} }),
      emit.item({ item_type: 'd', data: {} }),
      emit.item({ item_type: 'e', data: {} })
    ])

    // Check timestamps are ordered (allowing for same-millisecond)
    const timestamps = sink.eventCalls.map((c) => c.timestamp)
    for (let i = 1; i < timestamps.length; i++) {
      expect(timestamps[i]).toBeGreaterThanOrEqual(timestamps[i - 1])
    }

    // Call indices should be strictly increasing
    const indices = sink.eventCalls.map((c) => c.callIndex)
    for (let i = 1; i < indices.length; i++) {
      expect(indices[i]).toBeGreaterThan(indices[i - 1])
    }
  })

  it('events persisted in call order regardless of concurrent fire', async () => {
    sink = new FakeSink({ eventWriteDelayMs: 5 })
    const emit = createEmitAPI(run, sink)

    // Fire 10 emits concurrently
    const promises: Promise<void>[] = []
    for (let i = 0; i < 10; i++) {
      promises.push(emit.item({ item_type: `item-${i}`, data: { index: i } }))
    }

    await Promise.all(promises)

    // Verify order matches call order
    for (let i = 0; i < 10; i++) {
      expect(sink.envelopes[i].payload).toMatchObject({ item_type: `item-${i}` })
      expect(sink.envelopes[i].seq).toBe(i + 1)
    }
  })

  it('handles rapid sequential calls as if concurrent', async () => {
    const emit = createEmitAPI(run, sink)

    // Fire without await (effectively concurrent)
    const p1 = emit.item({ item_type: 'rapid-1', data: {} })
    const p2 = emit.item({ item_type: 'rapid-2', data: {} })
    const p3 = emit.item({ item_type: 'rapid-3', data: {} })

    await Promise.all([p1, p2, p3])

    expect(sink.envelopes).toHaveLength(3)
    expect(sink.envelopes.map((e) => e.seq)).toEqual([1, 2, 3])
  })

  it('artifacts interleaved with items maintain order', async () => {
    const emit = createEmitAPI(run, sink)

    await Promise.all([
      emit.item({ item_type: 'item-1', data: {} }),
      emit.artifact({ name: 'a.txt', content_type: 'text/plain', data: Buffer.from('a') }),
      emit.item({ item_type: 'item-2', data: {} })
    ])

    // All events persisted
    expect(sink.envelopes).toHaveLength(3)

    // Types should be in call order
    const types = sink.envelopes.map((e) => e.type)
    expect(types).toEqual(['item', 'artifact', 'item'])

    // Seqs should be monotonic
    expect(sink.envelopes.map((e) => e.seq)).toEqual([1, 2, 3])
  })
})
