/**
 * Type-level contract tests for EventEnvelope types.
 *
 * Goal: Ensure envelope types enforce correct structure.
 */
import { expectAssignable, expectType } from 'tsd'
import type {
  ArtifactId,
  ArtifactPayload,
  CheckpointId,
  ContractVersion,
  EventEnvelope,
  EventId,
  EventType,
  ItemPayload,
  JobId,
  LogLevel,
  LogPayload,
  PayloadMap,
  RunCompletePayload,
  RunErrorPayload,
  RunId
} from '../../../src/types/events'

expectAssignable<EventType>('item')
expectAssignable<EventType>('artifact')
expectAssignable<EventType>('checkpoint')
expectAssignable<EventType>('enqueue')
expectAssignable<EventType>('rotate_proxy')
expectAssignable<EventType>('log')
expectAssignable<EventType>('run_error')
expectAssignable<EventType>('run_complete')

expectType<ItemPayload>({} as PayloadMap['item'])
expectType<ArtifactPayload>({} as PayloadMap['artifact'])
expectType<LogPayload>({} as PayloadMap['log'])
expectType<RunErrorPayload>({} as PayloadMap['run_error'])
expectType<RunCompletePayload>({} as PayloadMap['run_complete'])

declare const itemEnvelope: EventEnvelope<'item'>
expectType<'item'>(itemEnvelope.type)
expectType<ItemPayload>(itemEnvelope.payload)

declare const logEnvelope: EventEnvelope<'log'>
expectType<'log'>(logEnvelope.type)
expectType<LogPayload>(logEnvelope.payload)

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

expectAssignable<string>({} as RunId)
expectAssignable<string>({} as EventId)
expectAssignable<string>({} as JobId)
expectAssignable<string>({} as ArtifactId)
expectAssignable<string>({} as CheckpointId)

expectAssignable<LogLevel>('debug')
expectAssignable<LogLevel>('info')
expectAssignable<LogLevel>('warn')
expectAssignable<LogLevel>('error')
