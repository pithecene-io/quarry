/**
 * Unit tests for emit.runComplete() envelope correctness.
 *
 * Goal: Validate envelope construction for runComplete emits.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 * - Tests only envelope construction, not terminal semantics
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import { createRunMeta, FakeSink, validateEnvelope } from '../_harness'

describe('emit.runComplete() envelope correctness', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('creates envelope with type === "run_complete"', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    expect(sink.envelopes[0].type).toBe('run_complete')
  })

  it('works with no arguments', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    expect(sink.envelopes[0].payload).toEqual({})
  })

  it('works with empty options', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete({})

    expect(sink.envelopes[0].payload).toEqual({})
  })

  it('includes summary in payload when provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete({ summary: { items_collected: 42, pages_visited: 10 } })

    expect(sink.envelopes[0].payload).toMatchObject({
      summary: { items_collected: 42, pages_visited: 10 }
    })
  })

  it('omits summary from payload when not provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    expect(sink.envelopes[0].payload).not.toHaveProperty('summary')
  })

  it('works with complex summary', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete({
      summary: {
        stats: { total: 100, success: 95, failed: 5 },
        artifacts: ['screenshot-1', 'screenshot-2'],
        metadata: { duration_ms: 5000 }
      }
    })

    expect(sink.envelopes[0].payload).toMatchObject({
      summary: {
        stats: { total: 100, success: 95, failed: 5 },
        artifacts: ['screenshot-1', 'screenshot-2'],
        metadata: { duration_ms: 5000 }
      }
    })
  })

  it('passes full envelope validation with summary', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete({ summary: { count: 1 } })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })

  it('passes full envelope validation without summary', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runComplete()

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })
})
