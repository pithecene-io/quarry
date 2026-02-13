import type { EmitAPI } from './emit'

/**
 * Options for creating a batcher.
 */
export type BatcherOptions = {
  /** Items per batch before auto-flush (>= 1) */
  readonly size: number
  /** Target for all enqueue events */
  readonly target: string
  /** Optional source partition override */
  readonly source?: string
  /** Optional category partition override */
  readonly category?: string
}

/**
 * Accumulates items and emits fewer, larger enqueue events.
 * Each flush emits a single enqueue with `params.items` containing the batch.
 */
export type Batcher<T = Record<string, unknown>> = {
  /** Accumulate an item; auto-flushes when buffer reaches configured size. */
  readonly add: (item: T) => Promise<void>
  /**
   * Emit any buffered items as a single enqueue event. No-op if empty.
   *
   * On failure, buffered items are lost. This is consistent with emit's
   * fail-fast semantics — after a sink failure the emit chain is poisoned
   * and items can never be delivered.
   */
  readonly flush: () => Promise<void>
  /** Number of unflushed items currently buffered. */
  readonly pending: number
}

/**
 * Create a batcher that accumulates items and emits batched enqueue events.
 *
 * Reduces child run count for fan-out workloads by grouping items into
 * fewer, larger enqueue payloads.
 */
export function createBatcher<T = Record<string, unknown>>(
  emit: Pick<EmitAPI, 'enqueue'>,
  options: BatcherOptions
): Batcher<T> {
  if (!Number.isFinite(options.size) || options.size < 1) {
    throw new RangeError(`Batcher size must be a finite number >= 1, got ${options.size}`)
  }

  const buffer: T[] = []

  async function flush(): Promise<void> {
    if (buffer.length === 0) return
    // Drain buffer synchronously before async emit (single-threaded safety)
    const items = buffer.splice(0)
    await emit.enqueue({
      target: options.target,
      params: { items },
      ...(options.source !== undefined && { source: options.source }),
      ...(options.category !== undefined && { category: options.category })
    })
  }

  return {
    async add(item: T): Promise<void> {
      buffer.push(item)
      if (buffer.length >= options.size) {
        await flush()
      }
    },
    flush,
    get pending(): number {
      return buffer.length
    }
  }
}
