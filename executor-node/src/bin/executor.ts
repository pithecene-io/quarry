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
import type { ProxyEndpoint } from '@justapithecus/quarry-sdk'
import { execute, parseRunMeta } from '../executor.js'

/**
 * Parse optional proxy endpoint from input.
 * Returns undefined if no proxy is configured.
 */
function parseProxy(input: Record<string, unknown>): ProxyEndpoint | undefined {
  if (!('proxy' in input) || input.proxy === null || input.proxy === undefined) {
    return undefined
  }

  const proxy = input.proxy as Record<string, unknown>

  // Validate required fields
  if (typeof proxy.protocol !== 'string') {
    throw new Error('proxy.protocol must be a string')
  }
  if (typeof proxy.host !== 'string' || proxy.host === '') {
    throw new Error('proxy.host must be a non-empty string')
  }
  if (typeof proxy.port !== 'number' || !Number.isInteger(proxy.port) || proxy.port < 1 || proxy.port > 65535) {
    throw new Error('proxy.port must be an integer between 1 and 65535')
  }

  // Validate protocol
  const validProtocols = ['http', 'https', 'socks5']
  if (!validProtocols.includes(proxy.protocol)) {
    throw new Error(`proxy.protocol must be one of: ${validProtocols.join(', ')}`)
  }

  // Validate auth pair
  const hasUsername = typeof proxy.username === 'string' && proxy.username !== ''
  const hasPassword = typeof proxy.password === 'string' && proxy.password !== ''
  if (hasUsername !== hasPassword) {
    throw new Error('proxy.username and proxy.password must be provided together')
  }

  return {
    protocol: proxy.protocol as 'http' | 'https' | 'socks5',
    host: proxy.host,
    port: proxy.port,
    ...(hasUsername && { username: proxy.username as string }),
    ...(hasPassword && { password: proxy.password as string })
  }
}

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

  // Parse optional proxy
  let proxy: ProxyEndpoint | undefined
  try {
    proxy = parseProxy(inputObj)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    process.stderr.write(`Error parsing proxy: ${message}\n`)
    process.exit(3)
  }

  // Execute
  const result = await execute({
    scriptPath,
    job,
    run,
    proxy,
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
