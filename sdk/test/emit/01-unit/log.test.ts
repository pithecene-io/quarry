/**
 * Unit tests for emit.log() envelope correctness.
 *
 * Goal: Validate envelope construction for single log emits.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 * - No terminal events
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import { CONTRACT_VERSION } from '../../../src/types/events'
import { FakeSink, createRunMeta, validateEnvelope } from '../_harness'

describe('emit.log() envelope correctness', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('creates envelope with type === "log"', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.log({ level: 'info', message: 'test message' })

    expect(sink.envelopes[0].type).toBe('log')
  })

  it('creates envelope with level in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.log({ level: 'warn', message: 'warning' })

    expect(sink.envelopes[0].payload).toMatchObject({ level: 'warn' })
  })

  it('creates envelope with message in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.log({ level: 'info', message: 'hello world' })

    expect(sink.envelopes[0].payload).toMatchObject({ message: 'hello world' })
  })

  it('creates envelope with optional fields in payload', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.log({
      level: 'debug',
      message: 'debugging',
      fields: { user_id: 123, action: 'click' }
    })

    expect(sink.envelopes[0].payload).toMatchObject({
      level: 'debug',
      message: 'debugging',
      fields: { user_id: 123, action: 'click' }
    })
  })

  it('creates envelope without fields when not provided', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.log({ level: 'info', message: 'no fields' })

    expect(sink.envelopes[0].payload).not.toHaveProperty('fields')
  })

  it('passes full envelope validation for all log levels', async () => {
    const emit = createEmitAPI(run, sink)

    await emit.log({ level: 'debug', message: 'debug' })
    await emit.log({ level: 'info', message: 'info' })
    await emit.log({ level: 'warn', message: 'warn' })
    await emit.log({ level: 'error', message: 'error' })

    for (const envelope of sink.envelopes) {
      const errors = validateEnvelope(envelope)
      expect(errors).toEqual([])
    }
  })

  describe('convenience methods', () => {
    it('emit.debug() creates log with level "debug"', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.debug('debug message')

      expect(sink.envelopes[0].type).toBe('log')
      expect(sink.envelopes[0].payload).toMatchObject({
        level: 'debug',
        message: 'debug message'
      })
    })

    it('emit.info() creates log with level "info"', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.info('info message')

      expect(sink.envelopes[0].payload).toMatchObject({
        level: 'info',
        message: 'info message'
      })
    })

    it('emit.warn() creates log with level "warn"', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.warn('warn message')

      expect(sink.envelopes[0].payload).toMatchObject({
        level: 'warn',
        message: 'warn message'
      })
    })

    it('emit.error() creates log with level "error"', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.error('error message')

      expect(sink.envelopes[0].payload).toMatchObject({
        level: 'error',
        message: 'error message'
      })
    })

    it('convenience methods accept optional fields', async () => {
      const emit = createEmitAPI(run, sink)

      await emit.info('with fields', { key: 'value' })

      expect(sink.envelopes[0].payload).toMatchObject({
        level: 'info',
        message: 'with fields',
        fields: { key: 'value' }
      })
    })
  })
})
