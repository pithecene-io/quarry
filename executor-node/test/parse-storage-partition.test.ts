/**
 * Unit tests for parseStoragePartition() input validation.
 *
 * Goal: Verify correct parsing, warning on malformed input, and silent
 * pass-through when storage is absent.
 */
import { describe, expect, it, vi } from 'vitest'
import { parseStoragePartition } from '../src/parse-storage-partition.js'

const validStorage = {
  dataset: 'quarry',
  source: 'my-source',
  category: 'default',
  day: '2026-02-23',
  run_id: 'run-001'
}

describe('parseStoragePartition()', () => {
  it('parses valid storage partition', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({ storage: validStorage }, warn)

    expect(result).toEqual(validStorage)
    expect(warn).not.toHaveBeenCalled()
  })

  it('returns undefined when storage is absent', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({}, warn)

    expect(result).toBeUndefined()
    expect(warn).not.toHaveBeenCalled()
  })

  it('returns undefined when storage is null', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({ storage: null }, warn)

    expect(result).toBeUndefined()
    expect(warn).not.toHaveBeenCalled()
  })

  it('returns undefined when storage is undefined', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({ storage: undefined }, warn)

    expect(result).toBeUndefined()
    expect(warn).not.toHaveBeenCalled()
  })

  it('warns when storage is a string', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({ storage: 'bad' }, warn)

    expect(result).toBeUndefined()
    expect(warn).toHaveBeenCalledOnce()
    expect(warn.mock.calls[0][0]).toMatch(/must be an object, got string/)
  })

  it('warns when storage is a number', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({ storage: 42 }, warn)

    expect(result).toBeUndefined()
    expect(warn).toHaveBeenCalledOnce()
    expect(warn.mock.calls[0][0]).toMatch(/must be an object, got number/)
  })

  it('warns when storage is an array', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({ storage: [1, 2] }, warn)

    expect(result).toBeUndefined()
    expect(warn).toHaveBeenCalledOnce()
    expect(warn.mock.calls[0][0]).toMatch(/must be an object, got array/)
  })

  it('warns when required fields are missing', () => {
    const warn = vi.fn()
    const result = parseStoragePartition({ storage: { dataset: 'quarry' } }, warn)

    expect(result).toBeUndefined()
    expect(warn).toHaveBeenCalledOnce()
    expect(warn.mock.calls[0][0]).toMatch(/missing\/empty: source, category, day, run_id/)
  })

  it('warns when a required field is empty string', () => {
    const warn = vi.fn()
    const result = parseStoragePartition(
      { storage: { ...validStorage, day: '' } },
      warn
    )

    expect(result).toBeUndefined()
    expect(warn).toHaveBeenCalledOnce()
    expect(warn.mock.calls[0][0]).toMatch(/missing\/empty: day/)
  })

  it('warns when a required field is non-string type', () => {
    const warn = vi.fn()
    const result = parseStoragePartition(
      { storage: { ...validStorage, source: 123 } },
      warn
    )

    expect(result).toBeUndefined()
    expect(warn).toHaveBeenCalledOnce()
    expect(warn.mock.calls[0][0]).toMatch(/missing\/empty: source/)
  })
})
