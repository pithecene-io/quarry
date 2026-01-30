/**
 * IPC module for executor-runtime communication.
 *
 * Implements CONTRACT_IPC.md framing and streaming protocol.
 *
 * @module
 */

export {
  type ArtifactChunkFrame,
  type ChunkMeta,
  calculateChunks,
  type EventFrame,
  encodeArtifactChunkFrame,
  encodeArtifactChunks,
  encodeEventFrame,
  // Encoding
  encodeFrame,
  type Frame,
  // Errors
  FrameSizeError,
  // Types
  type FrameType,
  LENGTH_PREFIX_SIZE,
  MAX_CHUNK_SIZE,
  // Constants
  MAX_FRAME_SIZE,
  MAX_PAYLOAD_SIZE
} from './frame.js'
export {
  ObservingSink,
  SinkAlreadyFailedError,
  type SinkState,
  type TerminalState,
  type TerminalType
} from './observing-sink.js'
export { StdioSink, StreamClosedError } from './sink.js'
