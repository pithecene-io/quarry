/**
 * Tests for the --resolve-from ESM resolve hook.
 *
 * Imports the hook code from the production module (src/resolve-from.ts)
 * and validates its structure. These tests catch:
 * - Missing required exports (initialize, resolve)
 * - Incorrect specifier guard clauses
 * - Regression to CJS createRequire (vs ESM parentURL)
 * - Missing observability (stderr logging)
 * - Data URL encoding correctness
 */
import { describe, expect, it } from 'vitest'
import { resolveFromHookCode } from '../src/resolve-from.js'

describe('resolve-from hook', () => {
  it('hook code can be encoded as a valid data URL module', () => {
    const encoded = Buffer.from(resolveFromHookCode).toString('base64')
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
    expect(resolveFromHookCode).toContain('export function initialize(data)')
  })

  it('hook code exports resolve function', () => {
    expect(resolveFromHookCode).toContain(
      'export async function resolve(specifier, context, nextResolve)'
    )
  })

  it('hook skips relative specifiers (. prefix)', () => {
    expect(resolveFromHookCode).toContain("specifier.startsWith('.')")
  })

  it('hook skips absolute specifiers (/ prefix)', () => {
    expect(resolveFromHookCode).toContain("specifier.startsWith('/')")
  })

  it('hook skips file: URL specifiers', () => {
    expect(resolveFromHookCode).toContain("specifier.startsWith('file:')")
  })

  it('hook uses parentURL for fallback (not createRequire)', () => {
    // The hook should use nextResolve with parentURL (ESM conditions),
    // not createRequire().resolve() (CJS conditions).
    // The string "createRequire" may appear in comments; check that
    // it's not imported or called as code.
    expect(resolveFromHookCode).toContain('parentURL: fallbackParentURL')
    expect(resolveFromHookCode).not.toMatch(/import.*createRequire/)
    expect(resolveFromHookCode).not.toMatch(/createRequire\(/)
  })

  it('hook logs to stderr on fallback resolution', () => {
    expect(resolveFromHookCode).toContain('process.stderr.write')
    expect(resolveFromHookCode).toContain('--resolve-from fallback')
  })

  it('data URL construction produces valid base64', () => {
    const encoded = Buffer.from(resolveFromHookCode).toString('base64')
    const decoded = Buffer.from(encoded, 'base64').toString()
    expect(decoded).toBe(resolveFromHookCode)
  })
})
