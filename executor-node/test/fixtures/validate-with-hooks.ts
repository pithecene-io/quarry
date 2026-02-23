/**
 * Test fixture: valid script with hooks.
 */
export function prepare() {
  return { action: 'continue' as const }
}

export async function cleanup(): Promise<void> {
  // no-op
}

export default async function run(): Promise<void> {
  // no-op
}
