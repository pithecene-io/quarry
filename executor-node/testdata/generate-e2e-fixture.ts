#!/usr/bin/env npx tsx
/**
 * Generates E2E IPC fixture by running the real executor.
 *
 * This script spawns the Node executor with the fixture script and
 * captures the raw stdout bytes as a binary fixture file.
 *
 * Usage:
 *   npx tsx testdata/generate-e2e-fixture.ts
 *
 * Output:
 *   ../quarry/ipc/testdata/e2e_fixture.bin
 *   ../quarry/ipc/testdata/e2e_fixture.manifest.json
 *
 * The manifest includes metadata about the fixture for validation.
 *
 * @module
 */
import { spawn } from 'node:child_process'
import { writeFileSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const OUTPUT_DIR = join(__dirname, '../../quarry/ipc/testdata')
const EXECUTOR_BIN = join(__dirname, '../dist/bin/executor.js')
const FIXTURE_SCRIPT = join(__dirname, 'e2e-fixture-script.js')

// Ensure output directory exists
mkdirSync(OUTPUT_DIR, { recursive: true })

// Run input matching the fixture script expectations
const runInput = {
  run_id: 'run-e2e-fixture-001',
  attempt: 1,
  job: {
    fixture: true,
    version: '1.0.0'
  }
}

async function generateFixture(): Promise<void> {
  console.log('Generating E2E fixture...')
  console.log(`Executor: ${EXECUTOR_BIN}`)
  console.log(`Script: ${FIXTURE_SCRIPT}`)

  const stdout: Buffer[] = []
  const stderr: string[] = []

  const executor = spawn('node', [EXECUTOR_BIN, FIXTURE_SCRIPT], {
    env: {
      ...process.env,
      QUARRY_NO_SANDBOX: '1' // Disable Chrome sandbox for CI
    }
  })

  // Send run input to stdin
  executor.stdin.write(JSON.stringify(runInput))
  executor.stdin.end()

  // Capture stdout (binary)
  executor.stdout.on('data', (chunk: Buffer) => {
    stdout.push(chunk)
  })

  // Capture stderr (text)
  executor.stderr.on('data', (chunk: Buffer) => {
    stderr.push(chunk.toString())
  })

  // Wait for process to exit
  const exitCode = await new Promise<number>((resolve, reject) => {
    executor.on('close', (code) => {
      resolve(code ?? 0)
    })
    executor.on('error', (err) => {
      reject(err)
    })
  })

  // Log stderr if any
  if (stderr.length > 0) {
    console.log('\nExecutor stderr:')
    console.log(stderr.join(''))
  }

  // Check exit code
  if (exitCode !== 0) {
    console.error(`\nExecutor exited with code ${exitCode}`)
    if (exitCode >= 2) {
      throw new Error(`Executor crashed with exit code ${exitCode}`)
    }
  }

  // Combine stdout chunks
  const fixtureData = Buffer.concat(stdout)
  console.log(`\nCaptured ${fixtureData.length} bytes`)

  // Write fixture binary
  const fixturePath = join(OUTPUT_DIR, 'e2e_fixture.bin')
  writeFileSync(fixturePath, fixtureData)
  console.log(`Written: ${fixturePath}`)

  // Count frames (simple length-prefix parsing)
  let frameCount = 0
  let offset = 0
  while (offset < fixtureData.length) {
    if (offset + 4 > fixtureData.length) break
    const length = fixtureData.readUInt32BE(offset)
    offset += 4 + length
    frameCount++
  }

  // Write manifest
  const manifest = {
    generated_at: new Date().toISOString(),
    run_id: runInput.run_id,
    fixture_script: 'e2e-fixture-script.ts',
    size_bytes: fixtureData.length,
    frame_count: frameCount,
    expected_events: [
      { seq: 1, type: 'item', item_type: 'fixture_product' },
      { seq: 2, type: 'log', level: 'info' },
      // artifact_chunk frames are not events, they don't have seq
      { seq: 3, type: 'artifact', name: 'fixture-screenshot.bin' },
      { seq: 4, type: 'item', item_type: 'fixture_summary' },
      { seq: 5, type: 'run_complete' }
    ],
    expected_artifact_chunks: 1, // 256 bytes fits in one chunk
    exit_code: exitCode
  }

  const manifestPath = join(OUTPUT_DIR, 'e2e_fixture.manifest.json')
  writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n')
  console.log(`Written: ${manifestPath}`)

  console.log('\nFixture generation complete!')
  console.log(`Frames: ${frameCount}`)
}

generateFixture().catch((err) => {
  console.error('Fixture generation failed:', err)
  process.exit(1)
})
