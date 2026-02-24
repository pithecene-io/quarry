import { PassThrough } from 'node:stream'
import { encode as msgpackEncode } from '@msgpack/msgpack'
import { describe, expect, it } from 'vitest'
import { AckReader } from '../../src/ipc/ack-reader.js'
import { encodeFrame } from '../../src/ipc/frame.js'

/** Encode a file_write_ack payload as a length-prefixed msgpack frame. */
function encodeAck(writeId: number, ok: boolean, error?: string): Buffer {
  const payload = msgpackEncode({
    type: 'file_write_ack',
    write_id: writeId,
    ok,
    ...(error != null && { error })
  })
  return Buffer.from(encodeFrame(payload))
}

describe('AckReader', () => {
  it('resolves on success ack', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()

    const promise = reader.waitForAck(1)
    stream.write(encodeAck(1, true))

    await expect(promise).resolves.toBeUndefined()
    reader.stop()
  })

  it('rejects on error ack', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()

    const promise = reader.waitForAck(1)
    stream.write(encodeAck(1, false, 'S3 PutObject failed'))

    await expect(promise).rejects.toThrow('S3 PutObject failed')
    reader.stop()
  })

  it('rejects all pending on EOF', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()

    const p1 = reader.waitForAck(1)
    const p2 = reader.waitForAck(2)

    stream.end()

    await expect(p1).rejects.toThrow('stdin closed (EOF)')
    await expect(p2).rejects.toThrow('stdin closed (EOF)')
  })

  it('handles partial frame buffering', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()

    const promise = reader.waitForAck(1)
    const fullFrame = encodeAck(1, true)

    // Write first half, then second half
    const mid = Math.floor(fullFrame.length / 2)
    stream.write(fullFrame.subarray(0, mid))

    // Small delay to let the first chunk be processed
    await new Promise((r) => setTimeout(r, 10))

    stream.write(fullFrame.subarray(mid))

    await expect(promise).resolves.toBeUndefined()
    reader.stop()
  })

  it('stop() rejects pending promises', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()

    const promise = reader.waitForAck(1)
    reader.stop()

    await expect(promise).rejects.toThrow('AckReader stopped')
  })

  it('ignores unknown frame types', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()

    const promise = reader.waitForAck(1)

    // Send an unknown frame type
    const unknownPayload = msgpackEncode({ type: 'unknown_frame', data: 'hello' })
    stream.write(Buffer.from(encodeFrame(unknownPayload)))

    // Then send the real ack
    stream.write(encodeAck(1, true))

    await expect(promise).resolves.toBeUndefined()
    reader.stop()
  })

  it('handles multiple acks in order', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()

    const p1 = reader.waitForAck(1)
    const p2 = reader.waitForAck(2)
    const p3 = reader.waitForAck(3)

    // Send all three acks
    stream.write(encodeAck(1, true))
    stream.write(encodeAck(2, false, 'disk full'))
    stream.write(encodeAck(3, true))

    await expect(p1).resolves.toBeUndefined()
    await expect(p2).rejects.toThrow('disk full')
    await expect(p3).resolves.toBeUndefined()
    reader.stop()
  })

  it('rejects waitForAck after stop', async () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()
    reader.stop()

    await expect(reader.waitForAck(1)).rejects.toThrow('AckReader is stopped')
  })

  it('reports idle when no pending acks', () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    expect(reader.idle).toBe(true)
  })

  it('reports non-idle when pending acks', () => {
    const stream = new PassThrough()
    const reader = new AckReader(stream)
    reader.start()
    reader.waitForAck(1).catch(() => {
      /* expected rejection on cleanup */
    })
    expect(reader.idle).toBe(false)
    reader.stop()
  })
})
