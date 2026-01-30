/**
 * Unit tests for emit.item() envelope correctness.
 *
 * Goal: Validate envelope construction for single item emits.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 * - No terminal events
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import type { JobId, RunId } from '../../../src/types/events'
import { CONTRACT_VERSION } from '../../../src/types/events'
import { createRunMeta, FakeSink, validateEnvelope } from '../_harness'

describe('emit.item() envelope correctness', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('creates envelope with contract_version', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: { name: 'test' } })

    expect(sink.envelopes).toHaveLength(1)
    expect(sink.envelopes[0].contract_version).toBe(CONTRACT_VERSION)
  })

  it('creates envelope with event_id', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].event_id).toBeDefined()
    expect(typeof sink.envelopes[0].event_id).toBe('string')
    expect(sink.envelopes[0].event_id.length).toBeGreaterThan(0)
  })

  it('creates envelope with run_id from RunMeta', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].run_id).toBe(run.run_id)
  })

  it('creates envelope with job_id when present in RunMeta', async () => {
    const runWithJob = createRunMeta({ job_id: 'job-123' as JobId })
    const emit = createEmitAPI(runWithJob, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].job_id).toBe('job-123')
  })

  it('creates envelope without job_id when not in RunMeta', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].job_id).toBeUndefined()
  })

  it('creates envelope with parent_run_id when present in RunMeta', async () => {
    const runWithParent = createRunMeta({ parent_run_id: 'parent-123' as RunId })
    const emit = createEmitAPI(runWithParent, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].parent_run_id).toBe('parent-123')
  })

  it('creates envelope without parent_run_id when not in RunMeta', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].parent_run_id).toBeUndefined()
  })

  it('creates envelope with attempt from RunMeta', async () => {
    const runWithAttempt = createRunMeta({ attempt: 3 })
    const emit = createEmitAPI(runWithAttempt, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].attempt).toBe(3)
  })

  it('creates envelope with ts as ISO string', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: {} })

    const ts = sink.envelopes[0].ts
    expect(typeof ts).toBe('string')
    expect(ts).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/)
    expect(new Date(ts).toISOString()).toBe(ts)
  })

  it('creates envelope with seq === 1 for first persisted event', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].seq).toBe(1)
  })

  it('creates envelope with type === "item"', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: {} })

    expect(sink.envelopes[0].type).toBe('item')
  })

  it('creates envelope with correct payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'product', data: { name: 'Widget', price: 9.99 } })

    const payload = sink.envelopes[0].payload
    expect(payload).toEqual({
      item_type: 'product',
      data: { name: 'Widget', price: 9.99 }
    })
  })

  it('passes full envelope validation', async () => {
    const runWithAll = createRunMeta({
      job_id: 'job-456' as JobId,
      parent_run_id: 'parent-789' as RunId,
      attempt: 2
    })
    const emit = createEmitAPI(runWithAll, sink)

    await emit.item({ item_type: 'product', data: { id: 1 } })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })

  it('generates unique event_id for each emit', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'a', data: {} })
    await emit.item({ item_type: 'b', data: {} })

    expect(sink.envelopes[0].event_id).not.toBe(sink.envelopes[1].event_id)
  })
})
