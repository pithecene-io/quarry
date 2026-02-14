/**
 * Child process fixture for resolve-from integration test.
 *
 * Exercises the ESM resolve hook end-to-end:
 * 1. Reads QUARRY_RESOLVE_FROM from env (set by parent test)
 * 2. Registers the same hook code used in bin/executor.ts
 * 3. Dynamically imports a bare specifier ("@test/greet") that only
 *    exists under the QUARRY_RESOLVE_FROM directory
 * 4. Writes the resolved value to stdout
 *
 * The parent test creates the fake package, spawns this fixture, and
 * asserts that the import succeeds and the fallback message appears
 * on stderr.
 *
 * Exit codes:
 *   0 = import succeeded via fallback
 *   1 = import failed (hook didn't work)
 *   2 = QUARRY_RESOLVE_FROM not set
 */
import { register } from 'node:module'

const resolveFrom = process.env.QUARRY_RESOLVE_FROM
if (!resolveFrom) {
  process.stderr.write('QUARRY_RESOLVE_FROM not set\n')
  process.exit(2)
}

// Register the same hook used in bin/executor.ts
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

const hookUrl = `data:text/javascript;base64,${Buffer.from(hookCode).toString('base64')}`
register(hookUrl, { data: { resolveFrom } })

// Now try to import a bare specifier that only exists in QUARRY_RESOLVE_FROM
try {
  const mod = await import('@test/greet')
  // Write the resolved value so the parent can verify
  process.stdout.write(JSON.stringify({ greeting: mod.greet() }) + '\n')
  process.exit(0)
} catch (err: unknown) {
  const message = err instanceof Error ? err.message : String(err)
  process.stderr.write(`Import failed: ${message}\n`)
  process.exit(1)
}
