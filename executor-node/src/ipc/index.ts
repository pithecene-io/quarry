/**
 * IPC module for executor-runtime communication.
 *
 * Implements CONTRACT_IPC.md framing and streaming protocol.
 *
 * @module
 */

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
  // Encoding
  encodeFrame,
  encodeEventFrame,
  encodeArtifactChunkFrame,
  calculateChunks,
  encodeArtifactChunks
} from './frame.js'

export { StdioSink, StreamClosedError } from './sink.js'
