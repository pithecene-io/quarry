/**
 * Unit tests for ctx.storage.put() key computation.
 *
 * Goal: Validate that storage.put() returns the correct Hive-partitioned key
 * and that buildStorageKey() matches the Go buildFilePath() formula.
 *
 * Constraints:
 * - No concurrency
 * - No failures
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { buildStorageKey, createAPIs } from '../../../src/emit-impl'
import type { StoragePartitionMeta } from '../../../src/emit'
import { createRunMeta, FakeSink } from '../_harness'

const testPartition: StoragePartitionMeta = {
  dataset: 'quarry',
  source: 'my-source',
  category: 'default',
  day: '2026-02-23',
  run_id: 'run-001'
}

describe('buildStorageKey()', () => {
  it('produces the correct Hive-partitioned path', () => {
    const key = buildStorageKey(testPartition, 'screenshot.png')
    expect(key).toBe(
      'datasets/quarry/partitions/source=my-source/category=default/day=2026-02-23/run_id=run-001/files/screenshot.png'
    )
  })

  it('handles special characters in filename', () => {
    const key = buildStorageKey(testPartition, 'my file (1).pdf')
    expect(key).toBe(
      'datasets/quarry/partitions/source=my-source/category=default/day=2026-02-23/run_id=run-001/files/my file (1).pdf'
    )
  })

  it('handles custom dataset and category', () => {
    const partition: StoragePartitionMeta = {
      ...testPartition,
      dataset: 'custom-ds',
      category: 'screenshots'
    }
    const key = buildStorageKey(partition, 'page.png')
    expect(key).toBe(
      'datasets/custom-ds/partitions/source=my-source/category=screenshots/day=2026-02-23/run_id=run-001/files/page.png'
    )
  })
})

describe('storage.put() return value', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createRunMeta()
  })

  it('returns StoragePutResult with computed key when partition is provided', async () => {
    const { storage } = createAPIs(run, sink, testPartition)

    const result = await storage.put({
      filename: 'data.json',
      content_type: 'application/json',
      data: Buffer.from('{}')
    })

    expect(result).toEqual({
      key: 'datasets/quarry/partitions/source=my-source/category=default/day=2026-02-23/run_id=run-001/files/data.json'
    })
  })

  it('returns empty key when no partition is provided', async () => {
    const { storage } = createAPIs(run, sink)

    const result = await storage.put({
      filename: 'data.json',
      content_type: 'application/json',
      data: Buffer.from('{}')
    })

    expect(result).toEqual({ key: '' })
  })

  it('writes to sink before returning key', async () => {
    const { storage } = createAPIs(run, sink, testPartition)

    await storage.put({
      filename: 'screenshot.png',
      content_type: 'image/png',
      data: Buffer.from('PNG')
    })

    // Verify writeFile was called
    const fileCalls = sink.allCalls.filter((c) => c.kind === 'writeFile')
    expect(fileCalls).toHaveLength(1)
  })
})
