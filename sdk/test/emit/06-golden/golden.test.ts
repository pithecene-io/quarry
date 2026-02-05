/**
 * Golden/wire format tests.
 *
 * Goal: Freeze the wire contract.
 * Invariant: Any schema change must be deliberate.
 *
 * Process:
 * - Emit known sequence
 * - Serialize envelopes to JSON
 * - Compare to checked-in snapshots (ignoring dynamic fields)
 */

import { readFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { beforeEach, describe, expect, it } from 'vitest'
import { createEmitAPI } from '../../../src/emit-impl'
import type { CheckpointId, EventEnvelope } from '../../../src/types/events'
import { CONTRACT_VERSION } from '../../../src/types/events'
import { createDeterministicRunMeta, FakeSink } from '../_harness'

const __dirname = dirname(fileURLToPath(import.meta.url))

/**
 * Normalize an envelope for comparison by replacing dynamic fields.
 */
function normalizeEnvelope(envelope: EventEnvelope): Record<string, unknown> {
  const normalized: Record<string, unknown> = { ...envelope }

  // Replace dynamic fields with placeholders
  normalized.event_id = 'EVENT_ID_PLACEHOLDER'
  normalized.ts = 'TIMESTAMP_PLACEHOLDER'

  // Normalize artifact_id in payload if present
  if (envelope.type === 'artifact' && 'artifact_id' in envelope.payload) {
    normalized.payload = {
      ...envelope.payload,
      artifact_id: 'ARTIFACT_ID_PLACEHOLDER'
    }
  }

  return normalized
}

/**
 * Load and parse a golden file.
 */
async function loadGolden(filename: string): Promise<Record<string, unknown>[]> {
  const path = join(__dirname, filename)
  const content = await readFile(path, 'utf-8')
  return JSON.parse(content)
}

describe('golden wire format - simple run', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createDeterministicRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createDeterministicRunMeta()
  })

  it('matches simple-run.json golden file', async () => {
    const emit = createEmitAPI(run, sink)

    // Emit the exact sequence from the golden file
    await emit.item({
      item_type: 'product',
      data: { name: 'Widget', price: 9.99 }
    })
    await emit.log({ level: 'info', message: 'Processing complete' })
    await emit.runComplete({ summary: { items_collected: 1 } })

    // Load golden file
    const golden = await loadGolden('simple-run.json')

    // Compare
    expect(sink.envelopes).toHaveLength(golden.length)

    for (let i = 0; i < sink.envelopes.length; i++) {
      const actual = normalizeEnvelope(sink.envelopes[i])
      const expected = golden[i]

      expect(actual).toEqual(expected)
    }
  })

  it('verifies contract_version matches source constant', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.item({ item_type: 'test', data: {} })

    expect(sink.envelopes[0].contract_version).toBe(CONTRACT_VERSION)
  })

  it('verifies run_id wiring from RunMeta', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.item({ item_type: 'test', data: {} })

    expect(sink.envelopes[0].run_id).toBe('00000000-0000-0000-0000-000000000001')
  })

  it('verifies job_id wiring from RunMeta', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.item({ item_type: 'test', data: {} })

    expect(sink.envelopes[0].job_id).toBe('00000000-0000-0000-0000-000000000002')
  })

  it('verifies attempt is always present', async () => {
    const emit = createEmitAPI(run, sink)
    await emit.item({ item_type: 'test', data: {} })
    await emit.log({ level: 'info', message: 'test' })
    await emit.runComplete()

    for (const envelope of sink.envelopes) {
      expect(envelope.attempt).toBeDefined()
      expect(envelope.attempt).toBe(1)
    }
  })
})

describe('golden wire format - artifact run', () => {
  let sink: FakeSink
  let run: ReturnType<typeof createDeterministicRunMeta>

  beforeEach(() => {
    sink = new FakeSink()
    run = createDeterministicRunMeta()
  })

  it('matches artifact-run.json golden file', async () => {
    const emit = createEmitAPI(run, sink)

    // Emit the exact sequence from the golden file
    await emit.item({
      item_type: 'page',
      data: { url: 'https://example.com', title: 'Example' }
    })
    await emit.artifact({
      name: 'screenshot.png',
      content_type: 'image/png',
      data: Buffer.from('fake png data') // 12 bytes to match golden
    })
    await emit.checkpoint({
      checkpoint_id: 'page-captured' as CheckpointId,
      note: 'Screenshot saved'
    })
    await emit.runComplete()

    // Load golden file
    const golden = await loadGolden('artifact-run.json')

    // Compare
    expect(sink.envelopes).toHaveLength(golden.length)

    for (let i = 0; i < sink.envelopes.length; i++) {
      const actual = normalizeEnvelope(sink.envelopes[i])
      const expected = golden[i]

      expect(actual).toEqual(expected)
    }
  })

  it('verifies artifact size_bytes matches data length', async () => {
    const emit = createEmitAPI(run, sink)
    const data = Buffer.from('fake png data')

    await emit.artifact({
      name: 'screenshot.png',
      content_type: 'image/png',
      data
    })

    const artifactEnvelope = sink.envelopes[0]
    expect(artifactEnvelope.type).toBe('artifact')
    expect(artifactEnvelope.payload).toMatchObject({
      size_bytes: data.byteLength
    })
  })
})

