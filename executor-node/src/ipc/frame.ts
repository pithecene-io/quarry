/**
 * IPC framing implementation per CONTRACT_IPC.md.
 *
 * Frame structure:
 * - 4-byte big-endian length prefix (unsigned, big-endian)
 * - payload bytes (msgpack-encoded)
 *
 * Constraints:
 * - Maximum frame size: 16 MiB (including length prefix)
 * - Maximum payload size: 16 MiB - 4 bytes
 * - Artifact chunk size: up to 8 MiB (raw bytes, before msgpack encoding)
 *
 * All frames are msgpack-encoded and include an explicit `type` discriminant
 * for deterministic decoding. Two frame types exist:
 * - `event`: wraps an EventEnvelope
 * - `artifact_chunk`: raw binary chunk for artifact streaming
 *
 * @module
 * @remarks Node.js only. Uses Buffer for transport efficiency.
 */

import type { ArtifactId, EventEnvelope } from '@justapithecus/quarry-sdk'
import { encode as msgpackEncode } from '@msgpack/msgpack'

/**
 * Maximum frame size in bytes (16 MiB), including length prefix.
 * Frames exceeding this limit are invalid and must be rejected.
 */
export const MAX_FRAME_SIZE = 16 * 1024 * 1024

/**
 * Maximum payload size in bytes (MAX_FRAME_SIZE - LENGTH_PREFIX_SIZE).
 */
export const MAX_PAYLOAD_SIZE = MAX_FRAME_SIZE - 4

/**
 * Maximum artifact chunk size in bytes (8 MiB).
 * This is the limit on raw bytes before msgpack encoding.
 * Artifacts larger than this must be split into multiple chunks.
 */
export const MAX_CHUNK_SIZE = 8 * 1024 * 1024

/**
 * Length prefix size in bytes.
 */
export const LENGTH_PREFIX_SIZE = 4

/**
 * Frame types for discriminating between event frames and artifact chunk frames.
 * All frames include this discriminant for deterministic decoding.
 */
export type FrameType = 'event' | 'artifact_chunk'

/**
 * Event frame envelope wrapping an EventEnvelope.
 * Provides explicit type discriminant for decoding.
 */
export interface EventFrame {
  readonly type: 'event'
  readonly envelope: EventEnvelope
}

/**
 * Artifact chunk frame per CONTRACT_IPC.md.
 * This is a stream-level construct, not a normal emit event.
 *
 * @remarks
 * The `data` field is encoded as msgpack bin type (not array).
 * Receivers must decode it as raw bytes.
 */
export interface ArtifactChunkFrame {
  readonly type: 'artifact_chunk'
  readonly artifact_id: ArtifactId
  /** Sequence number, starts at 1 */
  readonly seq: number
  /** True if this is the last chunk */
  readonly is_last: boolean
  /** Raw binary data (msgpack bin type) */
  readonly data: Uint8Array
}

/**
 * Union of all frame types for decoding.
 */
export type Frame = EventFrame | ArtifactChunkFrame

/**
 * Error thrown when a frame exceeds the maximum size.
 */
export class FrameSizeError extends Error {
  constructor(
    public readonly payloadSize: number,
    public readonly maxPayloadSize: number
  ) {
    super(`Payload size ${payloadSize} exceeds maximum ${maxPayloadSize}`)
    this.name = 'FrameSizeError'
  }
}

/**
 * Encode data into a framed buffer with length prefix.
 *
 * @param payload - The payload bytes to frame
 * @returns Buffer containing length prefix + payload
 * @throws FrameSizeError if payload exceeds MAX_PAYLOAD_SIZE
 *
 * @remarks
 * The total frame size (prefix + payload) is bounded by MAX_FRAME_SIZE.
 */
export function encodeFrame(payload: Uint8Array): Buffer {
  if (payload.length > MAX_PAYLOAD_SIZE) {
    throw new FrameSizeError(payload.length, MAX_PAYLOAD_SIZE)
  }

  const frame = Buffer.allocUnsafe(LENGTH_PREFIX_SIZE + payload.length)
  frame.writeUInt32BE(payload.length, 0)
  frame.set(payload, LENGTH_PREFIX_SIZE)

  return frame
}

/**
 * Encode an event envelope into a framed buffer.
 * Wraps the envelope in an EventFrame with explicit type discriminant.
 *
 * @param envelope - The event envelope to encode
 * @returns Buffer containing length prefix + msgpack-encoded frame
 * @throws FrameSizeError if encoded payload exceeds MAX_PAYLOAD_SIZE
 */
export function encodeEventFrame(envelope: EventEnvelope): Buffer {
  const frame: EventFrame = { type: 'event', envelope }
  const payload = msgpackEncode(frame)
  return encodeFrame(payload)
}

/**
 * Encode an artifact chunk into a framed buffer.
 *
 * @param artifactId - The artifact ID
 * @param seq - Sequence number (starts at 1)
 * @param isLast - True if this is the final chunk
 * @param data - Raw binary data (msgpack bin type)
 * @returns Buffer containing length prefix + msgpack-encoded frame
 * @throws FrameSizeError if encoded payload exceeds MAX_PAYLOAD_SIZE
 *
 * @remarks
 * The raw data size should not exceed MAX_CHUNK_SIZE. This function
 * validates the final encoded size against MAX_PAYLOAD_SIZE as a guard.
 */
export function encodeArtifactChunkFrame(
  artifactId: ArtifactId,
  seq: number,
  isLast: boolean,
  data: Uint8Array
): Buffer {
  const frame: ArtifactChunkFrame = {
    type: 'artifact_chunk',
    artifact_id: artifactId,
    seq,
    is_last: isLast,
    data
  }
  const payload = msgpackEncode(frame)
  return encodeFrame(payload)
}

/**
 * Metadata for a single artifact chunk (without data).
 * Used by chunk iterators to avoid data copying.
 */
export interface ChunkMeta {
  readonly seq: number
  readonly isLast: boolean
  readonly offset: number
  readonly length: number
}

/**
 * Calculate chunk metadata for artifact data.
 * Does not copy data; returns offsets for slicing.
 *
 * @param totalSize - Total size of the artifact data
 * @returns Array of chunk metadata
 */
export function calculateChunks(totalSize: number): ChunkMeta[] {
  const chunks: ChunkMeta[] = []

  if (totalSize === 0) {
    // Empty artifact: single chunk with is_last=true
    chunks.push({ seq: 1, isLast: true, offset: 0, length: 0 })
    return chunks
  }

  let offset = 0
  let seq = 1

  while (offset < totalSize) {
    const remaining = totalSize - offset
    const length = Math.min(remaining, MAX_CHUNK_SIZE)
    const isLast = offset + length >= totalSize

    chunks.push({ seq, isLast, offset, length })

    offset += length
    seq++
  }

  return chunks
}

/**
 * Generator that yields encoded artifact chunk frames.
 * Memory-efficient: encodes one chunk at a time.
 *
 * @param artifactId - The artifact ID for the chunks
 * @param data - The binary data to chunk
 * @yields Encoded frame buffers ready for transport
 */
export function* encodeArtifactChunks(
  artifactId: ArtifactId,
  data: Buffer | Uint8Array
): Generator<Buffer, void, unknown> {
  const chunks = calculateChunks(data.length)

  for (const chunk of chunks) {
    const chunkData = data.subarray(chunk.offset, chunk.offset + chunk.length)
    yield encodeArtifactChunkFrame(artifactId, chunk.seq, chunk.isLast, chunkData)
  }
}
