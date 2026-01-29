/**
 * Type-level contract tests for EventEnvelope types.
 *
 * Goal: Ensure envelope types enforce correct structure.
 */
import { expectType, expectError, expectAssignable } from 'tsd'
import type {
  EventEnvelope,
  EventType,
  PayloadMap,
  ItemPayload,
  ArtifactPayload,
  LogPayload,
  RunErrorPayload,
  RunCompletePayload,
  ContractVersion,
  EventId,
  RunId,
  JobId,
  ArtifactId,
  CheckpointId,
  LogLevel
} from '../../../src/types/events'

// ============================================
// EventType is a union of all event types
// ============================================

expectAssignable<EventType>('item')
expectAssignable<EventType>('artifact')
expectAssignable<EventType>('checkpoint')
expectAssignable<EventType>('enqueue')
expectAssignable<EventType>('rotate_proxy')
expectAssignable<EventType>('log')
expectAssignable<EventType>('run_error')
expectAssignable<EventType>('run_complete')

// @ts-expect-error - invalid event type
expectError<EventType>('invalid_event')

// ============================================
// PayloadMap maps event types to payloads
// ============================================

expectType<ItemPayload>({} as PayloadMap['item'])
expectType<ArtifactPayload>({} as PayloadMap['artifact'])
expectType<LogPayload>({} as PayloadMap['log'])
expectType<RunErrorPayload>({} as PayloadMap['run_error'])
expectType<RunCompletePayload>({} as PayloadMap['run_complete'])

// ============================================
// EventEnvelope is generic over EventType
// ============================================

declare const itemEnvelope: EventEnvelope<'item'>
expectType<'item'>(itemEnvelope.type)
expectType<ItemPayload>(itemEnvelope.payload)

declare const logEnvelope: EventEnvelope<'log'>
expectType<'log'>(logEnvelope.type)
expectType<LogPayload>(logEnvelope.payload)

// ============================================
// Envelope base fields are present
// ============================================

declare const anyEnvelope: EventEnvelope
expectType<ContractVersion>(anyEnvelope.contract_version)
expectType<EventId>(anyEnvelope.event_id)
expectType<RunId>(anyEnvelope.run_id)
expectType<number>(anyEnvelope.seq)
expectType<string>(anyEnvelope.ts)
expectType<number>(anyEnvelope.attempt)

// Optional fields
expectType<JobId | undefined>(anyEnvelope.job_id)
expectType<RunId | undefined>(anyEnvelope.parent_run_id)

// ============================================
// Envelope fields are readonly
// ============================================

declare const envelope: EventEnvelope<'item'>
// @ts-expect-error - contract_version is readonly
envelope.contract_version = '0.2.0'
// @ts-expect-error - event_id is readonly
envelope.event_id = 'new-id' as EventId
// @ts-expect-error - run_id is readonly
envelope.run_id = 'new-run' as RunId
// @ts-expect-error - seq is readonly
envelope.seq = 2
// @ts-expect-error - ts is readonly
envelope.ts = 'new-ts'
// @ts-expect-error - type is readonly
envelope.type = 'log'
// @ts-expect-error - payload is readonly
envelope.payload = { item_type: 'new', data: {} }
// @ts-expect-error - attempt is readonly
envelope.attempt = 2

// ============================================
// Branded types
// ============================================

// Branded types are strings with a brand
expectAssignable<string>({} as RunId)
expectAssignable<string>({} as EventId)
expectAssignable<string>({} as JobId)
expectAssignable<string>({} as ArtifactId)
expectAssignable<string>({} as CheckpointId)

// But plain strings cannot be assigned to branded types without casting
declare const plainString: string
// @ts-expect-error - string is not assignable to RunId
const runId: RunId = plainString
// @ts-expect-error - string is not assignable to EventId
const eventId: EventId = plainString

// ============================================
// LogLevel is a union
// ============================================

expectAssignable<LogLevel>('debug')
expectAssignable<LogLevel>('info')
expectAssignable<LogLevel>('warn')
expectAssignable<LogLevel>('error')

// @ts-expect-error - invalid log level
expectError<LogLevel>('trace')
// @ts-expect-error - invalid log level
expectError<LogLevel>('fatal')
