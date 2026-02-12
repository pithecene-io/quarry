/**
 * Integration test: IPC frames survive backpressure through stdout guard.
 *
 * Spawns a child process that installs the stdout guard, creates a StdioSink
 * with the split contract (ipcOutput for events, ipcWrite for data), and
 * writes ~120KB of IPC frames — enough to exceed the OS pipe buffer (~64KB
 * on Linux) and force at least one backpressure cycle.
 *
 * This test pins the drain deadlock fixed in #163: the old Object.create
 * proxy caused EventEmitter._events divergence after listener cleanup,
 * silently breaking drain delivery and hanging the executor forever.
 *
 * The 10s timeout is the deadlock detector — if drain delivery fails, the
 * child hangs and the test times out instead of passing silently.
 */
import { type ChildProcess, spawn } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { decode as msgpackDecode } from '@msgpack/msgpack'
import { describe, expect, it } from 'vitest'
import { LENGTH_PREFIX_SIZE } from '../../src/ipc/frame.js'

const FRAME_COUNT = 30

const testDir = dirname(fileURLToPath(import.meta.url))
const fixturePath = resolve(testDir, 'fixtures/backpressure-child.ts')
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

describe('backpressure integration', () => {
  it('IPC frames survive backpressure through stdout guard without deadlock', async () => {
    const { stdout, stderr, exitCode } = await runFixture()

    // Child should exit cleanly — a deadlock would hit the 10s timeout
    expect(exitCode, `child stderr: ${stderr}`).toBe(0)

    // Decode all frames from stdout
    const frames = decodeFrames(stdout) as Record<string, unknown>[]

    // All 31 frames (30 items + 1 run_complete) must arrive intact
    expect(frames).toHaveLength(FRAME_COUNT + 1)

    // Verify item frame ordering
    for (let i = 0; i < FRAME_COUNT; i++) {
      expect(frames[i].type).toBe('item')
      expect(frames[i].seq).toBe(i + 1)
    }

    // Verify terminal frame
    expect(frames[FRAME_COUNT].type).toBe('run_complete')
    expect(frames[FRAME_COUNT].seq).toBe(FRAME_COUNT + 1)

    // Stray write should appear on stderr, not stdout
    expect(stderr).toContain('[quarry] stdout guard:')
    expect(stderr).toContain('stray write during backpressure')
  }, 10_000) // 10s timeout: deadlock detector
})
