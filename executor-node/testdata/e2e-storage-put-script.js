/**
 * E2E fixture script for file_write_ack round-trip testing.
 *
 * Exercises ctx.storage.put() which generates file_write IPC frames.
 * The Go runtime must send file_write_ack frames back on stdin for
 * the storage.put() promise to resolve.
 *
 * Sequence:
 * 1. storage.put("report.json") — succeeds, verifies ack resolves promise
 * 2. item event (references storage key)
 * 3. run_complete
 *
 * Does NOT use Puppeteer navigation to avoid network dependencies.
 *
 * @module
 */

export default async function storagePutScript(ctx) {
  // 1. Write a file via storage.put() — blocks until ack received
  const result = await ctx.storage.put({
    filename: 'report.json',
    content_type: 'application/json',
    data: Buffer.from(JSON.stringify({ items: 42, status: 'ok' })),
  })

  // 2. Emit item referencing the storage key
  await ctx.emit.item({
    item_type: 'storage_test',
    data: {
      storage_key: result.key,
      filename: 'report.json',
    },
  })

  // 3. Complete
  await ctx.emit.runComplete({
    summary: { files_written: 1 },
  })
}
