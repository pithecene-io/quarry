/**
 * Factory for creating test RunMeta objects.
 */
import { randomUUID } from 'node:crypto'
import type { RunMeta } from '../../../src/types/context'
import type { JobId, RunId } from '../../../src/types/events'

export interface CreateRunMetaOptions {
  run_id?: RunId
  job_id?: JobId
  parent_run_id?: RunId
  attempt?: number
}

/**
 * Create a RunMeta for testing.
 * Generates a random run_id if not provided.
 */
export function createRunMeta(options: CreateRunMetaOptions = {}): RunMeta {
  return {
    run_id: options.run_id ?? (randomUUID() as RunId),
    attempt: options.attempt ?? 1,
    ...(options.job_id !== undefined && { job_id: options.job_id }),
    ...(options.parent_run_id !== undefined && { parent_run_id: options.parent_run_id })
  }
}

/**
 * Create a deterministic RunMeta for golden tests.
 */
export function createDeterministicRunMeta(): RunMeta {
  return {
    run_id: '00000000-0000-0000-0000-000000000001' as RunId,
    job_id: '00000000-0000-0000-0000-000000000002' as JobId,
    attempt: 1
  }
}
