import type { Browser, BrowserContext, Page } from 'puppeteer'
import type { EmitSink, StoragePartitionMeta } from './emit'
import { createAPIs } from './emit-impl'
import type { MemoryThresholds } from './memory'
import { createMemoryAPI } from './memory'
import type { QuarryContext, RunMeta } from './types/context'

/**
 * Options for creating a QuarryContext.
 * Used by executor-node to construct the context.
 *
 * @internal
 */
export type CreateContextOptions<Job = unknown> = {
  job: Job
  run: RunMeta
  page: Page
  browser: Browser
  browserContext: BrowserContext
  sink: EmitSink
  /** Storage partition metadata for SDK-side key computation. @internal */
  storagePartition?: StoragePartitionMeta
  /** Custom memory pressure thresholds. @internal */
  memoryThresholds?: MemoryThresholds
}

/**
 * Create a QuarryContext instance.
 * This is called by the executor-node, not by user scripts.
 *
 * @internal
 */
export function createContext<Job = unknown>(
  options: CreateContextOptions<Job>
): QuarryContext<Job> {
  const { emit, storage } = createAPIs(options.run, options.sink, options.storagePartition)
  const memory = createMemoryAPI({
    page: options.page,
    thresholds: options.memoryThresholds
  })

  const ctx: QuarryContext<Job> = {
    job: options.job,
    run: Object.freeze(options.run),
    page: options.page,
    browser: options.browser,
    browserContext: options.browserContext,
    emit,
    storage,
    memory
  }

  return Object.freeze(ctx)
}