describe('golden format stability', () => {
  it('all event types have consistent structure', async () => {
    const sink = new FakeSink()
    const run = createDeterministicRunMeta()
    const emit = createEmitAPI(run, sink)

    // Emit one of each non-terminal type
    await emit.item({ item_type: 'test', data: {} })
    await emit.log({ level: 'debug', message: 'test' })
    await emit.artifact({ name: 'test', content_type: 'text/plain', data: Buffer.from('') })
    await emit.checkpoint({ checkpoint_id: 'cp' as CheckpointId })
    await emit.enqueue({ target: 'next', params: {} })
    await emit.rotateProxy({ reason: 'test' })
    await emit.runComplete()

    // All envelopes should have base fields
    for (const envelope of sink.envelopes) {
      expect(envelope).toHaveProperty('contract_version')
      expect(envelope).toHaveProperty('event_id')
      expect(envelope).toHaveProperty('run_id')
      expect(envelope).toHaveProperty('seq')
      expect(envelope).toHaveProperty('ts')
      expect(envelope).toHaveProperty('attempt')
      expect(envelope).toHaveProperty('type')
      expect(envelope).toHaveProperty('payload')
    }
  })

  it('seq starts at 1 and increments', async () => {
    const sink = new FakeSink()
    const run = createDeterministicRunMeta()
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'a', data: {} })
    await emit.item({ item_type: 'b', data: {} })
    await emit.item({ item_type: 'c', data: {} })

    expect(sink.envelopes.map((e) => e.seq)).toEqual([1, 2, 3])
  })

  it('ts is valid ISO 8601', async () => {
    const sink = new FakeSink()
    const run = createDeterministicRunMeta()
    const emit = createEmitAPI(run, sink)

    await emit.item({ item_type: 'test', data: {} })

    const ts = sink.envelopes[0].ts
    const parsed = new Date(ts)

    expect(parsed.toISOString()).toBe(ts)
  })
})

describe('golden schema invariants', () => {
  it('item payload has exactly item_type and data', async () => {
    const sink = new FakeSink()
    const emit = createEmitAPI(createDeterministicRunMeta(), sink)

    await emit.item({ item_type: 'test', data: { key: 'value' } })

    const payload = sink.envelopes[0].payload
    expect(Object.keys(payload).sort()).toEqual(['data', 'item_type'])
  })

  it('artifact payload has exactly artifact_id, name, content_type, size_bytes', async () => {
    const sink = new FakeSink()
    const emit = createEmitAPI(createDeterministicRunMeta(), sink)

    await emit.artifact({ name: 'test', content_type: 'text/plain', data: Buffer.from('') })

    const payload = sink.envelopes[0].payload
    expect(Object.keys(payload).sort()).toEqual([
      'artifact_id',
      'content_type',
      'name',
      'size_bytes'
    ])
  })

  it('log payload has level, message, and optionally fields', async () => {
    const sink = new FakeSink()
    const emit = createEmitAPI(createDeterministicRunMeta(), sink)

    await emit.log({ level: 'info', message: 'test' })
    await emit.log({ level: 'info', message: 'with fields', fields: { key: 'value' } })

    expect(Object.keys(sink.envelopes[0].payload).sort()).toEqual(['level', 'message'])
    expect(Object.keys(sink.envelopes[1].payload).sort()).toEqual(['fields', 'level', 'message'])
  })

  it('run_complete payload has optionally summary', async () => {
    const sink = new FakeSink()
    const emit1 = createEmitAPI(createDeterministicRunMeta(), sink)

    await emit1.runComplete()
    expect(Object.keys(sink.envelopes[0].payload)).toEqual([])

    sink.reset()
    const emit2 = createEmitAPI(createDeterministicRunMeta(), sink)

    await emit2.runComplete({ summary: { count: 1 } })
    expect(Object.keys(sink.envelopes[0].payload)).toEqual(['summary'])
  })

  it('run_error payload has error_type, message, and optionally stack', async () => {
    const sink = new FakeSink()
    const emit1 = createEmitAPI(createDeterministicRunMeta(), sink)

    await emit1.runError({ error_type: 'test', message: 'error' })
    expect(Object.keys(sink.envelopes[0].payload).sort()).toEqual(['error_type', 'message'])

    sink.reset()
    const emit2 = createEmitAPI(createDeterministicRunMeta(), sink)

    await emit2.runError({ error_type: 'test', message: 'error', stack: 'trace' })
    expect(Object.keys(sink.envelopes[0].payload).sort()).toEqual([
      'error_type',
      'message',
      'stack'
    ])
  })
})
