#!/usr/bin/env node
/**
 * CLI entrypoint for the Quarry executor.
 *
 * Usage:
 *   quarry-executor <script-path>
 *
 * Run metadata is read from stdin as a JSON object with fields:
 * - run_id (string, required)
 * - attempt (number, required)
 * - job_id (string, optional)
 * - parent_run_id (string, optional)
 * - job (any, required) - the job payload
 *
 * Events are written to stdout as length-prefixed msgpack frames.
 * Stderr is used for executor diagnostics (not protocol).
 *
 * Exit codes:
 * - 0: Script completed (run_complete emitted)
 * - 1: Script error (run_error emitted)
 * - 2: Executor crash (no terminal event emitted)
 * - 3: Invalid arguments or input
 *
 * @module
 */
import { execute, parseRunMeta } from '../executor.js'

/**
 * Read all data from stdin.
 */
async function readStdin(): Promise<string> {
  const chunks: Buffer[] = []

  for await (const chunk of process.stdin) {
    chunks.push(chunk)
  }

  return Buffer.concat(chunks).toString('utf-8')
}

/**
 * Main entry point.
 */
async function main(): Promise<never> {
  const args = process.argv.slice(2)

  if (args.length < 1) {
    process.stderr.write('Usage: quarry-executor <script-path>\n')
    process.stderr.write('Run metadata is read from stdin as JSON.\n')
    process.exit(3)
  }

  const scriptPath = args[0]

  // Read and parse stdin
  let input: unknown
  try {
    const stdinData = await readStdin()
    if (stdinData.trim() === '') {
      process.stderr.write('Error: stdin is empty, expected JSON input\n')
      process.exit(3)
    }
    input = JSON.parse(stdinData)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    process.stderr.write(`Error parsing stdin JSON: ${message}\n`)
    process.exit(3)
  }

  if (input === null || typeof input !== 'object') {
    process.stderr.write('Error: stdin must be a JSON object\n')
    process.exit(3)
  }

  const inputObj = input as Record<string, unknown>

  // Parse run metadata
  let run: ReturnType<typeof parseRunMeta>
  try {
    run = parseRunMeta(inputObj)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    process.stderr.write(`Error parsing run metadata: ${message}\n`)
    process.exit(3)
  }

  // Extract job payload
  if (!('job' in inputObj)) {
    process.stderr.write('Error: missing "job" field in input\n')
    process.exit(3)
  }
  const job = inputObj.job

  // Execute
  const result = await execute({
    scriptPath,
    job,
    run,
    output: process.stdout,
    puppeteerOptions: {
      // Headless by default for executor mode
      headless: true,
      // Disable sandbox in containerized environments
      args:
        process.env.QUARRY_NO_SANDBOX === '1' ? ['--no-sandbox', '--disable-setuid-sandbox'] : []
    }
  })

  // Map outcome to exit code
  switch (result.outcome.status) {
    case 'completed':
      process.exit(0)
      break
    case 'error':
      process.exit(1)
      break
    case 'crash':
      process.stderr.write(`Executor crash: ${result.outcome.message}\n`)
      process.exit(2)
      break
    default: {
      // Exhaustiveness check
      const _exhaustive: never = result.outcome
      process.exit(2)
    }
  }
}

main().catch((err) => {
  process.stderr.write(`Unexpected error: ${err instanceof Error ? err.message : String(err)}\n`)
  process.exit(2)
})
