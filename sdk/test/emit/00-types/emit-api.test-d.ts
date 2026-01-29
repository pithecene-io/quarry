/**
 * Type-level contract tests for EmitAPI.
 *
 * Goal: Prevent invalid Emit usage from compiling.
 * Invariant: If it compiles, it is a valid Emit call.
 *
 * Uses tsd for compile-time type assertions.
 */
import { expectType, expectError, expectAssignable } from 'tsd'
import type { EmitAPI } from '../../../src/emit'
import type { ArtifactId, CheckpointId } from '../../../src/types/events'

declare const emit: EmitAPI

// ============================================
// EmitAPI method signatures
// ============================================

// item() returns Promise<void>
expectType<Promise<void>>(emit.item({ item_type: 'product', data: {} }))

// artifact() returns Promise<ArtifactId>
expectType<Promise<ArtifactId>>(
  emit.artifact({ name: 'screenshot.png', content_type: 'image/png', data: Buffer.from('') })
)

// checkpoint() returns Promise<void>
expectType<Promise<void>>(emit.checkpoint({ checkpoint_id: 'cp-1' as CheckpointId }))

// enqueue() returns Promise<void>
expectType<Promise<void>>(emit.enqueue({ target: 'next-page', params: {} }))

// rotateProxy() returns Promise<void>
expectType<Promise<void>>(emit.rotateProxy())
expectType<Promise<void>>(emit.rotateProxy({ reason: 'rate-limited' }))

// log() returns Promise<void>
expectType<Promise<void>>(emit.log({ level: 'info', message: 'test' }))

// Convenience log methods return Promise<void>
expectType<Promise<void>>(emit.debug('test'))
expectType<Promise<void>>(emit.info('test'))
expectType<Promise<void>>(emit.warn('test'))
expectType<Promise<void>>(emit.error('test'))

// runError() returns Promise<void>
expectType<Promise<void>>(emit.runError({ error_type: 'script_error', message: 'failed' }))

// runComplete() returns Promise<void>
expectType<Promise<void>>(emit.runComplete())
expectType<Promise<void>>(emit.runComplete({ summary: { items: 10 } }))

// ============================================
// Payload shape validation - required fields
// ============================================

// item() requires item_type and data
// @ts-expect-error - missing item_type
expectError(emit.item({ data: {} }))
// @ts-expect-error - missing data
expectError(emit.item({ item_type: 'product' }))

// artifact() requires name, content_type, and data
// @ts-expect-error - missing name
expectError(emit.artifact({ content_type: 'image/png', data: Buffer.from('') }))
// @ts-expect-error - missing content_type
expectError(emit.artifact({ name: 'test.png', data: Buffer.from('') }))
// @ts-expect-error - missing data
expectError(emit.artifact({ name: 'test.png', content_type: 'image/png' }))

// checkpoint() requires checkpoint_id
// @ts-expect-error - missing checkpoint_id
expectError(emit.checkpoint({}))

// enqueue() requires target and params
// @ts-expect-error - missing target
expectError(emit.enqueue({ params: {} }))
// @ts-expect-error - missing params
expectError(emit.enqueue({ target: 'next' }))

// log() requires level and message
// @ts-expect-error - missing level
expectError(emit.log({ message: 'test' }))
// @ts-expect-error - missing message
expectError(emit.log({ level: 'info' }))

// runError() requires error_type and message
// @ts-expect-error - missing error_type
expectError(emit.runError({ message: 'failed' }))
// @ts-expect-error - missing message
expectError(emit.runError({ error_type: 'script_error' }))

// ============================================
// Payload shape validation - type correctness
// ============================================

// item_type must be string
// @ts-expect-error - item_type must be string
expectError(emit.item({ item_type: 123, data: {} }))

// data must be Record<string, unknown>
// @ts-expect-error - data must be object
expectError(emit.item({ item_type: 'test', data: 'not-an-object' }))

// artifact data must be Buffer or Uint8Array
// @ts-expect-error - data must be Buffer or Uint8Array
expectError(emit.artifact({ name: 'test', content_type: 'text/plain', data: 'string-data' }))

// log level must be valid
// @ts-expect-error - invalid log level
expectError(emit.log({ level: 'invalid', message: 'test' }))

// ============================================
// Optional fields
// ============================================

// checkpoint note is optional
expectType<Promise<void>>(emit.checkpoint({ checkpoint_id: 'cp-1' as CheckpointId }))
expectType<Promise<void>>(emit.checkpoint({ checkpoint_id: 'cp-1' as CheckpointId, note: 'progress' }))

// log fields is optional
expectType<Promise<void>>(emit.log({ level: 'info', message: 'test' }))
expectType<Promise<void>>(emit.log({ level: 'info', message: 'test', fields: { key: 'value' } }))

// runError stack is optional
expectType<Promise<void>>(emit.runError({ error_type: 'error', message: 'msg' }))
expectType<Promise<void>>(emit.runError({ error_type: 'error', message: 'msg', stack: 'trace' }))

// runComplete summary is optional
expectType<Promise<void>>(emit.runComplete())
expectType<Promise<void>>(emit.runComplete({}))
expectType<Promise<void>>(emit.runComplete({ summary: { count: 1 } }))

// rotateProxy reason is optional
expectType<Promise<void>>(emit.rotateProxy())
expectType<Promise<void>>(emit.rotateProxy({}))
expectType<Promise<void>>(emit.rotateProxy({ reason: 'blocked' }))

// ============================================
// No extra fields allowed
// ============================================

// @ts-expect-error - extra field on item
expectError(emit.item({ item_type: 'test', data: {}, extra: true }))

// @ts-expect-error - extra field on artifact
expectError(emit.artifact({ name: 'test', content_type: 'text/plain', data: Buffer.from(''), extra: true }))

// @ts-expect-error - extra field on checkpoint
expectError(emit.checkpoint({ checkpoint_id: 'cp' as CheckpointId, extra: true }))

// @ts-expect-error - extra field on enqueue
expectError(emit.enqueue({ target: 'next', params: {}, extra: true }))

// @ts-expect-error - extra field on log
expectError(emit.log({ level: 'info', message: 'test', extra: true }))

// @ts-expect-error - extra field on runError
expectError(emit.runError({ error_type: 'error', message: 'msg', extra: true }))

// @ts-expect-error - extra field on runComplete
expectError(emit.runComplete({ summary: {}, extra: true }))

// @ts-expect-error - extra field on rotateProxy
expectError(emit.rotateProxy({ reason: 'test', extra: true }))

// ============================================
// Artifact data is byte-addressable
// ============================================

// Buffer is accepted
expectType<Promise<ArtifactId>>(
  emit.artifact({ name: 'test', content_type: 'application/octet-stream', data: Buffer.from([1, 2, 3]) })
)

// Uint8Array is accepted
expectType<Promise<ArtifactId>>(
  emit.artifact({ name: 'test', content_type: 'application/octet-stream', data: new Uint8Array([1, 2, 3]) })
)

// ============================================
// EmitAPI methods are readonly
// ============================================

expectAssignable<{ readonly item: (options: { item_type: string; data: Record<string, unknown> }) => Promise<void> }>(emit)
