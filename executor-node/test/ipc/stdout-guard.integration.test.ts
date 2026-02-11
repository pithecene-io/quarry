/**
 * Integration test: stdout guard protects IPC channel from stray writes.
 *
 * Spawns a child process that installs the stdout guard, writes IPC frames
 * through ipcOutput, injects a stray process.stdout.write (simulating
 * third-party code), and exits. The parent reads the pipe and asserts:
 * - stdout contains exactly 2 valid IPC frames (no stray text contamination)
 * - stderr contains the stray text + guard warning
 * - exit code 0
 */
import { type ChildProcess, spawn } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { decode as msgpackDecode } from '@msgpack/msgpack'
import { describe, expect, it } from 'vitest'
import { LENGTH_PREFIX_SIZE } from '../../src/ipc/frame.js'

const testDir = dirname(fileURLToPath(import.meta.url))
const fixturePath = resolve(testDir, 'fixtures/stdout-guard-child.ts')
const tsxBin = resolve(testDir, '../../../node_modules/.bin/tsx')

/**
 * Decode all length-prefixed msgpack frames from a buffer.
 */
function decodeFrames(data: Buffer): unknown[] {
  const frames: unknown[] = []
  let offset = 0
  while (offset + LENGTH_PREFIX_SIZE <= data.length) {
    const payloadLen = data.readUInt32BE(offset)
    offset += LENGTH_PREFIX_SIZE
    frames.push(msgpackDecode(data.subarray(offset, offset + payloadLen)))
    offset += payloadLen
  }
  return frames
}

/**
 * Spawn the fixture child process and collect stdout + stderr + exit code.
 */
function runFixture(): Promise<{ stdout: Buffer; stderr: string; exitCode: number | null }> {
  return new Promise((resolve, reject) => {
    let child: ChildProcess
    try {
      child = spawn(tsxBin, [fixturePath], {
        stdio: ['ignore', 'pipe', 'pipe']
      })
    } catch (err) {
      reject(err)
      return
    }

    const stdoutChunks: Buffer[] = []
    const stderrChunks: Buffer[] = []
    child.stdout!.on('data', (chunk: Buffer) => stdoutChunks.push(chunk))
    child.stderr!.on('data', (chunk: Buffer) => stderrChunks.push(chunk))
    child.on('error', reject)
    child.on('close', (code) => {
      resolve({
        stdout: Buffer.concat(stdoutChunks),
        stderr: Buffer.concat(stderrChunks).toString('utf-8'),
        exitCode: code
      })
    })
  })
}

describe('stdout guard integration', () => {
  it('IPC frames are clean on stdout, stray writes appear on stderr', async () => {
    const { stdout, stderr, exitCode } = await runFixture()

    // Child should exit cleanly
    expect(exitCode, `child stderr: ${stderr}`).toBe(0)

    // Stdout should contain exactly 2 valid IPC frames
    const frames = decodeFrames(stdout) as Record<string, unknown>[]
    expect(frames).toHaveLength(2)

    // Frame 0: item event
    expect(frames[0].type).toBe('item')
    expect(frames[0].seq).toBe(1)

    // Frame 1: run_complete terminal event
    expect(frames[1].type).toBe('run_complete')
    expect(frames[1].seq).toBe(2)

    // Stderr should contain the guard warning
    expect(stderr).toContain('[quarry] stdout guard:')
    expect(stderr).toContain('Browser started successfully')
  }, 15_000)
})
