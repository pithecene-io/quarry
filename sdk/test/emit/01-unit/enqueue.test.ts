/**
 * Unit tests for emit.enqueue() envelope correctness.
 *
 * Goal: Validate envelope construction for single enqueue emits.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 * - No terminal events
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import { createRunMeta, FakeSink, validateEnvelope } from '../_harness'

describe('emit.enqueue() envelope correctness', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('creates envelope with type === "enqueue"', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.enqueue({ target: 'next-page', params: { page: 2 } })

    expect(sink.envelopes[0].type).toBe('enqueue')
  })

  it('includes target in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.enqueue({ target: 'product-detail', params: {} })

    expect(sink.envelopes[0].payload).toMatchObject({ target: 'product-detail' })
  })

  it('includes params in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.enqueue({
      target: 'search',
      params: { query: 'widgets', page: 1, filters: ['new'] }
    })

    expect(sink.envelopes[0].payload).toMatchObject({
      target: 'search',
      params: { query: 'widgets', page: 1, filters: ['new'] }
    })
  })

  it('works with empty params', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.enqueue({ target: 'home', params: {} })

    expect(sink.envelopes[0].payload).toMatchObject({ params: {} })
  })

  it('works with nested params', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.enqueue({
      target: 'complex',
      params: {
        nested: { deep: { value: 42 } },
        array: [1, 2, 3]
      }
    })

    expect(sink.envelopes[0].payload).toMatchObject({
      params: {
        nested: { deep: { value: 42 } },
        array: [1, 2, 3]
      }
    })
  })

  it('passes full envelope validation', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.enqueue({ target: 'next', params: { id: 123 } })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })
})
