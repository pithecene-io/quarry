/**
 * ESM resolve hook for --resolve-from support.
 *
 * Registers a module.register() hook that retries bare-specifier resolution
 * with a fallback parentURL pointing at the --resolve-from directory.
 * This preserves ESM import conditions (not CJS require conditions).
 *
 * @module
 */

/**
 * The ESM loader hook source code, registered via data URL.
 *
 * Exported so integration tests can use the same hook code as production,
 * preventing copy drift.
 */
export const resolveFromHookCode = `
  import { pathToFileURL } from 'node:url';

  let fallbackParentURL;

  export function initialize(data) {
    // Construct a file:// URL pointing into the resolve-from directory.
    // nextResolve uses parentURL as the resolution base, so ESM
    // "imports" conditions are applied (not CJS "require" conditions).
    fallbackParentURL = pathToFileURL(data.resolveFrom + '/noop.js').href;
  }

  export async function resolve(specifier, context, nextResolve) {
    // Skip relative and absolute specifiers — only intercept bare specifiers
    if (specifier.startsWith('.') || specifier.startsWith('/') || specifier.startsWith('file:')) {
      return nextResolve(specifier, context);
    }

    // Try default resolution first
    try {
      return await nextResolve(specifier, context);
    } catch (defaultErr) {
      // Retry with parentURL pointing at --resolve-from directory.
      // This preserves ESM import conditions (package.json "exports"
      // with "import" key) unlike CJS require-based resolution.
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
        // Re-throw the original error if fallback also fails
        throw defaultErr;
      }
    }
  }
`

/**
 * Register the ESM resolve-from hook via module.register().
 *
 * The hook tries default resolution first, then falls back to resolving
 * bare specifiers from the given directory using nextResolve with a
 * substituted parentURL.
 */
export async function registerResolveFromHook(resolveFrom: string): Promise<void> {
  const { register } = await import('node:module')
  const hookUrl = `data:text/javascript;base64,${Buffer.from(resolveFromHookCode).toString('base64')}`
  register(hookUrl, { data: { resolveFrom } })
}
