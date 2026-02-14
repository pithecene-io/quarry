/** State tracked between idle-monitor poll ticks. */
export type IdlePollState = {
  idleStartedAt: number | null
  consecutiveFailures: number
}

/** What the caller should do after evaluating a poll tick. */
export type IdlePollAction =
  | { type: 'continue'; nextState: IdlePollState }
  | { type: 'shutdown' }
  | { type: 'crash-exit'; failures: number }

/**
 * Pure evaluation of a single idle-monitor poll tick.
 *
 * Caller supplies the fetch result and current state;
 * function returns the action and updated state.
 */
export function evaluateIdlePoll(
  fetchResult: { ok: true; activePages: number } | { ok: false },
  state: IdlePollState,
  config: { idleTimeoutMs: number; maxConsecutiveFailures: number },
  now: number = Date.now()
): IdlePollAction {
  if (!fetchResult.ok) {
    const failures = state.consecutiveFailures + 1
    if (failures >= config.maxConsecutiveFailures) {
      return { type: 'crash-exit', failures }
    }
    return {
      type: 'continue',
      nextState: { ...state, consecutiveFailures: failures }
    }
  }

  // Success — reset failure counter
  if (fetchResult.activePages > 0) {
    return {
      type: 'continue',
      nextState: { idleStartedAt: null, consecutiveFailures: 0 }
    }
  }

  // No active pages — start or continue idle countdown
  const idleStartedAt = state.idleStartedAt ?? now
  if (now - idleStartedAt >= config.idleTimeoutMs) {
    return { type: 'shutdown' }
  }

  return {
    type: 'continue',
    nextState: { idleStartedAt, consecutiveFailures: 0 }
  }
}
