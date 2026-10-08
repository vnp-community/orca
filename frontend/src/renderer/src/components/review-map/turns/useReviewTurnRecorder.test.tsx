// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import { create } from 'zustand'
import type { ReviewDataApi } from '../review-shell-data'
import { emptyReadingProgress } from '../review-wire-types'
import type { ReviewStateView } from '../review-wire-types'

const holder = vi.hoisted(() => ({ store: null as unknown }))
vi.mock('@/store', () => {
  const useAppStore = ((selector: (s: unknown) => unknown) =>
    (holder.store as (sel: (s: unknown) => unknown) => unknown)(selector)) as unknown as {
    subscribe: (fn: (s: unknown, p: unknown) => void) => () => void
    getState: () => unknown
  }
  useAppStore.subscribe = (fn) => (holder.store as { subscribe: typeof useAppStore.subscribe }).subscribe(fn)
  useAppStore.getState = () => (holder.store as { getState: () => unknown }).getState()
  return { useAppStore }
})

import { TURN_MARKER_SETTLE_MS, useReviewTurnRecorder } from './useReviewTurnRecorder'

const WT = 'repo::wt'
const entry = (state: string, startedAt: number, extra: Record<string, unknown> = {}) => ({
  state,
  prompt: 'SECRET PROMPT TEXT',
  stateStartedAt: startedAt,
  paneKey: 'tab:leaf',
  worktreeId: WT,
  agentType: 'claude',
  stateHistory: [{ state: 'working', startedAt: startedAt - 5000 }],
  ...extra
})

type TestState = {
  codeIntelSupportState: { state: string }
  agentStatusByPaneKey: Record<string, unknown>
  gitStatusByWorktree: Record<string, unknown[]>
  gitBranchCompareSummaryByWorktree: Record<string, unknown>
}

function makeStore(support = 'enabled') {
  const store = create<TestState>()(() => ({
    codeIntelSupportState: { state: support },
    agentStatusByPaneKey: { 'tab:leaf': entry('working', 1000) },
    gitStatusByWorktree: { [WT]: [{ path: 'a.ts', status: 'modified', area: 'unstaged', added: 2, removed: 1 }] },
    gitBranchCompareSummaryByWorktree: { [WT]: { headOid: 'H', baseOid: 'B', mergeBase: 'M' } }
  }))
  holder.store = store
  return store
}

function makeApi() {
  const saves: ReviewStateView[] = []
  let row: ReviewStateView = { baseCommit: '', headCommit: '', readingProgress: emptyReadingProgress(), version: 0 }
  const api = {
    getReviewState: vi.fn(async () => ({ ok: true as const, value: row })),
    saveReviewState: vi.fn(async (_w: string, state: ReviewStateView, expected: number) => {
      saves.push(state)
      row = { ...state, version: expected + 1 }
      return { ok: true as const, value: row }
    })
  } as unknown as ReviewDataApi
  return { api, saves }
}

const finishTurn = (store: ReturnType<typeof makeStore>, doneAt: number): void => {
  act(() => {
    store.setState({ agentStatusByPaneKey: { 'tab:leaf': entry('working', doneAt - 1) } })
    store.setState({ agentStatusByPaneKey: { 'tab:leaf': entry('done', doneAt) } })
  })
}

async function settle(): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(TURN_MARKER_SETTLE_MS + 100)
  })
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useReviewTurnRecorder', () => {
  it('records one marker per finished turn on the worktree-level row, without the prompt', async () => {
    const store = makeStore()
    const { api, saves } = makeApi()
    renderHook(() => useReviewTurnRecorder({ api, getSymbolKeys: () => ['s1', 's2'] }))
    finishTurn(store, 5000)
    await settle()
    expect(saves).toHaveLength(1)
    expect(saves[0].baseCommit).toBe('')
    expect(saves[0].headCommit).toBe('')
    const markers = saves[0].turnMarkers as Record<string, unknown>[]
    expect(markers).toHaveLength(1)
    expect(markers[0]).toMatchObject({
      turnId: 'tab:leaf:5000',
      worktreeId: WT,
      agentType: 'claude',
      startedAt: 0,
      endedAt: 5000,
      headOid: 'H',
      baseOid: 'M',
      overlayAvailable: true,
      symbolKeys: ['s1', 's2']
    })
    expect((markers[0].files as unknown[])).toHaveLength(1)
    expect(JSON.stringify(saves[0])).not.toContain('SECRET PROMPT TEXT')
    expect(JSON.stringify(markers[0])).not.toContain('prompt')
  })

  it('records a second turn separately and keeps overlayAvailable=false without symbol keys', async () => {
    const store = makeStore()
    const { api, saves } = makeApi()
    renderHook(() => useReviewTurnRecorder({ api }))
    finishTurn(store, 5000)
    await settle()
    finishTurn(store, 9000)
    await settle()
    expect(saves).toHaveLength(2)
    const second = saves[1].turnMarkers as { turnId: string; overlayAvailable: boolean }[]
    expect(second.map((m) => m.turnId)).toEqual(['tab:leaf:5000', 'tab:leaf:9000'])
    expect(second.every((m) => !m.overlayAvailable)).toBe(true)
  })

  it('does not record the same turn twice', async () => {
    const store = makeStore()
    const { api, saves } = makeApi()
    renderHook(() => useReviewTurnRecorder({ api }))
    finishTurn(store, 5000)
    // Same stateStartedAt re-announced after a bounce through "working".
    finishTurn(store, 5000)
    await settle()
    expect(saves).toHaveLength(1)
  })

  it('does nothing while code intel support is off', async () => {
    const store = makeStore('disabled')
    const { api } = makeApi()
    renderHook(() => useReviewTurnRecorder({ api }))
    finishTurn(store, 5000)
    await settle()
    expect(api.getReviewState).not.toHaveBeenCalled()
    expect(api.saveReviewState).not.toHaveBeenCalled()
  })

  it('reports a failed save and retries it', async () => {
    const store = makeStore()
    const { api } = makeApi()
    let fail = true
    ;(api.saveReviewState as ReturnType<typeof vi.fn>).mockImplementation(async (_w: string, state: ReviewStateView, expected: number) =>
      fail
        ? { ok: false as const, error: { kind: 'offline', message: '' } }
        : { ok: true as const, value: { ...state, version: expected + 1 } }
    )
    const { result } = renderHook(() => useReviewTurnRecorder({ api }))
    finishTurn(store, 5000)
    await settle()
    expect(result.current.failedWorktreeId).toBe(WT)
    fail = false
    await act(async () => {
      result.current.retry()
      await vi.advanceTimersByTimeAsync(10)
    })
    expect(result.current.failedWorktreeId).toBeNull()
  })

  it('clears pending timers on unmount', async () => {
    const store = makeStore()
    const { api } = makeApi()
    const { unmount } = renderHook(() => useReviewTurnRecorder({ api }))
    finishTurn(store, 5000)
    unmount()
    await settle()
    expect(api.saveReviewState).not.toHaveBeenCalled()
  })
})
