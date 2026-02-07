/**
 * IPC module for executor-runtime communication.
 *
 * Implements CONTRACT_IPC.md framing and streaming protocol.
 *
 * @module
 */

export {
  // Types
  type ArtifactChunkFrame,
  type ArtifactChunkType,
  type ChunkMeta,
  // Errors
  ChunkValidationError,
  // Encoding
  calculateChunks,
  encodeArtifactChunkFrame,
  encodeArtifactChunks,
  encodeEventFrame,
  encodeFileWriteFrame,
  encodeFrame,
  type FileWriteFrame,
  type Frame,
  FrameSizeError,
  // Constants
  LENGTH_PREFIX_SIZE,
  MAX_CHUNK_SIZE,
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
