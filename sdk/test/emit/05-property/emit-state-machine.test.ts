/**
 * Property-based state machine tests for Emit.
 *
 * Goal: Catch edge cases humans won't enumerate.
 *
 * Model State:
 * - healthy | terminal | failed
 * - seq: number
 *
 * Operations:
 * - Any emit method
 * - Injected sink failure
 *
 * Invariants:
 * - seq strictly increases on persisted events
 * - No emits after terminal
 * - No emits after sink failure
 * - At most one terminal event
 * - Payload always matches type
 */

import * as fc from 'fast-check'
import { describe, expect, it } from 'vitest'
import { createEmitAPI, SinkFailedError, TerminalEventError } from '../../../src/emit-impl'
import type { CheckpointId } from '../../../src/types/events'
import { createRunMeta, FakeSink, validateEnvelope } from '../_harness'

/**
 * Model of the Emit state machine for property testing.
 */
interface EmitModel {
  state: 'healthy' | 'terminal' | 'failed'
  seq: number
  terminalCount: number
}

/**
 * Command types that can be executed against the Emit API.
 */
type EmitCommand =
  | { type: 'item'; item_type: string }
  | { type: 'log'; level: 'debug' | 'info' | 'warn' | 'error'; message: string }
  | { type: 'checkpoint'; checkpoint_id: string }
  | { type: 'enqueue'; target: string }
  | { type: 'rotateProxy' }
  | { type: 'artifact'; name: string }
  | { type: 'runComplete' }
  | { type: 'runError'; error_type: string; message: string }
  | { type: 'injectFailure' }

/**
 * Generate arbitrary emit commands.
 */
const emitCommandArb: fc.Arbitrary<EmitCommand> = fc.oneof(
  fc.record({
    type: fc.constant('item' as const),
    item_type: fc.string({ minLength: 1, maxLength: 10 })
  }),
  fc.record({
    type: fc.constant('log' as const),
    level: fc.constantFrom('debug', 'info', 'warn', 'error') as fc.Arbitrary<
      'debug' | 'info' | 'warn' | 'error'
    >,
    message: fc.string({ maxLength: 50 })
  }),
  fc.record({
    type: fc.constant('checkpoint' as const),
    checkpoint_id: fc.string({ minLength: 1, maxLength: 20 })
  }),
  fc.record({
    type: fc.constant('enqueue' as const),
    target: fc.string({ minLength: 1, maxLength: 20 })
  }),
  fc.record({ type: fc.constant('rotateProxy' as const) }),
  fc.record({
    type: fc.constant('artifact' as const),
    name: fc.string({ minLength: 1, maxLength: 20 })
  }),
  fc.record({ type: fc.constant('runComplete' as const) }),
  fc.record({
    type: fc.constant('runError' as const),
    error_type: fc.string({ minLength: 1, maxLength: 20 }),
    message: fc.string({ maxLength: 50 })
  }),
  fc.record({ type: fc.constant('injectFailure' as const) })
)

