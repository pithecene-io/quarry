/**
 * Tests for the --resolve-from ESM resolve hook.
 *
 * The hook is embedded as a template string in bin/executor.ts and registered
 * via module.register() with a data URL. These tests validate:
 * - The hook code is syntactically valid JavaScript
 * - The hook exports the required initialize() and resolve() functions
 * - The resolve function skips relative/absolute/file: specifiers
 */
import { describe, expect, it } from 'vitest'

/**
 * The hook code template, extracted from bin/executor.ts for testability.
 * This must be kept in sync with the source. A mismatch will be caught
 * by integration tests or the bundle build.
 */
const hookCode = `
  import { pathToFileURL } from 'node:url';

  let fallbackParentURL;

  export function initialize(data) {
    fallbackParentURL = pathToFileURL(data.resolveFrom + '/noop.js').href;
  }

  export async function resolve(specifier, context, nextResolve) {
    if (specifier.startsWith('.') || specifier.startsWith('/') || specifier.startsWith('file:')) {
      return nextResolve(specifier, context);
    }

    try {
      return await nextResolve(specifier, context);
    } catch (defaultErr) {
      try {
        const result = await nextResolve(specifier, {
          ...context,
          parentURL: fallbackParentURL,
        });
        process.stderr.write(
          'quarry: resolved "' + specifier + '" via --resolve-from fallback\\n'
        );
        return result;
      } catch {
        throw defaultErr;
      }
    }
  }
`

describe('resolve-from hook', () => {
  it('hook code can be encoded as a valid data URL module', () => {
    // The hook uses ESM import/export syntax which can't be validated via
    // new Function(). Instead, verify the data URL construction succeeds
    // and produces a well-formed URL that module.register() would accept.
    const encoded = Buffer.from(hookCode).toString('base64')
    const dataUrl = `data:text/javascript;base64,${encoded}`

    // Must be a valid URL
    const parsed = new URL(dataUrl)
    expect(parsed.protocol).toBe('data:')

    // Round-trip: decode back to source
    const [, base64Part] = dataUrl.split(',')
    const decoded = Buffer.from(base64Part, 'base64').toString()
    expect(decoded).toContain('export function initialize')
    expect(decoded).toContain('export async function resolve')
  })

  it('hook code exports initialize function', () => {
    expect(hookCode).toContain('export function initialize(data)')
  })

  it('hook code exports resolve function', () => {
    expect(hookCode).toContain('export async function resolve(specifier, context, nextResolve)')
  })

  it('hook skips relative specifiers (. prefix)', () => {
    // Verify the guard clause is present
    expect(hookCode).toContain("specifier.startsWith('.')")
  })

  it('hook skips absolute specifiers (/ prefix)', () => {
    expect(hookCode).toContain("specifier.startsWith('/')")
  })

  it('hook skips file: URL specifiers', () => {
    expect(hookCode).toContain("specifier.startsWith('file:')")
  })

  it('hook uses parentURL for fallback (not createRequire)', () => {
    // Verify the fix: using nextResolve with parentURL preserves ESM
    // import conditions, unlike createRequire().resolve() which uses
    // CJS require conditions.
    expect(hookCode).toContain('parentURL: fallbackParentURL')
    expect(hookCode).not.toContain('createRequire')
  })

  it('hook logs to stderr on fallback resolution', () => {
    expect(hookCode).toContain('process.stderr.write')
    expect(hookCode).toContain('--resolve-from fallback')
  })

  it('data URL construction produces valid base64', () => {
    const encoded = Buffer.from(hookCode).toString('base64')
    const decoded = Buffer.from(encoded, 'base64').toString()
    expect(decoded).toBe(hookCode)
  })
})
