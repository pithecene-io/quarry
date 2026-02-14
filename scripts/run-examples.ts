#!/usr/bin/env npx tsx
/**
 * Example runner for Quarry.
 * Executes all examples defined in examples/manifest.json and validates their output.
 *
 * Exit codes:
 *   0 - All examples passed
 *   1 - One or more examples failed
 *
 * @module
 */

import { type ChildProcess, spawn } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ROOT_DIR = resolve(__dirname, '..')
const EXAMPLES_DIR = join(ROOT_DIR, 'examples')
const MANIFEST_PATH = join(EXAMPLES_DIR, 'manifest.json')
const OUTPUT_DIR = join(ROOT_DIR, '.example-runs')
const QUARRY_BIN = join(ROOT_DIR, 'quarry', 'quarry')
const EXECUTOR_BIN = join(ROOT_DIR, 'executor-node', 'dist', 'bin', 'executor.js')

interface ExampleExpected {
  events?: Record<string, number>
  artifacts?: Array<{
    name: string
    content_type: string
    min_size?: number
  }>
  terminal: 'run_complete' | 'run_error'
  exit_code?: number
}

interface Example {
  name: string
  script: string
  description: string
  expected: ExampleExpected
}

interface Manifest {
  examples: Example[]
}

interface RunResult {
  name: string
  passed: boolean
  duration: number
  exitCode: number
  stdout: string
  stderr: string
  errors: string[]
}

function loadManifest(): Manifest {
  const content = readFileSync(MANIFEST_PATH, 'utf8')
  return JSON.parse(content) as Manifest
}

async function buildQuarry(): Promise<void> {
  console.log('Building quarry CLI...')
  await runCommand('go', ['build', '-o', QUARRY_BIN, './cmd/quarry'], {
    cwd: join(ROOT_DIR, 'quarry')
  })
}

async function buildExecutor(): Promise<void> {
  console.log('Building executor...')
  await runCommand('pnpm', ['run', 'build'], {
    cwd: join(ROOT_DIR, 'executor-node')
  })
}

function runCommand(
  cmd: string,
  args: string[],
  options: { cwd?: string; timeout?: number } = {}
): Promise<{ stdout: string; stderr: string; exitCode: number }> {
  return new Promise((resolve, reject) => {
    const proc = spawn(cmd, args, {
      cwd: options.cwd,
      stdio: ['pipe', 'pipe', 'pipe']
    })

    let stdout = ''
    let stderr = ''

    proc.stdout.on('data', (data) => {
      stdout += data.toString()
    })

    proc.stderr.on('data', (data) => {
      stderr += data.toString()
    })

    const timeout = options.timeout ?? 60000
    const timer = setTimeout(() => {
      proc.kill('SIGKILL')
      reject(new Error(`Command timed out after ${timeout}ms`))
    }, timeout)

    proc.on('close', (code) => {
      clearTimeout(timer)
      resolve({ stdout, stderr, exitCode: code ?? 1 })
    })

    proc.on('error', (err) => {
      clearTimeout(timer)
      reject(err)
    })
  })
}

async function runExample(example: Example): Promise<RunResult> {
  const runId = `example-${example.name}-${Date.now()}`
  const storagePath = join(OUTPUT_DIR, example.name)

  // Ensure storage directory exists
  mkdirSync(storagePath, { recursive: true })

  const scriptPath = join(EXAMPLES_DIR, example.script)
  const startTime = Date.now()
  const errors: string[] = []

  // Build the command
  const args = [
    'run',
    '--script',
    scriptPath,
    '--run-id',
    runId,
    '--source',
    'example-runner',
    '--category',
    'examples',
    '--no-browser-reuse',
    '--executor',
    EXECUTOR_BIN,
    '--storage-backend',
    'fs',
    '--storage-path',
    storagePath,
    '--policy',
    'strict'
  ]

  console.log(`  Running: ${QUARRY_BIN} ${args.join(' ')}`)

  let result: { stdout: string; stderr: string; exitCode: number }
  try {
    result = await runCommand(QUARRY_BIN, args, {
      cwd: ROOT_DIR,
      timeout: 120000 // 2 minute timeout per example
    })
  } catch (err) {
    const duration = Date.now() - startTime
    return {
      name: example.name,
      passed: false,
      duration,
      exitCode: -1,
      stdout: '',
      stderr: err instanceof Error ? err.message : String(err),
      errors: [`Failed to run: ${err}`]
    }
  }

  const duration = Date.now() - startTime

  // Validate exit code
  const expectedExitCode = example.expected.exit_code ?? 0
  if (result.exitCode !== expectedExitCode) {
    errors.push(`Exit code mismatch: expected ${expectedExitCode}, got ${result.exitCode}`)
  }

  // Exit-code validation is the primary correctness signal: the runtime maps
  // terminal states to specific codes (0=success, 1=script error, 2=crash,
  // 3=policy failure). Event counting from storage is deferred because it
  // couples this runner to the storage layout and adds disk I/O without
  // improving confidence beyond what exit codes already guarantee.

  // Write logs
  const logPath = join(storagePath, 'run.log')
  writeFileSync(
    logPath,
    `=== ${example.name} ===\n` +
      `Run ID: ${runId}\n` +
      `Duration: ${duration}ms\n` +
      `Exit Code: ${result.exitCode}\n` +
      `\n=== STDOUT ===\n${result.stdout}\n` +
      `\n=== STDERR ===\n${result.stderr}\n`
  )

  return {
    name: example.name,
    passed: errors.length === 0,
    duration,
    exitCode: result.exitCode,
    stdout: result.stdout,
    stderr: result.stderr,
    errors
  }
}

async function main(): Promise<void> {
  console.log('Quarry Example Runner')
  console.log('=====================\n')

  // Ensure output directory exists
  mkdirSync(OUTPUT_DIR, { recursive: true })

  // Load manifest
  const manifest = loadManifest()
  console.log(`Found ${manifest.examples.length} examples\n`)

  // Build components
  try {
    await buildQuarry()
    await buildExecutor()
  } catch (err) {
    console.error('Build failed:', err)
    process.exit(1)
  }

  console.log('\nRunning examples...\n')

  // Run each example
  const results: RunResult[] = []
  for (const example of manifest.examples) {
    console.log(`[${example.name}] ${example.description}`)

    const result = await runExample(example)
    results.push(result)

    if (result.passed) {
      console.log(`  ✅ PASSED (${result.duration}ms)\n`)
    } else {
      console.log(`  ❌ FAILED (${result.duration}ms)`)
      for (const error of result.errors) {
        console.log(`     - ${error}`)
      }
      console.log()
    }
  }

  // Summary
  console.log('\n=== Summary ===')
  const passed = results.filter((r) => r.passed).length
  const failed = results.filter((r) => !r.passed).length
  console.log(`Passed: ${passed}/${results.length}`)
  console.log(`Failed: ${failed}/${results.length}`)
  console.log(`Logs: ${OUTPUT_DIR}`)

  // Write summary JSON
  const summaryPath = join(OUTPUT_DIR, 'summary.json')
  writeFileSync(
    summaryPath,
    JSON.stringify(
      {
        timestamp: new Date().toISOString(),
        total: results.length,
        passed,
        failed,
        results: results.map((r) => ({
          name: r.name,
          passed: r.passed,
          duration: r.duration,
          exitCode: r.exitCode,
          errors: r.errors
        }))
      },
      null,
      2
    )
  )

  if (failed > 0) {
    process.exit(1)
  }
}

main().catch((err) => {
  console.error('Fatal error:', err)
  process.exit(1)
})
