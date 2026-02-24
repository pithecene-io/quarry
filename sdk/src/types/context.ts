import type { Browser, BrowserContext, Page } from 'puppeteer'
import type { EmitAPI, StorageAPI } from '../emit'
import type { MemoryAPI } from '../memory'
import type { JobId, RunId } from './events'

/**
 * Run metadata available to scripts.
 * From CONTRACT_RUN.md.
 */
export type RunMeta = {
  /** Canonical run identifier */
  readonly run_id: RunId
  /** Job ID, may be undefined if not known */
  readonly job_id?: JobId
  /** Parent run ID for retries */
  readonly parent_run_id?: RunId
  /** Attempt number (starts at 1) */
  readonly attempt: number
}

/**
 * The main context object passed to extraction scripts.
 * Generic over the Job type to allow user-defined job payloads.
 *
 * @template Job - User-defined job payload type
 */
export type QuarryContext<Job = unknown> = {
  /**
   * The job payload provided when the run was initiated.
   * Type is user-defined via generic parameter.
   * Immutable for the duration of the run.
   */
  readonly job: Job

  /**
   * Run metadata (run_id, job_id, attempt, etc.)
   */
  readonly run: RunMeta

  /**
   * The Puppeteer Page instance.
   * This is the real Puppeteer object, not a wrapper.
   */
  readonly page: Page

  /**
   * The Puppeteer Browser instance.
   * This is the real Puppeteer object, not a wrapper.
   */
  readonly browser: Browser

  /**
   * The Puppeteer BrowserContext.
   * This is the real Puppeteer object, not a wrapper.
   */
  readonly browserContext: BrowserContext

  /**
   * The emit API for outputting data from the script.
   * This is the sole output mechanism.
   */
  readonly emit: EmitAPI

  /**
   * Storage API for sidecar file uploads.
   * Files land at Hive-partitioned paths under files/, bypassing
   * Dataset segment/manifest machinery.
   */
  readonly storage: StorageAPI

  /**
   * Memory pressure API for proactive memory management.
   * Provides node, browser, and cgroup usage snapshots with pressure classification.
   */
  readonly memory: MemoryAPI
}

/**
 * Script function signature.
 * Scripts export a default function conforming to this type.
 *
 * @template Job - User-defined job payload type
 */
export type QuarryScript<Job = unknown> = (ctx: QuarryContext<Job>) => Promise<void>
