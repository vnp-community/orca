// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const flags = { state: 'enabled', codeIntel: true, quality: true, ai: false }
const tracker = vi.hoisted(() => ({ registerCompletion: vi.fn(), decide: vi.fn() }))
const store = vi.hoisted(() => {
  const listeners = new Set<(s: unknown, p: unknown) => void>()
  return { listeners, subscribe: vi.fn((l: (s: unknown, p: unknown) => void) => (listeners.add(l), () => listeners.delete(l))) }
})

vi.mock('@/store', () => ({ useAppStore: { subscribe: store.subscribe } }))
vi.mock('./useQualityFeatureFlags', () => ({ useQualityFeatureFlags: () => flags }))
vi.mock('../lib/review-decision-tracker', () => tracker)

import { useReviewDecisionTelemetry } from './useReviewDecisionTelemetry'

const entry = (state: string, at: number) => ({
  state, prompt: '', updatedAt: at, stateStartedAt: at, paneKey: 'p', stateHistory: [], worktreeId: 'wt'
})
const emit = (prev: unknown, next: unknown) => store.listeners.forEach((l) => l({ agentStatusByPaneKey: next }, { agentStatusByPaneKey: prev }))

beforeEach(() => {
  tracker.registerCompletion.mockClear()
  tracker.decide.mockClear()
  store.listeners.clear()
  store.subscribe.mockClear()
  flags.codeIntel = true
  flags.quality = true
})

describe('useReviewDecisionTelemetry', () => {
  it('does not subscribe or record when both flags are off', () => {
    flags.codeIntel = false
    flags.quality = false
    const { result } = renderHook(() => useReviewDecisionTelemetry({ gate: 'none', openFindings: 0 }))
    expect(store.subscribe).not.toHaveBeenCalled()
    result.current.recordDecision('wt', 'commit')
    expect(tracker.decide).not.toHaveBeenCalled()
  })

  it('registers a finished agent turn with its done timestamp', () => {
    renderHook(() => useReviewDecisionTelemetry({ gate: 'none', openFindings: 0 }))
    emit({ p: entry('working', 1) }, { p: entry('done', 9) })
    expect(tracker.registerCompletion).toHaveBeenCalledWith('wt', 9)
  })

  it('records a decision with the latest gate context and swallows tracker errors', () => {
    const { result, rerender } = renderHook((c) => useReviewDecisionTelemetry(c), {
      initialProps: { gate: 'none', openFindings: 0 } as { gate: 'none' | 'fail'; openFindings: number }
    })
    rerender({ gate: 'fail', openFindings: 4 })
    result.current.recordDecision('wt', 'commit')
    expect(tracker.decide).toHaveBeenCalledWith('wt', 'commit', { gate: 'fail', openFindings: 4 })
    tracker.decide.mockImplementation(() => {
      throw new Error('boom')
    })
    expect(() => result.current.recordDecision('wt', 'create_review')).not.toThrow()
  })

  it('works with only the code-intel flag on (quality off)', () => {
    flags.quality = false
    renderHook(() => useReviewDecisionTelemetry({ gate: 'none', openFindings: 0 }))
    expect(store.subscribe).toHaveBeenCalledTimes(1)
  })

  it('unsubscribes on unmount', () => {
    const { unmount } = renderHook(() => useReviewDecisionTelemetry({ gate: 'none', openFindings: 0 }))
    unmount()
    expect(store.listeners.size).toBe(0)
  })
})
