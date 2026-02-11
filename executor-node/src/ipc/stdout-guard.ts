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
  /** Stream whose .write() sends data through the real stdout fd. */
  readonly ipcOutput: Writable
}

let installed = false

/**
 * Install the stdout guard. Must be called exactly once per process.
 *
 * After this call:
 * - `ipcOutput.write(data)` → real stdout (binary IPC)
 * - `process.stdout.write(text)` → redirected to stderr with warning
 *
 * The returned `ipcOutput` inherits stream state and events from
 * process.stdout via prototype, so backpressure (`drain`), lifecycle
 * (`destroyed`, `writableEnded`), and event listeners all work correctly.
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

  // Prototype-based proxy: inherits stream state + EventEmitter from real stdout
  const ipcOutput = Object.create(process.stdout) as Writable
  ipcOutput.write = origWrite

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

  return { ipcOutput }
}

/**
 * Reset guard state. Exported only for testing — never call in production.
 * @internal
 */
export function resetStdoutGuardForTest(): void {
  installed = false
}
