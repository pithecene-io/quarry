/**
 * Quarry Executor Node
 *
 * Executes Quarry scripts in a Puppeteer environment and streams
 * events to the runtime via IPC.
 *
 * @packageDocumentation
 */

// Executor
export { execute, parseRunMeta, type ExecutorConfig, type ExecutorResult, type ExecutionOutcome } from './executor.js'

// Loader
export { loadScript, ScriptLoadError, type LoadedScript } from './loader.js'

// IPC (re-export for advanced usage)
export {
  // Constants
  MAX_FRAME_SIZE,
  MAX_PAYLOAD_SIZE,
  MAX_CHUNK_SIZE,
  LENGTH_PREFIX_SIZE,
  // Types
  type FrameType,
  type EventFrame,
  type ArtifactChunkFrame,
  type Frame,
  type ChunkMeta,
  // Errors
  FrameSizeError,
  // Sink
  StdioSink,
  StreamClosedError
} from './ipc/index.js'
