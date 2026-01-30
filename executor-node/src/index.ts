/**
 * Quarry Executor Node
 *
 * Executes Quarry scripts in a Puppeteer environment and streams
 * events to the runtime via IPC.
 *
 * @packageDocumentation
 */

// Executor
export {
  type ExecutionOutcome,
  type ExecutorConfig,
  type ExecutorResult,
  execute,
  parseRunMeta
} from './executor.js'
// IPC (re-export for advanced usage)
export {
  type ArtifactChunkFrame,
  type ChunkMeta,
  type EventFrame,
  type Frame,
  // Errors
  FrameSizeError,
  // Types
  type FrameType,
  LENGTH_PREFIX_SIZE,
  MAX_CHUNK_SIZE,
  // Constants
  MAX_FRAME_SIZE,
  MAX_PAYLOAD_SIZE,
  ObservingSink,
  SinkAlreadyFailedError,
  type SinkState,
  // Sink
  StdioSink,
  StreamClosedError,
  type TerminalState,
  type TerminalType
} from './ipc/index.js'
// Loader
export { type LoadedScript, loadScript, ScriptLoadError } from './loader.js'
