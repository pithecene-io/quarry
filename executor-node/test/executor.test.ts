import { describe, it, expect, vi } from 'vitest'
import type { RunId, JobId } from '@justapithecus/quarry-sdk'
import { parseRunMeta } from '../src/executor.js'

describe('parseRunMeta', () => {
  describe('required field validation', () => {
    it('throws on null input', () => {
      expect(() => parseRunMeta(null)).toThrow('run metadata must be an object')
    })

    it('throws on non-object input', () => {
      expect(() => parseRunMeta('string')).toThrow('run metadata must be an object')
      expect(() => parseRunMeta(123)).toThrow('run metadata must be an object')
    })

    it('throws on missing run_id', () => {
      expect(() => parseRunMeta({ attempt: 1 })).toThrow('run_id must be a non-empty string')
    })

    it('throws on empty run_id', () => {
      expect(() => parseRunMeta({ run_id: '', attempt: 1 })).toThrow(
        'run_id must be a non-empty string'
      )
    })

    it('throws on missing attempt', () => {
      expect(() => parseRunMeta({ run_id: 'run-123' })).toThrow('attempt must be a positive integer')
    })

    it('throws on non-integer attempt', () => {
      expect(() => parseRunMeta({ run_id: 'run-123', attempt: 1.5 })).toThrow(
        'attempt must be a positive integer'
      )
    })

    it('throws on attempt < 1', () => {
      expect(() => parseRunMeta({ run_id: 'run-123', attempt: 0 })).toThrow(
        'attempt must be a positive integer'
      )
      expect(() => parseRunMeta({ run_id: 'run-123', attempt: -1 })).toThrow(
        'attempt must be a positive integer'
      )
    })
  })

  describe('valid input parsing', () => {
    it('parses minimal valid input', () => {
      const run = parseRunMeta({ run_id: 'run-123', attempt: 1 })

      expect(run.run_id).toBe('run-123')
      expect(run.attempt).toBe(1)
      expect(run.job_id).toBeUndefined()
      expect(run.parent_run_id).toBeUndefined()
    })

    it('parses input with job_id', () => {
      const run = parseRunMeta({ run_id: 'run-123', attempt: 1, job_id: 'job-456' })

      expect(run.job_id).toBe('job-456')
    })

    it('ignores empty job_id', () => {
      const run = parseRunMeta({ run_id: 'run-123', attempt: 1, job_id: '' })

      expect(run.job_id).toBeUndefined()
    })
  })

  describe('lineage validation', () => {
    it('excludes parent_run_id for initial run (attempt === 1)', () => {
      const run = parseRunMeta({
        run_id: 'run-123',
        attempt: 1,
        parent_run_id: 'run-parent'
      })

      // parent_run_id should be excluded for initial runs
      expect(run.parent_run_id).toBeUndefined()
    })

    it('includes parent_run_id for retry run (attempt > 1)', () => {
      const run = parseRunMeta({
        run_id: 'run-123',
        attempt: 2,
        parent_run_id: 'run-parent'
      })

      expect(run.parent_run_id).toBe('run-parent')
    })

    it('allows retry run without parent_run_id (with warning)', () => {
      const diagnostics = vi.fn()
      const run = parseRunMeta(
        {
          run_id: 'run-123',
          attempt: 3
        },
        diagnostics
      )

      expect(run.parent_run_id).toBeUndefined()
      expect(diagnostics).toHaveBeenCalledWith(
        expect.stringContaining('attempt is 3 but parent_run_id is missing')
      )
    })

    it('warns when initial run has parent_run_id', () => {
      const diagnostics = vi.fn()
      parseRunMeta(
        {
          run_id: 'run-123',
          attempt: 1,
          parent_run_id: 'run-parent'
        },
        diagnostics
      )

      expect(diagnostics).toHaveBeenCalledWith(
        expect.stringContaining('attempt is 1 but parent_run_id is present')
      )
    })

    it('does not warn for valid initial run', () => {
      const diagnostics = vi.fn()
      parseRunMeta(
        {
          run_id: 'run-123',
          attempt: 1
        },
        diagnostics
      )

      expect(diagnostics).not.toHaveBeenCalled()
    })

    it('does not warn for valid retry run', () => {
      const diagnostics = vi.fn()
      parseRunMeta(
        {
          run_id: 'run-123',
          attempt: 2,
          parent_run_id: 'run-parent'
        },
        diagnostics
      )

      expect(diagnostics).not.toHaveBeenCalled()
    })
  })

  describe('type branding', () => {
    it('returns branded types', () => {
      const run = parseRunMeta({
        run_id: 'run-123',
        attempt: 2,
        job_id: 'job-456',
        parent_run_id: 'run-parent'
      })

      // TypeScript would catch misuse, but we can verify the values
      const runId: RunId = run.run_id
      const jobId: JobId | undefined = run.job_id
      const parentRunId: RunId | undefined = run.parent_run_id

      expect(runId).toBe('run-123')
      expect(jobId).toBe('job-456')
      expect(parentRunId).toBe('run-parent')
    })
  })
})
