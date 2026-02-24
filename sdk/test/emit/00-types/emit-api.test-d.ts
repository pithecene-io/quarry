/**
 * Type-level contract tests for EmitAPI.
 *
 * Goal: Prevent invalid Emit usage from compiling.
 * Invariant: If it compiles, it is a valid Emit call.
 *
 * Uses tsd for compile-time type assertions.
 */
import { expectAssignable, expectType } from 'tsd'
import type { EmitAPI } from '../../../src/emit'
import type { ArtifactId, CheckpointId } from '../../../src/types/events'

declare const emit: EmitAPI

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

// checkpoint note is optional
expectType<Promise<void>>(emit.checkpoint({ checkpoint_id: 'cp-1' as CheckpointId }))
expectType<Promise<void>>(
  emit.checkpoint({ checkpoint_id: 'cp-1' as CheckpointId, note: 'progress' })
)

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

// Buffer is accepted
expectType<Promise<ArtifactId>>(
  emit.artifact({
    name: 'test',
    content_type: 'application/octet-stream',
    data: Buffer.from([1, 2, 3])
  })
)

// Uint8Array is accepted
expectType<Promise<ArtifactId>>(
  emit.artifact({
    name: 'test',
    content_type: 'application/octet-stream',
    data: new Uint8Array([1, 2, 3])
  })
)

expectAssignable<{
  readonly item: (options: { item_type: string; data: Record<string, unknown> }) => Promise<void>
}>(emit)
