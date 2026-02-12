/**
 * Stdout guard: protects the IPC channel from stray writes.
 *
 * The executor uses process.stdout as a binary IPC channel (4-byte BE length
 * prefix + msgpack). If third-party code (puppeteer-extra, stealth plugin, any
 * npm dep) writes text to stdout, the Go FrameDecoder interprets ASCII bytes
 * as a length prefix, derives a huge payload size, and io.ReadFull blocks
 * indefinitely.
 *
 * The guard captures the real stdout.write for IPC use and redirects any other
 * stdout writes to stderr with a diagnostic warning.
 *
 * @module
 */
import type { Writable } from 'node:stream'

export type StdoutGuardResult = {
  /** The real stdout stream for event listening and state inspection. */
  readonly ipcOutput: Writable
  /** Original stdout.write, bypasses the guard patch for IPC frame data. */
  readonly ipcWrite: (data: Buffer) => boolean
}

let installed = false

/**
 * Install the stdout guard. Must be called exactly once per process.
 *
 * After this call:
 * - `ipcWrite(data)` → real stdout (binary IPC frames)
 * - `process.stdout.write(text)` → redirected to stderr with warning
 *
 * Returns the real `process.stdout` stream (for event listening and state
 * inspection) and the original write function (for IPC frame data). The
 * stream and write function are separated because `process.stdout.write`
 * is patched to intercept stray writes — IPC code must bypass the patch
 * via `ipcWrite` while still listening for backpressure events (`drain`,
 * `error`, etc.) on the real stream.
 *
 * @throws Error if called more than once (would capture the patched
 *   redirector instead of the real write, corrupting the IPC channel)
 */
export function installStdoutGuard(): StdoutGuardResult {
  if (installed) {
    throw new Error('installStdoutGuard() must only be called once per process')
  }
  installed = true

  const origWrite = process.stdout.write.bind(process.stdout)

  // Patch process.stdout.write to redirect stray writes to stderr.
  // Always returns true: stray callers do not own backpressure on this
  // stream, and returning stderr's boolean would leak a drain contract
  // on the wrong stream (caller waits for 'drain' on stdout, but the
  // underlying write happened on stderr).
  process.stdout.write = ((
    chunk: Uint8Array | string,
    encodingOrCallback?: BufferEncoding | ((error?: Error | null) => void),
    callback?: (error?: Error | null) => void
  ): boolean => {
    const text = typeof chunk === 'string' ? chunk : Buffer.from(chunk).toString('utf-8')
    const preview = text.replace(/\n/g, '\\n').slice(0, 200)
    process.stderr.write(`[quarry] stdout guard: intercepted stray write: ${preview}\n`)

    // Forward to stderr so content is not lost
    if (typeof encodingOrCallback === 'function') {
      process.stderr.write(chunk, encodingOrCallback)
    } else {
      process.stderr.write(chunk, encodingOrCallback, callback)
    }
    return true
  }) as typeof process.stdout.write

  return {
    ipcOutput: process.stdout,
    ipcWrite: (data: Buffer) => origWrite(data)
  }
}

/**
 * Reset guard state. Exported only for testing — never call in production.
 * @internal
 */
export function resetStdoutGuardForTest(): void {
  installed = false
}
