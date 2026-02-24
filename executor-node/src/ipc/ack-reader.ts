/**
 * AckReader: reads length-prefixed msgpack frames from stdin (phase 2).
 *
 * After the executor reads JSON metadata (phase 1), stdin remains open for
 * the runtime to send file_write_ack frames back. AckReader attaches to
 * stdin and matches incoming acks to pending promises by write_id.
 *
 * @module
 */
import type { Readable } from 'node:stream'
import { decodeFileWriteAck } from './frame.js'

/** Minimum frame size: 4-byte length prefix + at least 1 byte payload. */
const LENGTH_PREFIX_SIZE = 4

/**
 * AckReader reads file_write_ack frames from stdin and resolves pending promises.
 *
 * Lifecycle:
 * 1. Construct with a readable stream (stdin)
 * 2. Call start() to begin reading
 * 3. Call waitForAck(writeId) before sending the file_write frame
 * 4. Call stop() after execution completes
 *
 * On EOF or error, all pending promises are rejected.
 */
export class AckReader {
  private readonly pending = new Map<
    number,
    { resolve: () => void; reject: (err: Error) => void }
  >()
  private buffer = Buffer.alloc(0)
  private stopped = false
  private readonly stream: Readable

  constructor(stream: Readable) {
    this.stream = stream
  }

  /**
   * Register a pending ack for the given writeId.
   * Returns a promise that resolves on success ack or rejects on error ack/EOF.
   */
  waitForAck(writeId: number): Promise<void> {
    if (this.stopped) {
      return Promise.reject(new Error('AckReader is stopped'))
    }
    return new Promise<void>((resolve, reject) => {
      this.pending.set(writeId, { resolve, reject })
    })
  }

  /**
   * Start reading frames from the stream.
   * Attaches data/end/error listeners.
   */
  start(): void {
    this.stream.on('data', this.onData)
    this.stream.on('end', this.onEnd)
    this.stream.on('error', this.onError)
  }

  /**
   * Stop the reader and reject all pending promises.
   */
  stop(): void {
    if (this.stopped) return
    this.stopped = true
    this.stream.removeListener('data', this.onData)
    this.stream.removeListener('end', this.onEnd)
    this.stream.removeListener('error', this.onError)
    this.rejectAll(new Error('AckReader stopped'))
  }

  /**
   * Returns true if there are no pending ack promises.
   */
  get idle(): boolean {
    return this.pending.size === 0
  }

  private readonly onData = (chunk: Buffer): void => {
    this.buffer = Buffer.concat([this.buffer, chunk])
    this.drainBuffer()
  }

  private readonly onEnd = (): void => {
    // stdin EOF — reject remaining if any, but 0 pending is normal ("no ack support")
    this.stopped = true
    this.rejectAll(new Error('stdin closed (EOF)'))
  }

  private readonly onError = (err: Error): void => {
    this.stopped = true
    this.rejectAll(new Error(`stdin error: ${err.message}`))
  }

  /** Consume complete frames from the internal buffer. */
  private drainBuffer(): void {
    while (this.buffer.length >= LENGTH_PREFIX_SIZE) {
      const payloadLen = this.buffer.readUInt32BE(0)
      const totalLen = LENGTH_PREFIX_SIZE + payloadLen

      if (this.buffer.length < totalLen) {
        // Incomplete frame — wait for more data
        return
      }

      const payload = this.buffer.subarray(LENGTH_PREFIX_SIZE, totalLen)
      this.buffer = this.buffer.subarray(totalLen)

      this.processPayload(payload)
    }
  }

  /** Decode and dispatch a single ack frame. */
  private processPayload(payload: Uint8Array): void {
    let ack: ReturnType<typeof decodeFileWriteAck>
    try {
      ack = decodeFileWriteAck(payload)
    } catch {
      // Unknown or malformed frame — ignore (future frame types may appear)
      return
    }

    const entry = this.pending.get(ack.write_id)
    if (!entry) {
      // No pending promise for this write_id — stale or duplicate ack
      return
    }

    this.pending.delete(ack.write_id)

    if (ack.ok) {
      entry.resolve()
    } else {
      entry.reject(new Error(ack.error ?? 'file write failed'))
    }
  }

  /** Reject all pending promises with the given error. */
  private rejectAll(err: Error): void {
    for (const [, entry] of this.pending) {
      entry.reject(err)
    }
    this.pending.clear()
  }
}
