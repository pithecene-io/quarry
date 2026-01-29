/**
 * Unit tests for emit.checkpoint() envelope correctness.
 *
 * Goal: Validate envelope construction for single checkpoint emits.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 * - No terminal events
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import { FakeSink, createRunMeta, validateEnvelope } from '../_harness'
import type { CheckpointId } from '../../../src/types/events'

describe('emit.checkpoint() envelope correctness', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('creates envelope with type === "checkpoint"', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.checkpoint({ checkpoint_id: 'cp-1' as CheckpointId })

    expect(sink.envelopes[0].type).toBe('checkpoint')
  })

  it('includes checkpoint_id in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.checkpoint({ checkpoint_id: 'login-complete' as CheckpointId })

    expect(sink.envelopes[0].payload).toMatchObject({ checkpoint_id: 'login-complete' })
  })

  it('includes note in payload when provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.checkpoint({
      checkpoint_id: 'page-loaded' as CheckpointId,
      note: 'Successfully loaded product page'
    })

    expect(sink.envelopes[0].payload).toMatchObject({
      checkpoint_id: 'page-loaded',
      note: 'Successfully loaded product page'
    })
  })

  it('omits note from payload when not provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.checkpoint({ checkpoint_id: 'no-note' as CheckpointId })

    expect(sink.envelopes[0].payload).not.toHaveProperty('note')
  })

  it('passes full envelope validation', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.checkpoint({
      checkpoint_id: 'test-cp' as CheckpointId,
      note: 'Test checkpoint'
    })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })

  it('passes validation without note', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.checkpoint({ checkpoint_id: 'simple' as CheckpointId })

    const errors = validateEnvelope(sink.envelopes[0])
    expect(errors).toEqual([])
  })
})
