// Event types (from sdk/src/types/events.ts)

export type { CreateContextOptions } from './context'
export { createContext } from './context'
// Emit types
// Internal exports for executor-node (@internal — not for user scripts)
export type {
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
// Errors (public — useful for user scripts to catch)
export { createEmitAPI, TerminalEventError } from './emit-impl'
// Hook types
export type {
  AfterRunHook,
  BeforeRunHook,
  CleanupHook,
  OnErrorHook,
  QuarryHooks,
  QuarryScriptModule
} from './hooks'
// Proxy validation (from sdk/src/proxy.ts)
export type { ProxyValidationError, ProxyValidationResult, ProxyValidationWarning } from './proxy'
export { redactProxyEndpoint, validateProxyEndpoint, validateProxyPool } from './proxy'
// Context types (from sdk/src/types/context.ts)
export type { QuarryContext, QuarryScript, RunMeta } from './types/context'
export type {
  AnyEventEnvelope,
  ArtifactId,
  ArtifactPayload,
  CheckpointId,
  CheckpointPayload,
  ContractVersion,
  EnqueuePayload,
  EventEnvelope,
  EventEnvelopeBase,
  EventId,
  EventType,
  ItemPayload,
  JobId,
  LogLevel,
  LogPayload,
  PayloadMap,
  RotateProxyPayload,
  RunCompletePayload,
  RunErrorPayload,
  RunId
} from './types/events'
export { CONTRACT_VERSION } from './types/events'
// Proxy types (from sdk/src/types/proxy.ts)
export type {
  JobProxyRequest,
  ProxyEndpoint,
  ProxyEndpointRedacted,
  ProxyPool,
  ProxyProtocol,
  ProxySticky,
  ProxyStickyScope,
  ProxyStrategy
} from './types/proxy'
