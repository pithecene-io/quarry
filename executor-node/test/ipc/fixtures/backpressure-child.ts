/**
 * Child process fixture for backpressure integration test.
 *
 * Installs the stdout guard, creates a StdioSink with the split contract
 * (ipcOutput for events, ipcWrite for data), and writes enough IPC frames
 * to exceed the OS pipe buffer (~64KB on Linux). This forces write() to
 * return false and the sink to wait for drain events on ipcOutput.
 *
 * If the drain event is not delivered (the bug this test pins), the child
 * will hang indefinitely and the parent test's timeout will catch it.
 */
import type { EventEnvelope, EventId, RunId } from '@pithecene-io/quarry-sdk'
import { drainStdout, StdioSink } from '../../../src/ipc/sink.js'
import { installStdoutGuard } from '../../../src/ipc/stdout-guard.js'

const { ipcOutput, ipcWrite } = installStdoutGuard()
const sink = new StdioSink(ipcOutput, ipcWrite)

// Write 30 frames × ~4KB each ≈ 120KB — exceeds 64KB pipe buffer,
// guaranteeing at least one backpressure cycle (write returns false,
// must wait for drain).
const FRAME_COUNT = 30

for (let i = 1; i <= FRAME_COUNT; i++) {
  await sink.writeEvent({
    contract_version: '0.7.2',
    event_id: `evt-${i}` as EventId,
    run_id: 'run-bp-test' as RunId,
    seq: i,
    type: 'item',
    ts: '2024-01-01T00:00:00.000Z',
    payload: { item_type: 'test', data: { padding: 'x'.repeat(4000) } },
    attempt: 1
  } as EventEnvelope<'item'>)
}

// Inject a stray stdout write mid-stream to exercise the guard under load
process.stdout.write('stray write during backpressure\n')

// Terminal event
await sink.writeEvent({
  contract_version: '0.7.2',
  event_id: `evt-${FRAME_COUNT + 1}` as EventId,
  run_id: 'run-bp-test' as RunId,
  seq: FRAME_COUNT + 1,
  type: 'run_complete',
  ts: '2024-01-01T00:00:00.000Z',
  payload: { summary: { items: FRAME_COUNT } },
  attempt: 1
} as EventEnvelope<'run_complete'>)

await drainStdout()
process.exit(0)
