/**
 * Unit tests for emit.rotateProxy() envelope correctness.
 *
 * Goal: Validate envelope construction for single rotateProxy emits.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 * - No terminal events
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import { createRunMeta, FakeSink, validateEnvelope } from '../_harness'

describe('emit.rotateProxy() envelope correctness', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('creates envelope with type === "rotate_proxy"', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.rotateProxy()

    expect(sink.envelopes[0].type).toBe('rotate_proxy')
  })

  it('works with no arguments', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.rotateProxy()

    expect(sink.envelopes[0].payload).toEqual({})
  })

  it('works with empty options', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.rotateProxy({})

    expect(sink.envelopes[0].payload).toEqual({})
  })

  it('includes reason in payload when provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.rotateProxy({ reason: 'rate-limited' })

    expect(sink.envelopes[0].payload).toMatchObject({ reason: 'rate-limited' })
  })

  it('omits reason from payload when not provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.rotateProxy()

    expect(sink.envelopes[0].payload).not.toHaveProperty('reason')
  })

  it('passes full envelope validation with reason', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.rotateProxy({ reason: 'blocked' })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })

  it('passes full envelope validation without reason', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.rotateProxy()

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })
})
