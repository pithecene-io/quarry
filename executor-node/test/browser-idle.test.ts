import { describe, expect, it } from 'vitest'
import { evaluateIdlePoll, type IdlePollState } from '../src/browser-idle.js'

const defaultConfig = { idleTimeoutMs: 60_000, maxConsecutiveFailures: 3 }

function freshState(): IdlePollState {
  return { idleStartedAt: null, consecutiveFailures: 0 }
}

describe('evaluateIdlePoll', () => {
  describe('transient failure retry', () => {
    it('survives 1 failure — continues with incremented counter', () => {
      const action = evaluateIdlePoll({ ok: false }, freshState(), defaultConfig)

      expect(action.type).toBe('continue')
      if (action.type === 'continue') {
        expect(action.nextState.consecutiveFailures).toBe(1)
      }
    })

    it('survives 2 consecutive failures — continues', () => {
      const state: IdlePollState = { idleStartedAt: null, consecutiveFailures: 1 }
      const action = evaluateIdlePoll({ ok: false }, state, defaultConfig)

      expect(action.type).toBe('continue')
      if (action.type === 'continue') {
        expect(action.nextState.consecutiveFailures).toBe(2)
      }
    })

    it('exits on 3 consecutive failures', () => {
      const state: IdlePollState = { idleStartedAt: null, consecutiveFailures: 2 }
      const action = evaluateIdlePoll({ ok: false }, state, defaultConfig)

      expect(action.type).toBe('crash-exit')
      if (action.type === 'crash-exit') {
        expect(action.failures).toBe(3)
      }
    })

    it('resets failure counter on success after failures', () => {
      const state: IdlePollState = { idleStartedAt: null, consecutiveFailures: 2 }
      const action = evaluateIdlePoll({ ok: true, activePages: 1 }, state, defaultConfig)

      expect(action.type).toBe('continue')
      if (action.type === 'continue') {
        expect(action.nextState.consecutiveFailures).toBe(0)
      }
    })

    it('2 failures then success then 2 failures survives (no accumulation)', () => {
      let state = freshState()

      // Fail twice
      let action = evaluateIdlePoll({ ok: false }, state, defaultConfig)
      expect(action.type).toBe('continue')
      state = (action as { type: 'continue'; nextState: IdlePollState }).nextState

      action = evaluateIdlePoll({ ok: false }, state, defaultConfig)
      expect(action.type).toBe('continue')
      state = (action as { type: 'continue'; nextState: IdlePollState }).nextState
      expect(state.consecutiveFailures).toBe(2)

      // Succeed — resets counter
      action = evaluateIdlePoll({ ok: true, activePages: 1 }, state, defaultConfig)
      expect(action.type).toBe('continue')
      state = (action as { type: 'continue'; nextState: IdlePollState }).nextState
      expect(state.consecutiveFailures).toBe(0)

      // Fail twice more — still alive
      action = evaluateIdlePoll({ ok: false }, state, defaultConfig)
      state = (action as { type: 'continue'; nextState: IdlePollState }).nextState
      action = evaluateIdlePoll({ ok: false }, state, defaultConfig)
      expect(action.type).toBe('continue')
    })
  })

  describe('non-OK HTTP counts as failure', () => {
    it('non-OK response increments failure count', () => {
      // The caller wraps fetch errors (including non-OK throws) as { ok: false }
      const action = evaluateIdlePoll({ ok: false }, freshState(), defaultConfig)

      expect(action.type).toBe('continue')
      if (action.type === 'continue') {
        expect(action.nextState.consecutiveFailures).toBe(1)
      }
    })

    it('3 consecutive non-OK responses trigger crash-exit', () => {
      let state = freshState()
      for (let i = 0; i < 2; i++) {
        const action = evaluateIdlePoll({ ok: false }, state, defaultConfig)
        state = (action as { type: 'continue'; nextState: IdlePollState }).nextState
      }
      const action = evaluateIdlePoll({ ok: false }, state, defaultConfig)
      expect(action.type).toBe('crash-exit')
    })
  })

  describe('idle timeout behavior', () => {
    it('starts idle timer when no active pages', () => {
      const now = 1_000_000
      const action = evaluateIdlePoll(
        { ok: true, activePages: 0 },
        freshState(),
        defaultConfig,
        now
      )

      expect(action.type).toBe('continue')
      if (action.type === 'continue') {
        expect(action.nextState.idleStartedAt).toBe(now)
      }
    })

    it('resets idle timer when active pages found', () => {
      const state: IdlePollState = { idleStartedAt: 1_000_000, consecutiveFailures: 0 }
      const action = evaluateIdlePoll({ ok: true, activePages: 1 }, state, defaultConfig)

      expect(action.type).toBe('continue')
      if (action.type === 'continue') {
        expect(action.nextState.idleStartedAt).toBeNull()
      }
    })

    it('continues when idle but timeout not reached', () => {
      const state: IdlePollState = { idleStartedAt: 1_000_000, consecutiveFailures: 0 }
      const action = evaluateIdlePoll(
        { ok: true, activePages: 0 },
        state,
        defaultConfig,
        1_030_000 // 30s elapsed, timeout is 60s
      )

      expect(action.type).toBe('continue')
    })

    it('shuts down when idle timeout exceeded', () => {
      const state: IdlePollState = { idleStartedAt: 1_000_000, consecutiveFailures: 0 }
      const action = evaluateIdlePoll(
        { ok: true, activePages: 0 },
        state,
        defaultConfig,
        1_060_000 // exactly 60s elapsed
      )

      expect(action.type).toBe('shutdown')
    })

    it('shuts down when idle timeout exceeded by margin', () => {
      const state: IdlePollState = { idleStartedAt: 1_000_000, consecutiveFailures: 0 }
      const action = evaluateIdlePoll(
        { ok: true, activePages: 0 },
        state,
        defaultConfig,
        1_065_000 // 65s elapsed
      )

      expect(action.type).toBe('shutdown')
    })
  })
})
