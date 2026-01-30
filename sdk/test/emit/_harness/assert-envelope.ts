/**
 * Envelope assertion helpers for tests.
 *
 * These helpers check envelope structure invariants.
 * No test framework assertions — returns boolean or throws.
 */
import type { EventEnvelope, EventType, PayloadMap } from '../../../src/types/events'
import { CONTRACT_VERSION } from '../../../src/types/events'

/**
 * Validate that an envelope has all required base fields.
 * Returns an array of validation errors (empty if valid).
 */
export function validateEnvelopeBase(envelope: EventEnvelope): string[] {
  const errors: string[] = []

  if (envelope.contract_version !== CONTRACT_VERSION) {
    errors.push(
      `contract_version should be '${CONTRACT_VERSION}', got '${envelope.contract_version}'`
    )
  }

  if (typeof envelope.event_id !== 'string' || envelope.event_id.length === 0) {
    errors.push(`event_id should be a non-empty string, got '${envelope.event_id}'`)
  }

  if (typeof envelope.run_id !== 'string' || envelope.run_id.length === 0) {
    errors.push(`run_id should be a non-empty string, got '${envelope.run_id}'`)
  }

  if (typeof envelope.seq !== 'number' || envelope.seq < 1 || !Number.isInteger(envelope.seq)) {
    errors.push(`seq should be a positive integer, got '${envelope.seq}'`)
  }

  if (typeof envelope.ts !== 'string' || !isISODateString(envelope.ts)) {
    errors.push(`ts should be an ISO date string, got '${envelope.ts}'`)
  }

  if (
    typeof envelope.attempt !== 'number' ||
    envelope.attempt < 1 ||
    !Number.isInteger(envelope.attempt)
  ) {
    errors.push(`attempt should be a positive integer, got '${envelope.attempt}'`)
  }

  if (envelope.type === undefined || envelope.type === null) {
    errors.push('type should be defined')
  }

  if (envelope.payload === undefined || envelope.payload === null) {
    errors.push('payload should be defined')
  }

  return errors
}

/**
 * Check if a string is a valid ISO 8601 date string.
 */
export function isISODateString(s: string): boolean {
  const date = new Date(s)
  return !Number.isNaN(date.getTime()) && s.includes('T')
}

/**
 * Validate item payload structure.
 */
export function validateItemPayload(payload: PayloadMap['item']): string[] {
  const errors: string[] = []

  if (typeof payload.item_type !== 'string') {
    errors.push(`item_type should be a string, got '${typeof payload.item_type}'`)
  }

  if (typeof payload.data !== 'object' || payload.data === null) {
    errors.push('data should be an object')
  }

  return errors
}

/**
 * Validate artifact payload structure.
 */
export function validateArtifactPayload(payload: PayloadMap['artifact']): string[] {
  const errors: string[] = []

  if (typeof payload.artifact_id !== 'string' || payload.artifact_id.length === 0) {
    errors.push(`artifact_id should be a non-empty string, got '${payload.artifact_id}'`)
  }

  if (typeof payload.name !== 'string') {
    errors.push(`name should be a string, got '${typeof payload.name}'`)
  }

  if (typeof payload.content_type !== 'string') {
    errors.push(`content_type should be a string, got '${typeof payload.content_type}'`)
  }

  if (typeof payload.size_bytes !== 'number' || payload.size_bytes < 0) {
    errors.push(`size_bytes should be a non-negative number, got '${payload.size_bytes}'`)
  }

  return errors
}

/**
 * Validate checkpoint payload structure.
 */
export function validateCheckpointPayload(payload: PayloadMap['checkpoint']): string[] {
  const errors: string[] = []

  if (typeof payload.checkpoint_id !== 'string' || payload.checkpoint_id.length === 0) {
    errors.push(`checkpoint_id should be a non-empty string, got '${payload.checkpoint_id}'`)
  }

  if (payload.note !== undefined && typeof payload.note !== 'string') {
    errors.push(`note should be a string if present, got '${typeof payload.note}'`)
  }

  return errors
}

