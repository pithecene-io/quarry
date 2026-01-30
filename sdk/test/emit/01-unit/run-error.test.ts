/**
 * Unit tests for emit.runError() envelope correctness.
 *
 * Goal: Validate envelope construction for runError emits.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 * - Tests only envelope construction, not terminal semantics
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import { createRunMeta, FakeSink, validateEnvelope } from '../_harness'

describe('emit.runError() envelope correctness', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('creates envelope with type === "run_error"', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'script_error', message: 'Something went wrong' })

    expect(sink.envelopes[0].type).toBe('run_error')
  })

  it('includes error_type in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'timeout', message: 'Page load timeout' })

    expect(sink.envelopes[0].payload).toMatchObject({ error_type: 'timeout' })
  })

  it('includes message in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'blocked', message: 'Access denied by CAPTCHA' })

    expect(sink.envelopes[0].payload).toMatchObject({ message: 'Access denied by CAPTCHA' })
  })

  it('includes stack in payload when provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({
      error_type: 'script_error',
      message: 'TypeError: undefined',
      stack: 'TypeError: undefined\n    at Object.<anonymous>'
    })

    expect(sink.envelopes[0].payload).toMatchObject({
      stack: 'TypeError: undefined\n    at Object.<anonymous>'
    })
  })

  it('omits stack from payload when not provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'abort', message: 'Aborted by user' })

    expect(sink.envelopes[0].payload).not.toHaveProperty('stack')
  })

  it('works with various error types', async () => {
    const errorTypes = [
      'script_error',
      'timeout',
      'blocked',
      'abort',
      'network_error',
      'custom_error'
    ]

    for (const error_type of errorTypes) {
      sink.reset()
      const newEmit = createEmitAPI(createRunMeta(), sink)
      await newEmit.runError({ error_type, message: `Error: ${error_type}` })
      expect(sink.envelopes[0].payload).toMatchObject({ error_type })
    }
  })

  it('passes full envelope validation with stack', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({
      error_type: 'script_error',
      message: 'Error',
      stack: 'Error\n    at test'
    })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })

  it('passes full envelope validation without stack', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.runError({ error_type: 'timeout', message: 'Timed out' })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })
})