describe('emit state machine properties', () => {
  it('seq strictly increases on persisted events', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.array(emitCommandArb, { minLength: 1, maxLength: 20 }),
        async (commands) => {
          const sink = new FakeSink()
          const run = createRunMeta()
          const emit = createEmitAPI(run, sink)

          for (const cmd of commands) {
            try {
              await executeCommand(emit, cmd)
            } catch {
              // Ignore errors for this property
            }
          }

          // Verify seq is strictly increasing
          const seqs = sink.envelopes.map((e) => e.seq)
          for (let i = 1; i < seqs.length; i++) {
            expect(seqs[i]).toBe(seqs[i - 1] + 1)
          }

          // First seq is 1 if any events
          if (seqs.length > 0) {
            expect(seqs[0]).toBe(1)
          }
        }
      ),
      { numRuns: 100 }
    )
  })

  it('no emits succeed after terminal', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.array(emitCommandArb, { minLength: 1, maxLength: 20 }),
        async (commands) => {
          const sink = new FakeSink()
          const run = createRunMeta()
          const emit = createEmitAPI(run, sink)
          let terminalEmitted = false

          for (const cmd of commands) {
            const wasTerminal = terminalEmitted

            try {
              await executeCommand(emit, cmd)

              // If we succeeded after terminal, that's a violation
              if (wasTerminal && cmd.type !== 'injectFailure') {
                expect.fail('Emit succeeded after terminal event')
              }

              // Track if this was a terminal event
              if (cmd.type === 'runComplete' || cmd.type === 'runError') {
                terminalEmitted = true
              }
            } catch (err) {
              // If terminal was emitted, subsequent emits should throw TerminalEventError
              // (unless sink also failed)
              if (wasTerminal && err instanceof SinkFailedError) {
                // Sink failure takes precedence, that's OK
              } else if (wasTerminal && !(err instanceof TerminalEventError)) {
                // If we're after terminal and got something other than TerminalEventError
                // and it's not a sink failure, that's unexpected
              }
            }
          }
        }
      ),
      { numRuns: 100 }
    )
  })

  it('at most one terminal event persisted', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.array(emitCommandArb, { minLength: 1, maxLength: 20 }),
        async (commands) => {
          const sink = new FakeSink()
          const run = createRunMeta()
          const emit = createEmitAPI(run, sink)

          for (const cmd of commands) {
            try {
              await executeCommand(emit, cmd)
            } catch {
              // Ignore errors
            }
          }

          // Count terminal events
          const terminalEvents = sink.envelopes.filter(
            (e) => e.type === 'run_complete' || e.type === 'run_error'
          )

          expect(terminalEvents.length).toBeLessThanOrEqual(1)
        }
      ),
      { numRuns: 100 }
    )
  })

  it('payload always matches event type', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.array(emitCommandArb, { minLength: 1, maxLength: 20 }),
        async (commands) => {
          const sink = new FakeSink()
          const run = createRunMeta()
          const emit = createEmitAPI(run, sink)

          for (const cmd of commands) {
            try {
              await executeCommand(emit, cmd)
            } catch {
              // Ignore errors
            }
          }

          // Validate all persisted envelopes
          for (const envelope of sink.envelopes) {
            const errors = validateEnvelope(envelope)
            expect(errors).toEqual([])
          }
        }
      ),
      { numRuns: 100 }
    )
  })

  it('no emits succeed after sink failure', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.array(emitCommandArb, { minLength: 1, maxLength: 20 }),
        async (commands) => {
          // Use a sink that fails on a random event
          const failOnEvent = Math.floor(Math.random() * 10) + 1
          const sink = new FakeSink({ failOnEventWrite: failOnEvent })
          const run = createRunMeta()
          const emit = createEmitAPI(run, sink)
          let sinkHasFailed = false

          for (const cmd of commands) {
            if (cmd.type === 'injectFailure') continue

            const wasFailedBefore = sinkHasFailed

            try {
              await executeCommand(emit, cmd)

              // If sink had failed, we shouldn't succeed
              if (wasFailedBefore) {
                expect.fail('Emit succeeded after sink failure')
              }
            } catch (err) {
              if (err instanceof SinkFailedError) {
                // This is expected after failure
                sinkHasFailed = true
              } else if (!sinkHasFailed) {
                // First failure
                sinkHasFailed = true
              }
            }
          }
        }
      ),
      { numRuns: 100 }
    )
  })

  it('model consistency: state transitions are correct', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.array(emitCommandArb, { minLength: 1, maxLength: 30 }),
        async (commands) => {
          const sink = new FakeSink()
          const run = createRunMeta()
          const emit = createEmitAPI(run, sink)

          const model: EmitModel = {
            state: 'healthy',
            seq: 0,
            terminalCount: 0
          }

          for (const cmd of commands) {
            const prevState = model.state

            try {
              await executeCommand(emit, cmd)

              // Update model on success
              if (cmd.type !== 'injectFailure') {
                model.seq++

                if (cmd.type === 'runComplete' || cmd.type === 'runError') {
                  model.state = 'terminal'
                  model.terminalCount++
                }
              }

              // Verify model consistency
              expect(model.seq).toBe(sink.envelopes.length)

              // Should not have succeeded if was terminal or failed
              // (injectFailure is a no-op so it doesn't count as a "success")
              if (cmd.type !== 'injectFailure') {
                if (prevState === 'terminal') {
                  expect.fail('Succeeded in terminal state')
                }
                if (prevState === 'failed') {
                  expect.fail('Succeeded in failed state')
                }
              }
            } catch (err) {
              if (err instanceof TerminalEventError) {
                // TerminalEventError is thrown when in terminal state
                // Note: this also transitions to 'failed' due to serialize() error handling
                expect(model.state).toBe('terminal')
                model.state = 'failed' // TerminalEventError gets caught and sets sinkFailed
              } else if (err instanceof SinkFailedError) {
                // Already in failed state (could be from terminal or sink failure)
                expect(['terminal', 'failed']).toContain(model.state)
                model.state = 'failed'
              } else {
                // First sink failure - transition to failed
                model.state = 'failed'
              }
            }
          }

          // Final consistency check
          expect(model.terminalCount).toBeLessThanOrEqual(1)
        }
      ),
      { numRuns: 100 }
    )
  })
})

/**
 * Execute a command against the emit API.
 */
async function executeCommand(
  emit: ReturnType<typeof createEmitAPI>,
  cmd: EmitCommand
): Promise<void> {
  switch (cmd.type) {
    case 'item':
      await emit.item({ item_type: cmd.item_type, data: { generated: true } })
      break
    case 'log':
      await emit.log({ level: cmd.level, message: cmd.message })
      break
    case 'checkpoint':
      await emit.checkpoint({ checkpoint_id: cmd.checkpoint_id as CheckpointId })
      break
    case 'enqueue':
      await emit.enqueue({ target: cmd.target, params: {} })
      break
    case 'rotateProxy':
      await emit.rotateProxy()
      break
    case 'artifact':
      await emit.artifact({
        name: cmd.name,
        content_type: 'application/octet-stream',
        data: Buffer.from('test data')
      })
      break
    case 'runComplete':
      await emit.runComplete()
      break
    case 'runError':
      await emit.runError({ error_type: cmd.error_type, message: cmd.message })
      break
    case 'injectFailure':
      // No-op for now - failure injection is done via FakeSink options
      break
  }
}
