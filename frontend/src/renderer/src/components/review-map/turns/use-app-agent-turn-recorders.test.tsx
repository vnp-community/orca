// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import { create } from 'zustand'
import { emptyReadingProgress } from '../review-wire-types'
import type { ReviewStateView } from '../review-wire-types'

const h = vi.hoisted(() => ({
  store: null as unknown,
  saves: [] as ReviewStateView[],
  call: vi.fn()
}))
vi.mock('@/store', () => {
  const useAppStore = ((selector: (s: unknown) => unknown) =>
    (h.store as (sel: (s: unknown) => unknown) => unknown)(selector)) as unknown as {
    subscribe: (fn: (s: unknown, p: unknown) => void) => () => void
    getState: () => unknown
  }
  useAppStore.subscribe = (fn) => (h.store as { subscribe: typeof useAppStore.subscribe }).subscribe(fn)
  useAppStore.getState = () => (h.store as { getState: () => unknown }).getState()
  return { useAppStore }
})
vi.mock('@/lib/worktree-runtime-owner', () => ({ getRuntimeEnvironmentIdForWorktree: () => null }))
vi.mock('../../../hooks/useQualityFeatureFlags', () => ({
  useQualityFeatureFlags: () => ({ state: 'enabled', codeIntel: true, quality: true, ai: false })
}))
vi.mock('../../../runtime/code-intel-client', () => ({
  getCodeIntelClient: () => ({ call: h.call }),
  classifyCodeIntelError: () => ({ kind: 'unknown' })
}))
vi.mock('../review-data-api-default', () => {
  let row: ReviewStateView = { baseCommit: '', headCommit: '', readingProgress: emptyReadingProgress(), version: 0 }
  return {
    defaultReviewDataApi: {
      getReviewState: async () => ({ ok: true, value: row }),
      saveReviewState: async (_w: string, state: ReviewStateView, expected: number) => {
        h.saves.push(state)
        row = { ...state, version: expected + 1 }
        return { ok: true, value: row }
      }
    }
  }
})

import { TURN_MARKER_SETTLE_MS } from './useReviewTurnRecorder'
import { registerTurnSymbolKeys, subscribeTurnSaved } from './review-turn-recorder-bus'
import { useAppAgentTurnRecorders } from './use-app-agent-turn-recorders'

const WT = 'repo::wt'
const entry = (state: string, at: number) => ({
  state,
  prompt: 'SECRET PROMPT',
  stateStartedAt: at,
  paneKey: 'tab:leaf',
  worktreeId: WT,
  agentType: 'claude',
  stateHistory: [{ state: 'working', startedAt: at - 1000 }]
})

beforeEach(() => {
  vi.useFakeTimers()
  h.saves.length = 0
  h.call.mockReset()
  h.call.mockResolvedValue({ ok: true, result: {} })
  h.store = create(() => ({
    codeIntelSupportState: { state: 'enabled' },
    agentStatusByPaneKey: {} as Record<string, unknown>,
    gitStatusByWorktree: { [WT]: [{ path: 'a.ts', status: 'modified', area: 'unstaged', added: 2, removed: 1 }] },
    gitBranchCompareSummaryByWorktree: { [WT]: { headOid: 'H', baseOid: 'B', mergeBase: 'M' } },
    worktreesByRepo: { r: [{ id: WT, projectId: 'proj' }] }
  }))
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useAppAgentTurnRecorders', () => {
  it('records the local marker and the backend turn exactly once with no Review tab mounted', async () => {
    const saved = vi.fn()
    const off = subscribeTurnSaved(saved)
    const offKeys = registerTurnSymbolKeys(WT, () => ['sym1'])
    renderHook(() => useAppAgentTurnRecorders())
    const store = h.store as ReturnType<typeof create<Record<string, unknown>>>
    act(() => {
      store.setState({ agentStatusByPaneKey: { 'tab:leaf': entry('working', 4000) } })
      store.setState({ agentStatusByPaneKey: { 'tab:leaf': entry('done', 5000) } })
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(TURN_MARKER_SETTLE_MS + 100)
    })
    expect(h.saves).toHaveLength(1)
    const marker = (h.saves[0].turnMarkers as { files: { p: string; h: string }[]; symbolKeys: string[] }[])[0]
    expect(marker.symbolKeys).toEqual(['sym1'])
    expect(saved).toHaveBeenCalledWith(WT, expect.any(Array))
    expect(h.call).toHaveBeenCalledTimes(1)
    const params = h.call.mock.calls[0][2]
    // Backend file identity is derived from the marker's per-file hashes.
    expect(params.filesChangedCount).toBe(marker.files.length)
    expect(JSON.stringify(params)).not.toContain('SECRET PROMPT')
    off()
    offKeys()
  })
})
