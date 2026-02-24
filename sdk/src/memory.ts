import { readFileSync } from 'node:fs'
import v8 from 'node:v8'
import type { Page } from 'puppeteer'

/**
 * Memory pressure classification.
 */
export type MemoryPressureLevel = 'low' | 'moderate' | 'high' | 'critical'

/**
 * Memory usage for a single source (node, browser, or cgroup).
 */
export type MemoryUsage = {
  /** Bytes in use */
  readonly used: number
  /** Bytes available (heap limit or cgroup limit) */
  readonly limit: number
  /** Usage ratio 0.0 to 1.0 */
  readonly ratio: number
}

/**
 * A point-in-time memory snapshot across all available sources.
 */
export type MemorySnapshot = {
  /** Always available (v8 heap stats) */
  readonly node: MemoryUsage
  /** null if no browser or opted out */
  readonly browser: MemoryUsage | null
  /** null if not in a cgroup */
  readonly cgroup: MemoryUsage | null
  /** Highest pressure level across all sources */
  readonly pressure: MemoryPressureLevel
  /** ISO 8601 UTC timestamp */
  readonly ts: string
}

/**
 * Custom pressure thresholds.
 */
export type MemoryThresholds = {
  /** Ratio threshold for moderate pressure. Default: 0.5 */
  readonly moderate?: number
  /** Ratio threshold for high pressure. Default: 0.7 */
  readonly high?: number
  /** Ratio threshold for critical pressure. Default: 0.9 */
  readonly critical?: number
}

/**
 * Memory pressure API for proactive memory management.
 */
export type MemoryAPI = {
  /** Snapshot current memory state. Browser metrics involve a CDP call. */
  readonly snapshot: (options?: { browser?: boolean }) => Promise<MemorySnapshot>
  /** Convenience: check if any source is at or above the given level. */
  readonly isAbove: (level: MemoryPressureLevel) => Promise<boolean>
}

const PRESSURE_ORDER: readonly MemoryPressureLevel[] = ['low', 'moderate', 'high', 'critical']

const DEFAULT_THRESHOLDS: Required<MemoryThresholds> = {
  moderate: 0.5,
  high: 0.7,
  critical: 0.9
}

/**
 * Validate that thresholds are in (0, 1] and strictly monotonic.
 */
function validateThresholds(t: Required<MemoryThresholds>): void {
  for (const [name, value] of Object.entries(t) as Array<[string, number]>) {
    if (value <= 0 || value > 1) {
      throw new RangeError(`MemoryThreshold "${name}" must be in (0, 1], got ${value}`)
    }
  }
  if (t.moderate >= t.high) {
    throw new RangeError(
      `MemoryThresholds must be monotonic: moderate (${t.moderate}) must be < high (${t.high})`
    )
  }
  if (t.high >= t.critical) {
    throw new RangeError(
      `MemoryThresholds must be monotonic: high (${t.high}) must be < critical (${t.critical})`
    )
  }
}

/**
 * Classify a ratio into a pressure level.
 */
function classifyPressure(
  ratio: number,
  thresholds: Required<MemoryThresholds>
): MemoryPressureLevel {
  if (ratio >= thresholds.critical) return 'critical'
  if (ratio >= thresholds.high) return 'high'
  if (ratio >= thresholds.moderate) return 'moderate'
  return 'low'
}

/**
 * Return the highest pressure level among candidates.
 */
function highestPressure(levels: MemoryPressureLevel[]): MemoryPressureLevel {
  let max = 0
  for (const level of levels) {
    const idx = PRESSURE_ORDER.indexOf(level)
    if (idx > max) max = idx
  }
  return PRESSURE_ORDER[max]
}

/**
 * Read Node.js heap usage via v8 and process.memoryUsage().
 */
function readNodeUsage(): MemoryUsage {
  const heapStats = v8.getHeapStatistics()
  const used = process.memoryUsage().heapUsed
  const limit = heapStats.heap_size_limit
  return { used, limit, ratio: limit > 0 ? used / limit : 0 }
}

/**
 * Read browser heap usage via page.metrics() CDP call.
 * Returns null on failure.
 */
async function readBrowserUsage(page: Page | null): Promise<MemoryUsage | null> {
  if (!page) return null
  try {
    const m = await page.metrics()
    const used = m.JSHeapUsedSize ?? 0
    const limit = m.JSHeapTotalSize ?? 0
    return { used, limit, ratio: limit > 0 ? used / limit : 0 }
  } catch {
    return null
  }
}

