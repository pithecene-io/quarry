/**
 * Event envelope and payload types per CONTRACT_EMIT.md
 */

export const CONTRACT_VERSION = '0.4.1' as const
export type ContractVersion = typeof CONTRACT_VERSION

// ============================================
// Branded ID Types
// ============================================

/** Branded type for run identifiers */
export type RunId = string & { readonly __brand: 'RunId' }

/** Branded type for event identifiers */
export type EventId = string & { readonly __brand: 'EventId' }

/** Branded type for job identifiers */
export type JobId = string & { readonly __brand: 'JobId' }

/** Branded type for artifact identifiers */
export type ArtifactId = string & { readonly __brand: 'ArtifactId' }

/** Branded type for checkpoint identifiers */
export type CheckpointId = string & { readonly __brand: 'CheckpointId' }

/**
 * All supported event types from CONTRACT_EMIT.md
 *
 * Note: 'artifact_chunk' is reserved for IPC framing (CONTRACT_IPC.md)
 * and must never be added to this union.
 */
export type EventType =
  | 'item'
  | 'artifact'
  | 'checkpoint'
  | 'enqueue'
  | 'rotate_proxy'
  | 'log'
  | 'run_error'
  | 'run_complete'

/**
 * Log levels for the log event type.
 */
export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

// ============================================
// Payload Types (one per event type)
// ============================================

/**
 * Payload for 'item' events.
 * Represents a durable, structured output record.
 */
export interface ItemPayload {
  /** Caller-defined type label */
  item_type: string
  /** The record payload */
  data: Record<string, unknown>
}

/**
 * Payload for 'artifact' events.
 * Represents a binary or large payload.
 * Note: Actual binary data is handled separately via chunking (IPC layer).
 */
export interface ArtifactPayload {
  /** Unique identifier for the artifact */
  artifact_id: ArtifactId
  /** Human-readable name */
  name: string
  /** MIME content type */
  content_type: string
  /** Total size in bytes */
  size_bytes: number
}

/**
 * Payload for 'checkpoint' events.
 * Represents an explicit script checkpoint.
 */
export interface CheckpointPayload {
  /** Unique identifier for the checkpoint */
  checkpoint_id: CheckpointId
  /** Optional human-readable note */
  note?: string
}

/**
 * Payload for 'enqueue' events.
 * Advisory suggestion to enqueue additional work.
 */
export interface EnqueuePayload {
  /** Target identifier for the work */
  target: string
  /** Parameters for the work */
  params: Record<string, unknown>
}

/**
 * Payload for 'rotate_proxy' events.
 * Advisory suggestion to rotate proxy/session identity.
 */
export interface RotateProxyPayload {
  /** Optional reason for the rotation request */
  reason?: string
}

/**
 * Payload for 'log' events.
 * Structured log event emitted by script.
 */
export interface LogPayload {
  /** Log level */
  level: LogLevel
  /** Log message */
  message: string
  /** Optional structured fields */
  fields?: Record<string, unknown>
}

/**
 * Payload for 'run_error' events.
 * Represents a script-level error that should terminate the run.
 */
export interface RunErrorPayload {
  /**
   * Error type/category.
   * Expected values include: script_error, timeout, blocked, abort, etc.
   * Not an exhaustive enum — new error types may be introduced.
   */
  error_type: string
  /** Error message */
  message: string
  /** Optional stack trace */
  stack?: string
}

/**
 * Payload for 'run_complete' events.
 * Represents normal completion of the script.
 */
export interface RunCompletePayload {
  /** Optional summary object */
  summary?: Record<string, unknown>
}

// ============================================
// Payload Type Map (for type inference)
// ============================================

/**
 * Maps event types to their payload types.
 * Used for type-safe payload handling.
 */
export interface PayloadMap {
  item: ItemPayload
  artifact: ArtifactPayload
  checkpoint: CheckpointPayload
  enqueue: EnqueuePayload
  rotate_proxy: RotateProxyPayload
  log: LogPayload
  run_error: RunErrorPayload
  run_complete: RunCompletePayload
}

// ============================================
// Event Envelope
// ============================================

/**
 * Base envelope fields present on all events.
 * From CONTRACT_EMIT.md.
 *
 * All fields are readonly — envelopes are immutable once constructed.
 */
export interface EventEnvelopeBase {
  /** Semantic version string for the emit contract */
  readonly contract_version: ContractVersion
  /** Unique ID for the event, scoped to a run */
  readonly event_id: EventId
  /** Canonical run identifier */
  readonly run_id: RunId
  /** Monotonic sequence number per run, starts at 1 */
  readonly seq: number
  /** Event timestamp in ISO 8601 UTC */
  readonly ts: string
  /** Job ID, included when known at emit-time */
  readonly job_id?: JobId
  /** Parent run ID, included when run is a retry or child run */
  readonly parent_run_id?: RunId
  /** Attempt number. Always present. Starts at 1. */
  readonly attempt: number
}

/**
 * Typed event envelope with specific event type and payload.
 */
export interface EventEnvelope<T extends EventType = EventType> extends EventEnvelopeBase {
  /** Event type identifier */
  readonly type: T
  /** Type-specific payload */
  readonly payload: PayloadMap[T]
}

/**
 * Union type of all possible event envelopes.
 */
export type AnyEventEnvelope = {
  [K in EventType]: EventEnvelope<K>
}[EventType]
