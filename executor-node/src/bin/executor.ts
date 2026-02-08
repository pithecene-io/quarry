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
import { errorMessage, execute, parseRunMeta } from '../executor.js'

/**
 * Write an error message to stderr and exit with code 3 (invalid input).
 */
function fatalError(message: string): never {
  process.stderr.write(`Error: ${message}\n`)
  process.exit(3)
}

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
  if (
    typeof proxy.port !== 'number' ||
    !Number.isInteger(proxy.port) ||
    proxy.port < 1 ||
    proxy.port > 65535
  ) {
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
 * Launch a shared browser and print its WebSocket endpoint to stdout.
 * Stays alive until stdin closes, then shuts down the browser.
 *
 * Used by the Go runtime to provide a managed browser for fan-out runs.
 * The script path is used to resolve puppeteer from the user's project.
 */
async function launchBrowserServer(scriptPath: string): Promise<never> {
  const { getPuppeteer: getBrowserPuppeteer } = await import('../executor.js')

  const puppeteer = await getBrowserPuppeteer(scriptPath, {
    stealth: process.env.QUARRY_STEALTH !== '0',
    adblocker: process.env.QUARRY_ADBLOCKER === '1'
  })

  const browser = await puppeteer.launch({
    headless: true,
    args: process.env.QUARRY_NO_SANDBOX === '1' ? ['--no-sandbox', '--disable-setuid-sandbox'] : []
  })

  // Print WS endpoint to stdout (Go runtime reads this line)
  const wsEndpoint = browser.wsEndpoint()
  process.stdout.write(`${wsEndpoint}\n`)

  // Stay alive until stdin closes (Go runtime closes stdin to signal shutdown)
  await new Promise<void>((resolve) => {
    process.stdin.resume()
    process.stdin.on('end', resolve)
    process.stdin.on('close', resolve)
    process.on('SIGTERM', resolve)
    process.on('SIGINT', resolve)
  })

  await browser.close()
  process.exit(0)
}

/**
 * Main entry point.
 */
async function main(): Promise<never> {
  const args = process.argv.slice(2)

  // Browser server mode: launch shared browser for fan-out
  if (args[0] === '--launch-browser') {
    const scriptPath = args[1]
    if (!scriptPath) {
      process.stderr.write('Usage: quarry-executor --launch-browser <script-path>\n')
      process.exit(3)
    }
    return launchBrowserServer(scriptPath)
  }

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
      fatalError('stdin is empty, expected JSON input')
    }
    input = JSON.parse(stdinData)
  } catch (err) {
    fatalError(`parsing stdin JSON: ${errorMessage(err)}`)
  }

  if (input === null || typeof input !== 'object') {
    fatalError('stdin must be a JSON object')
  }

  const inputObj = input as Record<string, unknown>

  // Parse run metadata
  let run: ReturnType<typeof parseRunMeta>
  try {
    run = parseRunMeta(inputObj)
  } catch (err) {
    fatalError(`parsing run metadata: ${errorMessage(err)}`)
  }

  // Extract job payload
  if (!('job' in inputObj)) {
    fatalError('missing "job" field in input')
  }
  const job = inputObj.job

  // Parse optional proxy
  let proxy: ProxyEndpoint | undefined
  try {
    proxy = parseProxy(inputObj)
  } catch (err) {
    fatalError(`parsing proxy: ${errorMessage(err)}`)
  }

  // Parse optional browser WebSocket endpoint
  const browserWSEndpoint =
    typeof inputObj.browser_ws_endpoint === 'string' && inputObj.browser_ws_endpoint !== ''
      ? inputObj.browser_ws_endpoint
      : undefined

  // Execute
  const result = await execute({
    scriptPath,
    job,
    run,
    proxy,
    browserWSEndpoint,
    output: process.stdout,
    puppeteerOptions: {
      // Headless by default for executor mode
      headless: true,
      // Disable sandbox in containerized environments
      args:
        process.env.QUARRY_NO_SANDBOX === '1' ? ['--no-sandbox', '--disable-setuid-sandbox'] : []
    },
    // Stealth on by default; disable with QUARRY_STEALTH=0
    stealth: process.env.QUARRY_STEALTH !== '0',
    // Adblocker off by default; enable with QUARRY_ADBLOCKER=1
    adblocker: process.env.QUARRY_ADBLOCKER === '1'
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
  process.stderr.write(`Unexpected error: ${errorMessage(err)}\n`)
  process.exit(2)
})
