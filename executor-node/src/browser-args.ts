/**
 * Build Chromium launch arguments.
 * Always includes --disable-dev-shm-usage; conditionally adds sandbox flags.
 */
export function chromiumArgs(extra: string[] = []): string[] {
  const args = ['--disable-dev-shm-usage', ...extra]
  if (process.env.QUARRY_NO_SANDBOX === '1') {
    args.push('--no-sandbox', '--disable-setuid-sandbox')
  }
  return args
}