// Cgroup v1 uses a large sentinel (typically 2^63 - page_size) to indicate
// "no limit". Any value above 2^62 bytes (4 EiB) is treated as unlimited.
const CGROUP_UNLIMITED_THRESHOLD = 2 ** 62

/**
 * Read cgroup memory usage.
 * Tries cgroup v2 paths first, falls back to v1.
 * Returns null if not in a cgroup or if the limit is unlimited.
 */
function readCgroupUsage(
  readFile: (path: string) => number | null = readCgroupFile
): MemoryUsage | null {
  // cgroup v2
  const v2Current = readFile('/sys/fs/cgroup/memory.current')
  const v2Max = readFile('/sys/fs/cgroup/memory.max')
  if (v2Current !== null && v2Max !== null) {
    return { used: v2Current, limit: v2Max, ratio: v2Max > 0 ? v2Current / v2Max : 0 }
  }

  // cgroup v1 fallback
  const v1Usage = readFile('/sys/fs/cgroup/memory/memory.usage_in_bytes')
  const v1Limit = readFile('/sys/fs/cgroup/memory/memory.limit_in_bytes')
  if (v1Usage !== null && v1Limit !== null) {
    if (v1Limit >= CGROUP_UNLIMITED_THRESHOLD) return null
    return { used: v1Usage, limit: v1Limit, ratio: v1Limit > 0 ? v1Usage / v1Limit : 0 }
  }

  return null
}

/**
 * Read a cgroup file and parse as integer.
 * Returns null if file doesn't exist, is unreadable, or contains "max".
 */
function readCgroupFile(path: string): number | null {
  try {
    const content = readFileSync(path, 'utf8').trim()
    if (content === 'max') return null
    const value = Number.parseInt(content, 10)
    return Number.isNaN(value) ? null : value
  } catch {
    return null
  }
}

/**
 * Options for createMemoryAPI.
 *
 * @internal
 */
export type CreateMemoryAPIOptions = {
  /** Puppeteer page for browser metrics. May be null for headless-only scripts. */
  readonly page: Page | null
  /** Custom pressure thresholds. */
  readonly thresholds?: MemoryThresholds
  /** Override cgroup reader for testing. @internal */
  readonly _readCgroup?: () => MemoryUsage | null
  /** Override cgroup file reader for testing (exercises real readCgroupUsage logic). @internal */
  readonly _readCgroupFile?: (path: string) => number | null
  /** Override node reader for testing. @internal */
  readonly _readNode?: () => MemoryUsage
  /** Override browser reader for testing. @internal */
  readonly _readBrowser?: (page: Page | null) => Promise<MemoryUsage | null>
}

/**
 * Create a MemoryAPI instance.
 *
 * @internal
 */
export function createMemoryAPI(options: CreateMemoryAPIOptions): MemoryAPI {
  const thresholds: Required<MemoryThresholds> = {
    ...DEFAULT_THRESHOLDS,
    ...options.thresholds
  }
  validateThresholds(thresholds)

  const readNode = options._readNode ?? readNodeUsage
  const readBrowser = options._readBrowser ?? readBrowserUsage
  const readCgroup = options._readCgroup ?? (() => readCgroupUsage(options._readCgroupFile))

  async function snapshot(opts?: { browser?: boolean }): Promise<MemorySnapshot> {
    const includeBrowser = opts?.browser !== false
    const node = readNode()
    const browser = includeBrowser ? await readBrowser(options.page) : null
    const cgroup = readCgroup()

    const levels: MemoryPressureLevel[] = [classifyPressure(node.ratio, thresholds)]
    if (browser) levels.push(classifyPressure(browser.ratio, thresholds))
    if (cgroup) levels.push(classifyPressure(cgroup.ratio, thresholds))

    return {
      node,
      browser,
      cgroup,
      pressure: highestPressure(levels),
      ts: new Date().toISOString()
    }
  }

  return {
    snapshot,
    async isAbove(level: MemoryPressureLevel): Promise<boolean> {
      const snap = await snapshot()
      return PRESSURE_ORDER.indexOf(snap.pressure) >= PRESSURE_ORDER.indexOf(level)
    }
  }
}
