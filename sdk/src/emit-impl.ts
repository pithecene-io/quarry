import { randomUUID } from 'node:crypto'
import type {
  EmitAPI,
  EmitArtifactOptions,
  EmitCheckpointOptions,
  EmitEnqueueOptions,
  EmitItemOptions,
  EmitLogOptions,
  EmitRotateProxyOptions,
  EmitRunCompleteOptions,
  EmitRunErrorOptions,
  EmitSink
} from './emit'
import type { RunMeta } from './types/context'
import type { ArtifactId, EventEnvelope, EventId, EventType, PayloadMap } from './types/events'
import { CONTRACT_VERSION } from './types/events'

/**
 * Error thrown when attempting to emit after a terminal event.
 */
export class TerminalEventError extends Error {
  constructor() {
    super('Cannot emit: a terminal event (run_error or run_complete) has already been emitted')
    this.name = 'TerminalEventError'
  }
}

/**
 * Error thrown when attempting to emit after a sink failure.
 */
export class SinkFailedError extends Error {
  constructor(cause: unknown) {
    super('Cannot emit: sink has previously failed')
    this.name = 'SinkFailedError'
    this.cause = cause
  }
}

/**
 * Create an EmitAPI implementation backed by an EmitSink.
 *
 * Emission is serialized via promise chain to ensure ordering and
 * correct terminal state handling. Fail-fast: after any sink failure,
 * subsequent emits will throw SinkFailedError.
 *
 * @internal
 */
export function createEmitAPI(run: RunMeta, sink: EmitSink): EmitAPI {
  let seq = 0
  let terminalEmitted = false
  let sinkFailed: unknown = null
  let chain = Promise.resolve()

  /**
   * Serialize an emit operation through the promise chain.
   * Ensures strict ordering and prevents concurrent interleaving.
   * Fail-fast: after first sink failure, all subsequent emits fail.
   */
  function serialize<T>(fn: () => Promise<T>): Promise<T> {
    const result = chain.then(async () => {
      if (sinkFailed !== null) {
        throw new SinkFailedError(sinkFailed)
      }
      try {
        return await fn()
      } catch (err) {
        sinkFailed = err
        throw err
      }
    })
    // Chain continues regardless to maintain serialization
    chain = result.then(
      () => {},
      () => {}
    )
    return result
  }

  /**
   * Create an event envelope without seq (assigned after successful write).
   * Sequence numbers represent persisted order.
   */
  function createEnvelope<T extends EventType>(
    type: T,
    payload: PayloadMap[T]
  ): Readonly<Omit<EventEnvelope<T>, 'seq'>> {
    return {
      contract_version: CONTRACT_VERSION,
      event_id: randomUUID() as EventId,
      run_id: run.run_id,
      type,
      ts: new Date().toISOString(),
      payload,
      attempt: run.attempt,
      ...(run.job_id !== undefined && { job_id: run.job_id }),
      ...(run.parent_run_id !== undefined && { parent_run_id: run.parent_run_id })
    }
  }

  /**
   * Assign seq and write envelope. Seq represents persisted order.
   */
  async function writeEnvelope<T extends EventType>(
    envelope: Readonly<Omit<EventEnvelope<T>, 'seq'>>
  ): Promise<void> {
    seq += 1
    const complete: Readonly<EventEnvelope<T>> = { ...envelope, seq } as Readonly<EventEnvelope<T>>
    await sink.writeEvent(complete)
  }

  function assertNotTerminal(): void {
    if (terminalEmitted) {
      throw new TerminalEventError()
    }
  }

  /**
   * Emit a non-terminal event: serialize → assert → envelope → write.
   */
  function emitEvent<T extends EventType>(type: T, payload: PayloadMap[T]): Promise<void> {
    return serialize(async () => {
      assertNotTerminal()
      const envelope = createEnvelope(type, payload)
      await writeEnvelope(envelope)
    })
  }

  const emit: EmitAPI = {
    item(options: EmitItemOptions): Promise<void> {
      return emitEvent('item', {
        item_type: options.item_type,
        data: options.data
      })
    },

    artifact(options: EmitArtifactOptions): Promise<ArtifactId> {
      return serialize(async () => {
        assertNotTerminal()
        const artifact_id = randomUUID() as ArtifactId
        const size_bytes = options.data.byteLength

        // Artifact bytes may be written before the artifact event.
        // The artifact event is the commit record.
        //
        // Error cases:
        // - Bytes fail → no event emitted, no orphan.
        // - Event fails after bytes → orphaned blob, eligible for GC.
        await sink.writeArtifactData(artifact_id, options.data)

        const envelope = createEnvelope('artifact', {
          artifact_id,
          name: options.name,
          content_type: options.content_type,
          size_bytes
        })
        await writeEnvelope(envelope)

        return artifact_id
      })
    },

    checkpoint(options: EmitCheckpointOptions): Promise<void> {
      return emitEvent('checkpoint', {
        checkpoint_id: options.checkpoint_id,
        ...(options.note !== undefined && { note: options.note })
      })
    },

    enqueue(options: EmitEnqueueOptions): Promise<void> {
      return emitEvent('enqueue', {
        target: options.target,
        params: options.params
      })
    },

    rotateProxy(options?: EmitRotateProxyOptions): Promise<void> {
      return emitEvent('rotate_proxy', {
        ...(options?.reason !== undefined && { reason: options.reason })
      })
    },

    log(options: EmitLogOptions): Promise<void> {
      return emitEvent('log', {
        level: options.level,
        message: options.message,
        ...(options.fields !== undefined && { fields: options.fields })
      })
    },

    async debug(message: string, fields?: Record<string, unknown>): Promise<void> {
      await emit.log({ level: 'debug', message, fields })
    },

    async info(message: string, fields?: Record<string, unknown>): Promise<void> {
      await emit.log({ level: 'info', message, fields })
    },

    async warn(message: string, fields?: Record<string, unknown>): Promise<void> {
      await emit.log({ level: 'warn', message, fields })
    },

    async error(message: string, fields?: Record<string, unknown>): Promise<void> {
      await emit.log({ level: 'error', message, fields })
    },

    runError(options: EmitRunErrorOptions): Promise<void> {
      return serialize(async () => {
        assertNotTerminal()
        const envelope = createEnvelope('run_error', {
          error_type: options.error_type,
          message: options.message,
          ...(options.stack !== undefined && { stack: options.stack })
        })
        await writeEnvelope(envelope)
        terminalEmitted = true
      })
    },

    runComplete(options?: EmitRunCompleteOptions): Promise<void> {
      return serialize(async () => {
        assertNotTerminal()
        const envelope = createEnvelope('run_complete', {
          ...(options?.summary !== undefined && { summary: options.summary })
        })
        await writeEnvelope(envelope)
        terminalEmitted = true
      })
    }
  }

  return emit
}
