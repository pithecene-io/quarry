/**
 * Script loader for dynamically importing user scripts.
 *
 * Validates that the loaded module conforms to QuarryScriptModule interface.
 *
 * @module
 */

import { isAbsolute, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import type { QuarryScript, QuarryScriptModule } from '@justapithecus/quarry-sdk'

/**
 * Error thrown when a script module is invalid.
 */
export class ScriptLoadError extends Error {
  constructor(
    public readonly scriptPath: string,
    public readonly reason: string
  ) {
    super(`Failed to load script "${scriptPath}": ${reason}`)
    this.name = 'ScriptLoadError'
  }
}

/**
 * Validated script module with guaranteed default export.
 */
export interface LoadedScript<Job = unknown> {
  /** The main script function */
  readonly script: QuarryScript<Job>
  /** Optional lifecycle hooks */
  readonly hooks: {
    readonly beforeRun?: QuarryScriptModule<Job>['beforeRun']
    readonly afterRun?: QuarryScriptModule<Job>['afterRun']
    readonly onError?: QuarryScriptModule<Job>['onError']
    readonly cleanup?: QuarryScriptModule<Job>['cleanup']
  }
  /** Original module for debugging */
  readonly module: QuarryScriptModule<Job>
}

/**
 * Type guard to check if a value is a function.
 */
function isFunction(value: unknown): value is (...args: unknown[]) => unknown {
  return typeof value === 'function'
}

/**
 * Type guard to check if a value is an optional function (undefined or function).
 */
function isOptionalFunction(
  value: unknown
): value is ((...args: unknown[]) => unknown) | undefined {
  return value === undefined || isFunction(value)
}

/**
 * Load and validate a script module.
 *
 * @param scriptPath - Path to the script (absolute or relative to cwd)
 * @returns Validated script module with hooks
 * @throws ScriptLoadError if the module is invalid or cannot be loaded
 */
export async function loadScript<Job = unknown>(scriptPath: string): Promise<LoadedScript<Job>> {
  // Resolve to absolute path
  const absolutePath = isAbsolute(scriptPath) ? scriptPath : resolve(process.cwd(), scriptPath)

  // Convert to file URL for dynamic import
  const fileUrl = pathToFileURL(absolutePath).href

  let module: unknown
  try {
    module = await import(fileUrl)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    throw new ScriptLoadError(scriptPath, `import failed: ${message}`)
  }

  // Validate module shape
  if (module === null || typeof module !== 'object') {
    throw new ScriptLoadError(scriptPath, 'module is not an object')
  }

  const mod = module as Record<string, unknown>

  // Check for default export
  if (!('default' in mod)) {
    throw new ScriptLoadError(scriptPath, 'missing default export')
  }

  if (!isFunction(mod.default)) {
    throw new ScriptLoadError(scriptPath, 'default export is not a function')
  }

  // Validate optional hooks
  if (!isOptionalFunction(mod.beforeRun)) {
    throw new ScriptLoadError(scriptPath, 'beforeRun hook is not a function')
  }
  if (!isOptionalFunction(mod.afterRun)) {
    throw new ScriptLoadError(scriptPath, 'afterRun hook is not a function')
  }
  if (!isOptionalFunction(mod.onError)) {
    throw new ScriptLoadError(scriptPath, 'onError hook is not a function')
  }
  if (!isOptionalFunction(mod.cleanup)) {
    throw new ScriptLoadError(scriptPath, 'cleanup hook is not a function')
  }

  const validatedModule = mod as QuarryScriptModule<Job>

  return {
    script: validatedModule.default,
    hooks: {
      beforeRun: validatedModule.beforeRun,
      afterRun: validatedModule.afterRun,
      onError: validatedModule.onError,
      cleanup: validatedModule.cleanup
    },
    module: validatedModule
  }
}
