/**
 * Unit tests for createBatcher utility.
 *
 * Goal: Validate batching semantics, auto-flush, pending counter,
 * option propagation, error handling, and edge cases.
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createBatcher } from '../../../src/batcher'
import { createEmitAPI, SinkFailedError, TerminalEventError } from '../../../src/emit-impl'
import type { EnqueuePayload } from '../../../src/types/events'
import { createRunMeta, FakeSink } from '../_harness'

describe('createBatcher', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  // ── Basic batching ───────────────────────────────────────────────

  describe('basic batching', () => {
    it('auto-flushes when buffer reaches configured size', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 3, target: 'worker' })

      await batcher.add({ id: 1 })
      await batcher.add({ id: 2 })
      expect(sink.envelopes).toHaveLength(0)

      await batcher.add({ id: 3 })
      expect(sink.envelopes).toHaveLength(1)
      expect(sink.envelopes[0].type).toBe('enqueue')

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.params).toEqual({ items: [{ id: 1 }, { id: 2 }, { id: 3 }] })
    })

    it('manual flush emits partial batch', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 10, target: 'worker' })

      await batcher.add({ id: 1 })
      await batcher.add({ id: 2 })
      await batcher.flush()

      expect(sink.envelopes).toHaveLength(1)
      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.params).toEqual({ items: [{ id: 1 }, { id: 2 }] })
    })

    it('flush on empty buffer is a no-op', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 5, target: 'worker' })

      await batcher.flush()

      expect(sink.envelopes).toHaveLength(0)
    })

    it('allows continued use after flush', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 10, target: 'worker' })

      await batcher.add({ id: 1 })
      await batcher.flush()
      await batcher.add({ id: 2 })
      await batcher.flush()

      expect(sink.envelopes).toHaveLength(2)
      const first = sink.envelopes[0].payload as EnqueuePayload
      const second = sink.envelopes[1].payload as EnqueuePayload
      expect(first.params).toEqual({ items: [{ id: 1 }] })
      expect(second.params).toEqual({ items: [{ id: 2 }] })
    })

    it('allows continued use after auto-flush', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 2, target: 'worker' })

      await batcher.add({ id: 1 })
      await batcher.add({ id: 2 }) // triggers auto-flush
      await batcher.add({ id: 3 })
      await batcher.flush()

      expect(sink.envelopes).toHaveLength(2)
      const first = sink.envelopes[0].payload as EnqueuePayload
      const second = sink.envelopes[1].payload as EnqueuePayload
      expect(first.params).toEqual({ items: [{ id: 1 }, { id: 2 }] })
      expect(second.params).toEqual({ items: [{ id: 3 }] })
    })
  })

  // ── Pending counter ──────────────────────────────────────────────

  describe('pending counter', () => {
    it('starts at 0', () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 5, target: 'worker' })

      expect(batcher.pending).toBe(0)
    })

    it('increments on add', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 5, target: 'worker' })

      await batcher.add({ id: 1 })
      expect(batcher.pending).toBe(1)

      await batcher.add({ id: 2 })
      expect(batcher.pending).toBe(2)
    })

    it('resets to 0 after manual flush', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 5, target: 'worker' })

      await batcher.add({ id: 1 })
      await batcher.add({ id: 2 })
      await batcher.flush()

      expect(batcher.pending).toBe(0)
    })

    it('resets to 0 after auto-flush', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 2, target: 'worker' })

      await batcher.add({ id: 1 })
      await batcher.add({ id: 2 })

      expect(batcher.pending).toBe(0)
    })
  })

  // ── Options propagation ──────────────────────────────────────────

  describe('options propagation', () => {
    it('propagates target to enqueue event', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 1, target: 'detail-worker' })

      await batcher.add({ url: '/page' })

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.target).toBe('detail-worker')
    })

    it('includes source when provided', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, {
        size: 1,
        target: 'worker',
        source: 'my-source'
      })

      await batcher.add({ id: 1 })

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.source).toBe('my-source')
    })

    it('includes category when provided', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, {
        size: 1,
        target: 'worker',
        category: 'premium'
      })

      await batcher.add({ id: 1 })

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.category).toBe('premium')
    })

    it('includes both source and category when provided', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, {
        size: 1,
        target: 'worker',
        source: 'alt-source',
        category: 'special'
      })

      await batcher.add({ id: 1 })

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.source).toBe('alt-source')
      expect(payload.category).toBe('special')
    })

    it('omits source and category when not provided', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 1, target: 'worker' })

      await batcher.add({ id: 1 })

      const payload = sink.envelopes[0].payload as Record<string, unknown>
      expect(payload).not.toHaveProperty('source')
      expect(payload).not.toHaveProperty('category')
    })
  })

  // ── Type safety ──────────────────────────────────────────────────

  describe('type safety', () => {
    it('works with typed items', async () => {
      type Product = { sku: string; price: number }
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher<Product>(emit, { size: 2, target: 'ingest' })

      await batcher.add({ sku: 'A', price: 10 })
      await batcher.add({ sku: 'B', price: 20 })

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.params).toEqual({
        items: [
          { sku: 'A', price: 10 },
          { sku: 'B', price: 20 }
        ]
      })
    })

    it('works with default Record<string, unknown>', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 1, target: 'worker' })

      await batcher.add({ arbitrary: 'data', count: 42 })

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect(payload.params).toEqual({ items: [{ arbitrary: 'data', count: 42 }] })
    })
  })

  // ── Terminal events ──────────────────────────────────────────────

  describe('terminal events', () => {
    it('flush after runComplete throws TerminalEventError', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 10, target: 'worker' })

      await batcher.add({ id: 1 })
      await emit.runComplete()

      await expect(batcher.flush()).rejects.toThrow(TerminalEventError)
    })

    it('add triggering auto-flush after runComplete throws TerminalEventError', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 2, target: 'worker' })

      await batcher.add({ id: 1 })
      await emit.runComplete()

      // Second add fills buffer → triggers auto-flush → TerminalEventError
      await expect(batcher.add({ id: 2 })).rejects.toThrow(TerminalEventError)
    })

    it('add below threshold after runComplete succeeds (no emit)', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 10, target: 'worker' })

      await emit.runComplete()

      // Adding without triggering auto-flush doesn't touch emit
      await batcher.add({ id: 1 })
      expect(batcher.pending).toBe(1)
    })
  })

  // ── Sink failures ────────────────────────────────────────────────

  describe('sink failures', () => {
    it('propagates sink failure on flush', async () => {
      const failSink = new FakeSink({ failOnEventWrite: 1, failureError: new Error('write fail') })
      const emit = createEmitAPI(run, failSink)
      const batcher = createBatcher(emit, { size: 10, target: 'worker' })

      await batcher.add({ id: 1 })

      await expect(batcher.flush()).rejects.toThrow('write fail')
    })

    it('propagates sink failure on auto-flush', async () => {
      const failSink = new FakeSink({ failOnEventWrite: 1, failureError: new Error('auto fail') })
      const emit = createEmitAPI(run, failSink)
      const batcher = createBatcher(emit, { size: 2, target: 'worker' })

      await batcher.add({ id: 1 })
      await expect(batcher.add({ id: 2 })).rejects.toThrow('auto fail')
    })

    it('subsequent flush after sink failure throws SinkFailedError', async () => {
      const failSink = new FakeSink({ failOnEventWrite: 1, failureError: new Error('boom') })
      const emit = createEmitAPI(run, failSink)
      const batcher = createBatcher(emit, { size: 10, target: 'worker' })

      await batcher.add({ id: 1 })
      await expect(batcher.flush()).rejects.toThrow('boom')

      // Failed batch is lost (fail-fast: sink is poisoned, items undeliverable)
      await batcher.add({ id: 2 })
      await expect(batcher.flush()).rejects.toThrow(SinkFailedError)
    })
  })

  // ── Edge cases ───────────────────────────────────────────────────

  describe('edge cases', () => {
    it('size=1 flushes on every add', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 1, target: 'worker' })

      await batcher.add({ id: 1 })
      await batcher.add({ id: 2 })
      await batcher.add({ id: 3 })

      expect(sink.envelopes).toHaveLength(3)
      for (let i = 0; i < 3; i++) {
        const payload = sink.envelopes[i].payload as EnqueuePayload
        expect(payload.params).toEqual({ items: [{ id: i + 1 }] })
      }
    })

    it('large batch size collects all items until flush', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 10_000, target: 'worker' })

      for (let i = 0; i < 50; i++) {
        await batcher.add({ i })
      }
      expect(sink.envelopes).toHaveLength(0)
      expect(batcher.pending).toBe(50)

      await batcher.flush()
      expect(sink.envelopes).toHaveLength(1)

      const payload = sink.envelopes[0].payload as EnqueuePayload
      expect((payload.params.items as unknown[]).length).toBe(50)
    })

    it('multiple empty flushes are no-ops', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 5, target: 'worker' })

      await batcher.flush()
      await batcher.flush()
      await batcher.flush()

      expect(sink.envelopes).toHaveLength(0)
    })

    it('size < 1 throws RangeError at construction', () => {
      const emit = createEmitAPI(run, sink)

      expect(() => createBatcher(emit, { size: 0, target: 'worker' })).toThrow(RangeError)
      expect(() => createBatcher(emit, { size: -1, target: 'worker' })).toThrow(RangeError)
    })

    it('fractional size < 1 throws RangeError', () => {
      const emit = createEmitAPI(run, sink)

      expect(() => createBatcher(emit, { size: 0.5, target: 'worker' })).toThrow(RangeError)
    })

    it('NaN size throws RangeError', () => {
      const emit = createEmitAPI(run, sink)

      expect(() => createBatcher(emit, { size: NaN, target: 'worker' })).toThrow(RangeError)
    })

    it('Infinity size throws RangeError', () => {
      const emit = createEmitAPI(run, sink)

      expect(() => createBatcher(emit, { size: Infinity, target: 'worker' })).toThrow(RangeError)
    })

    it('-Infinity size throws RangeError', () => {
      const emit = createEmitAPI(run, sink)

      expect(() => createBatcher(emit, { size: -Infinity, target: 'worker' })).toThrow(RangeError)
    })

    it('fractional size > 1 throws RangeError', () => {
      const emit = createEmitAPI(run, sink)

      expect(() => createBatcher(emit, { size: 1.5, target: 'worker' })).toThrow(RangeError)
      expect(() => createBatcher(emit, { size: 2.7, target: 'worker' })).toThrow(RangeError)
    })
  })

  // ── Realistic usage ──────────────────────────────────────────────

  describe('realistic usage', () => {
    it('120 items at batch size 50 produces 3 enqueues (50+50+20)', async () => {
      const emit = createEmitAPI(run, sink)
      const batcher = createBatcher(emit, { size: 50, target: 'detail-worker' })

      for (let i = 0; i < 120; i++) {
        await batcher.add({ index: i })
      }
      // Two auto-flushes at 50 and 100
      expect(sink.envelopes).toHaveLength(2)
      expect(batcher.pending).toBe(20)

      await batcher.flush()
      expect(sink.envelopes).toHaveLength(3)

      const counts = sink.envelopes.map(
        (e) => ((e.payload as EnqueuePayload).params.items as unknown[]).length
      )
      expect(counts).toEqual([50, 50, 20])
    })
  })
})