/**
 * Validate enqueue payload structure.
 */
export function validateEnqueuePayload(payload: PayloadMap['enqueue']): string[] {
  const errors: string[] = []

  if (typeof payload.target !== 'string') {
    errors.push(`target should be a string, got '${typeof payload.target}'`)
  }

  if (typeof payload.params !== 'object' || payload.params === null) {
    errors.push('params should be an object')
  }

  return errors
}

/**
 * Validate log payload structure.
 */
export function validateLogPayload(payload: PayloadMap['log']): string[] {
  const errors: string[] = []

  const validLevels = ['debug', 'info', 'warn', 'error']
  if (!validLevels.includes(payload.level)) {
    errors.push(`level should be one of ${validLevels.join(', ')}, got '${payload.level}'`)
  }

  if (typeof payload.message !== 'string') {
    errors.push(`message should be a string, got '${typeof payload.message}'`)
  }

  if (
    payload.fields !== undefined &&
    (typeof payload.fields !== 'object' || payload.fields === null)
  ) {
    errors.push('fields should be an object if present')
  }

  return errors
}

/**
 * Validate run_error payload structure.
 */
export function validateRunErrorPayload(payload: PayloadMap['run_error']): string[] {
  const errors: string[] = []

  if (typeof payload.error_type !== 'string') {
    errors.push(`error_type should be a string, got '${typeof payload.error_type}'`)
  }

  if (typeof payload.message !== 'string') {
    errors.push(`message should be a string, got '${typeof payload.message}'`)
  }

  if (payload.stack !== undefined && typeof payload.stack !== 'string') {
    errors.push(`stack should be a string if present, got '${typeof payload.stack}'`)
  }

  return errors
}

/**
 * Validate run_complete payload structure.
 */
export function validateRunCompletePayload(payload: PayloadMap['run_complete']): string[] {
  const errors: string[] = []

  if (
    payload.summary !== undefined &&
    (typeof payload.summary !== 'object' || payload.summary === null)
  ) {
    errors.push('summary should be an object if present')
  }

  return errors
}

/**
 * Validate rotate_proxy payload structure.
 */
export function validateRotateProxyPayload(payload: PayloadMap['rotate_proxy']): string[] {
  const errors: string[] = []

  if (payload.reason !== undefined && typeof payload.reason !== 'string') {
    errors.push(`reason should be a string if present, got '${typeof payload.reason}'`)
  }

  return errors
}

/**
 * Validate an envelope's payload matches its type.
 */
export function validatePayloadForType<T extends EventType>(
  type: T,
  payload: PayloadMap[T]
): string[] {
  switch (type) {
    case 'item':
      return validateItemPayload(payload as PayloadMap['item'])
    case 'artifact':
      return validateArtifactPayload(payload as PayloadMap['artifact'])
    case 'checkpoint':
      return validateCheckpointPayload(payload as PayloadMap['checkpoint'])
    case 'enqueue':
      return validateEnqueuePayload(payload as PayloadMap['enqueue'])
    case 'log':
      return validateLogPayload(payload as PayloadMap['log'])
    case 'run_error':
      return validateRunErrorPayload(payload as PayloadMap['run_error'])
    case 'run_complete':
      return validateRunCompletePayload(payload as PayloadMap['run_complete'])
    case 'rotate_proxy':
      return validateRotateProxyPayload(payload as PayloadMap['rotate_proxy'])
    default:
      return [`Unknown event type: ${type}`]
  }
}

/**
 * Fully validate an envelope (base + payload).
 */
export function validateEnvelope(envelope: EventEnvelope): string[] {
  const errors = validateEnvelopeBase(envelope)
  errors.push(...validatePayloadForType(envelope.type, envelope.payload))
  return errors
}
