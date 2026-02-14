import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { loadScript, ScriptLoadError } from '../src/loader.js'

/**
 * Loader tests for hook validation of prepare and beforeTerminal hooks.
 *
 * Writes real .mjs files to a temp directory so loadScript's native
 * import() resolves them. Each test targets validation logic in the
 * loader, not execution behavior (that's covered by executor tests).
 */

let tmpDir: string

beforeAll(async () => {
  tmpDir = await mkdtemp(join(tmpdir(), 'quarry-loader-test-'))
})

afterAll(async () => {
  await rm(tmpDir, { recursive: true, force: true })
})

/** Write a temp .mjs script and return its absolute path. */
async function writeScript(name: string, content: string): Promise<string> {
  const filePath = join(tmpDir, name)
  await writeFile(filePath, content, 'utf-8')
  return filePath
}

describe('loadScript hook validation', () => {
  describe('prepare hook', () => {
    it('accepts a module with a prepare function', async () => {
      const path = await writeScript(
        'prepare-fn.mjs',
        `
        export default async function() {}
        export function prepare() { return { action: 'continue' } }
      `
      )

      const loaded = await loadScript(path)

      expect(loaded.hooks.prepare).toBeTypeOf('function')
    })

    it('accepts a module without prepare (optional)', async () => {
      const path = await writeScript(
        'no-prepare.mjs',
        `
        export default async function() {}
      `
      )

      const loaded = await loadScript(path)

      expect(loaded.hooks.prepare).toBeUndefined()
    })

    it('rejects a module with non-function prepare', async () => {
      const path = await writeScript(
        'prepare-bad.mjs',
        `
        export default async function() {}
        export const prepare = 'not a function'
      `
      )

      await expect(loadScript(path)).rejects.toThrow(ScriptLoadError)
      await expect(loadScript(path)).rejects.toThrow('prepare hook is not a function')
    })
  })

  describe('beforeTerminal hook', () => {
    it('accepts a module with a beforeTerminal function', async () => {
      const path = await writeScript(
        'bt-fn.mjs',
        `
        export default async function() {}
        export async function beforeTerminal() {}
      `
      )

      const loaded = await loadScript(path)

      expect(loaded.hooks.beforeTerminal).toBeTypeOf('function')
    })

    it('accepts a module without beforeTerminal (optional)', async () => {
      const path = await writeScript(
        'no-bt.mjs',
        `
        export default async function() {}
      `
      )

      const loaded = await loadScript(path)

      expect(loaded.hooks.beforeTerminal).toBeUndefined()
    })

    it('rejects a module with non-function beforeTerminal', async () => {
      const path = await writeScript(
        'bt-bad.mjs',
        `
        export default async function() {}
        export const beforeTerminal = { not: 'a function' }
      `
      )

      await expect(loadScript(path)).rejects.toThrow(ScriptLoadError)
      await expect(loadScript(path)).rejects.toThrow('beforeTerminal hook is not a function')
    })
  })

  describe('all hooks together', () => {
    it('accepts a module exporting all hooks', async () => {
      const path = await writeScript(
        'all-hooks.mjs',
        `
        export default async function() {}
        export function prepare() { return { action: 'continue' } }
        export async function beforeRun() {}
        export async function afterRun() {}
        export async function onError() {}
        export async function beforeTerminal() {}
        export async function cleanup() {}
      `
      )

      const loaded = await loadScript(path)

      expect(loaded.hooks.prepare).toBeTypeOf('function')
      expect(loaded.hooks.beforeRun).toBeTypeOf('function')
      expect(loaded.hooks.afterRun).toBeTypeOf('function')
      expect(loaded.hooks.onError).toBeTypeOf('function')
      expect(loaded.hooks.beforeTerminal).toBeTypeOf('function')
      expect(loaded.hooks.cleanup).toBeTypeOf('function')
    })
  })
})
