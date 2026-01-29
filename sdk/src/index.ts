// Event types (from sdk/src/types/events.ts)
export type {
  ContractVersion,
  EventType,
  LogLevel,
  RunId,
  EventId,
  JobId,
  ArtifactId,
  CheckpointId,
  ItemPayload,
  ArtifactPayload,
  CheckpointPayload,
  EnqueuePayload,
  RotateProxyPayload,
  LogPayload,
  RunErrorPayload,
  RunCompletePayload,
  PayloadMap,
  EventEnvelopeBase,
  EventEnvelope,
  AnyEventEnvelope
} from './types/events'

export { CONTRACT_VERSION } from './types/events'

// Context types (from sdk/src/types/context.ts)
export type { RunMeta, QuarryContext, QuarryScript } from './types/context'

// Emit types
export type {
  EmitAPI,
  EmitItemOptions,
  EmitArtifactOptions,
  EmitCheckpointOptions,
  EmitEnqueueOptions,
  EmitRotateProxyOptions,
  EmitLogOptions,
  EmitRunErrorOptions,
  EmitRunCompleteOptions
} from './emit'

// Hook types
export type {
  BeforeRunHook,
  AfterRunHook,
  OnErrorHook,
  CleanupHook,
  QuarryHooks,
  QuarryScriptModule
} from './hooks'

// Errors (public — useful for user scripts to catch)
export { TerminalEventError } from './emit-impl'

// Internal exports for executor-node (@internal — not for user scripts)
export type { EmitSink } from './emit'
export type { CreateContextOptions } from './context'
export { createContext } from './context'
export { createEmitAPI } from './emit-impl'
