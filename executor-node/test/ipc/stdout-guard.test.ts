/**
 * Unit tests for the stdout guard.
 *
 * These tests verify that installStdoutGuard() correctly:
 * - Returns ipcWrite that calls the original stdout.write
 * - Returns ipcOutput that IS process.stdout (for event listening)
 * - Patches process.stdout.write to redirect to stderr with a warning
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { installStdoutGuard, resetStdoutGuardForTest } from '../../src/ipc/stdout-guard.js'

describe('installStdoutGuard', () => {
  let originalWrite: typeof process.stdout.write

  beforeEach(() => {
    originalWrite = process.stdout.write
  })

  afterEach(() => {
    // Restore original write and reset guard state to avoid polluting other tests
    process.stdout.write = originalWrite
    resetStdoutGuardForTest()
  })

  it('ipcWrite sends data through the original stdout.write', () => {
    const writeSpy = vi.fn<typeof process.stdout.write>().mockReturnValue(true)
    process.stdout.write = writeSpy

    const { ipcWrite } = installStdoutGuard()

    const data = Buffer.from([0x00, 0x00, 0x00, 0x04, 0x01, 0x02, 0x03, 0x04])
    ipcWrite(data)

    // The spy captured the original write binding, so ipcWrite should
    // call the write function that was current at install time
    expect(writeSpy).toHaveBeenCalledWith(data)
  })

  it('ipcOutput is the real process.stdout', () => {
    const { ipcOutput } = installStdoutGuard()

    // ipcOutput must be the real stream — no proxy, no Object.create.
    // This ensures event listeners (drain, error, etc.) registered on
    // ipcOutput fire when the underlying stream emits them.
    expect(ipcOutput).toBe(process.stdout)
  })

  it('process.stdout.write redirects to stderr after guard is installed', () => {
    const stderrSpy = vi.spyOn(process.stderr, 'write').mockReturnValue(true)

    installStdoutGuard()

    process.stdout.write('Browser started successfully\n')

    // Should have been called twice: once for the warning, once for the content
    expect(stderrSpy).toHaveBeenCalledTimes(2)

    // First call is the warning
    const warningCall = stderrSpy.mock.calls[0][0] as string
    expect(warningCall).toContain('[quarry] stdout guard:')
    expect(warningCall).toContain('Browser started successfully')

    // Second call forwards the original content to stderr
    expect(stderrSpy.mock.calls[1][0]).toBe('Browser started successfully\n')

    stderrSpy.mockRestore()
  })

  it('warning preview truncates long content', () => {
    const stderrSpy = vi.spyOn(process.stderr, 'write').mockReturnValue(true)

    installStdoutGuard()

    const longText = 'x'.repeat(500)
    process.stdout.write(longText)

    const warningCall = stderrSpy.mock.calls[0][0] as string
    // Preview should be truncated to 200 chars
    expect(warningCall).toContain('x'.repeat(200))
    expect(warningCall).not.toContain('x'.repeat(201))

    stderrSpy.mockRestore()
  })

  it('warning preview escapes newlines', () => {
    const stderrSpy = vi.spyOn(process.stderr, 'write').mockReturnValue(true)

    installStdoutGuard()

    process.stdout.write('line1\nline2\nline3')

    const warningCall = stderrSpy.mock.calls[0][0] as string
    expect(warningCall).toContain('line1\\nline2\\nline3')

    stderrSpy.mockRestore()
  })

  it('ipcOutput exposes stream properties for state inspection', () => {
    const { ipcOutput } = installStdoutGuard()

    expect(typeof ipcOutput.destroyed).toBe('boolean')
    expect(typeof ipcOutput.writableEnded).toBe('boolean')
    expect(typeof ipcOutput.writableFinished).toBe('boolean')
    expect(typeof ipcOutput.on).toBe('function')
    expect(typeof ipcOutput.off).toBe('function')
    expect(typeof ipcOutput.end).toBe('function')
  })

  it('process.stdout.write passes through encoding parameter', () => {
    const stderrSpy = vi.spyOn(process.stderr, 'write').mockReturnValue(true)

    installStdoutGuard()

    const cb = vi.fn()
    process.stdout.write('test', 'utf-8', cb)

    // The content forwarding call should pass through encoding
    const forwardCall = stderrSpy.mock.calls[1]
    expect(forwardCall[0]).toBe('test')
    expect(forwardCall[1]).toBe('utf-8')

    stderrSpy.mockRestore()
  })

  it('process.stdout.write passes through callback-only signature', () => {
    const stderrSpy = vi.spyOn(process.stderr, 'write').mockReturnValue(true)

    installStdoutGuard()

    const cb = vi.fn()
    process.stdout.write('test', cb)

    // The content forwarding call should pass through the callback
    const forwardCall = stderrSpy.mock.calls[1]
    expect(forwardCall[0]).toBe('test')
    expect(forwardCall[1]).toBe(cb)

    stderrSpy.mockRestore()
  })

  it('throws on double install to prevent capturing patched redirector', () => {
    installStdoutGuard()

    expect(() => installStdoutGuard()).toThrow(
      'installStdoutGuard() must only be called once per process'
    )
  })

  it('patched process.stdout.write always returns true to avoid cross-stream drain hang', () => {
    // Mock stderr.write to return false (backpressure)
    const stderrSpy = vi.spyOn(process.stderr, 'write').mockReturnValue(false)

    installStdoutGuard()

    // Even though stderr signals backpressure, patched stdout.write must
    // return true — callers waiting for 'drain' on process.stdout would
    // hang because the underlying write went to stderr, not stdout.
    const result = process.stdout.write('stray data')
    expect(result).toBe(true)

    stderrSpy.mockRestore()
  })
})
