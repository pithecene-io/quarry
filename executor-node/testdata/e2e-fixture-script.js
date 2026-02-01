/**
 * E2E fixture script for cross-language IPC testing.
 *
 * This script emits a deterministic sequence of events that the Go runtime
 * can decode and validate. It covers all major event types and artifact
 * chunking per CONTRACT_IPC.md.
 *
 * Event sequence:
 * 1. item (type: "fixture_product")
 * 2. log (level: "info")
 * 3. artifact chunks + artifact event (small binary blob)
 * 4. item (type: "fixture_summary")
 * 5. run_complete with summary
 *
 * The script does NOT use Puppeteer navigation to avoid network dependencies.
 * All emits are deterministic and synchronous (no randomness).
 *
 * @module
 */

/**
 * Generate deterministic fake artifact data.
 * Uses a simple pattern that's easy to verify.
 */
function generateFixtureArtifactData() {
  // Create a small fake binary blob (256 bytes fits in a single chunk)
  const size = 256
  const data = Buffer.alloc(size)
  for (let i = 0; i < size; i++) {
    data[i] = i % 256
  }
  return data
}

export default async function fixtureScript(ctx) {
  // 1. Emit first item
  await ctx.emit.item({
    item_type: 'fixture_product',
    data: {
      id: 'prod-001',
      name: 'Test Product',
      price: 99.99,
      tags: ['electronics', 'sale'],
      in_stock: true
    }
  })

  // 2. Emit log
  await ctx.emit.info('Fixture script processing', {
    step: 'items',
    count: 1
  })

  // 3. Emit artifact (this generates chunk frames + artifact event)
  const artifactData = generateFixtureArtifactData()
  const artifactId = await ctx.emit.artifact({
    name: 'fixture-screenshot.bin',
    content_type: 'application/octet-stream',
    data: artifactData
  })

  // 4. Emit second item referencing the artifact
  await ctx.emit.item({
    item_type: 'fixture_summary',
    data: {
      artifact_id: artifactId,
      artifact_size: artifactData.length,
      processed_at: '2024-01-15T10:00:00.000Z' // Fixed timestamp for determinism
    }
  })

  // 5. Emit run_complete with summary
  await ctx.emit.runComplete({
    summary: {
      items_emitted: 2,
      artifacts_emitted: 1,
      fixture_version: '1.0.0'
    }
  })
}
