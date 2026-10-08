// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { create } from 'zustand'

const h = vi.hoisted(() => ({ store: null as unknown, compare: vi.fn() }))
vi.mock('@/store', () => {
  const useAppStore = ((sel: (s: unknown) => unknown) =>
    (h.store as (s: (x: unknown) => unknown) => unknown)(sel)) as unknown as {
    getState: () => unknown
  }
  useAppStore.getState = () => (h.store as { getState: () => unknown }).getState()
  return { useAppStore }
})
vi.mock('@/runtime/runtime-git-client', () => ({ getRuntimeGitBranchCompare: h.compare }))
vi.mock('@/lib/connection-context', () => ({ getConnectionId: () => 'ssh-1' }))
vi.mock('@/lib/repo-runtime-owner', () => ({
  getRepoOwnerRoutedSettings: (settings: object, repo: { id: string } | null) => ({
    ...settings,
    routedFor: repo?.id
  })
}))

import {
  reviewCompareBaseRef,
  useReviewBranchCompareFetch
} from './use-review-branch-compare-fetch'

const WT = 'repo1::/p/wt'
type State = {
  gitBranchCompareSummaryByWorktree: Record<string, unknown>
  requestKey: string | null
  begin: ReturnType<typeof vi.fn>
}

function seed(worktree: Record<string, unknown>, repo: Record<string, unknown> = {}) {
  const begin = vi.fn()
  h.store = create<State & Record<string, unknown>>((set) => ({
    settings: { activeRuntimeEnvironmentId: null },
    repos: [{ id: 'repo1', ...repo }],
    worktreesByRepo: { repo1: [{ id: WT, repoId: 'repo1', path: '/p/wt', ...worktree }] },
    gitBranchCompareSummaryByWorktree: {},
    requestKey: null,
    begin,
    beginGitBranchCompareRequest: (id: string, key: string, base: string, opts: unknown) => {
      begin(id, key, base, opts)
      set({ requestKey: key })
    },
    setGitBranchCompareResult: (id: string, key: string, result: { summary: unknown }) =>
      set((s) =>
        s.requestKey === key
          ? {
              gitBranchCompareSummaryByWorktree: {
                ...s.gitBranchCompareSummaryByWorktree,
                [id]: result.summary
              }
            }
          : s
      )
  }))
  return h.store as {
    getState: () => State & Record<string, unknown>
    setState: (p: Partial<State>) => void
  }
}

beforeEach(() => {
  h.compare.mockReset()
})
afterEach(cleanup)

describe('reviewCompareBaseRef', () => {
  it('prefers the worktree base, then the repo base', () => {
    expect(
      reviewCompareBaseRef({ baseRef: ' origin/dev ' }, { worktreeBaseRef: 'origin/main' })
    ).toBe('origin/dev')
    expect(reviewCompareBaseRef({ baseRef: '' }, { worktreeBaseRef: 'origin/main' })).toBe(
      'origin/main'
    )
    expect(reviewCompareBaseRef(null, null)).toBeNull()
  })
})

describe('useReviewBranchCompareFetch', () => {
  it('fetches git.branchCompare through the runtime client when no summary exists and stores it', async () => {
    const store = seed({ baseRef: 'origin/main' })
    h.compare.mockResolvedValue({
      summary: { status: 'ready', baseRef: 'origin/main', headOid: 'H' },
      entries: []
    })
    renderHook(() => useReviewBranchCompareFetch(WT, true))
    await waitFor(() =>
      expect(store.getState().gitBranchCompareSummaryByWorktree[WT]).toMatchObject({ headOid: 'H' })
    )
    expect(h.compare).toHaveBeenCalledWith(
      {
        settings: { activeRuntimeEnvironmentId: null, routedFor: 'repo1' },
        worktreeId: WT,
        worktreePath: '/p/wt',
        connectionId: 'ssh-1'
      },
      'origin/main'
    )
    // No loading placeholder: the pinned base keeps the default scope usable meanwhile.
    expect(store.getState().begin).toHaveBeenCalledWith(WT, expect.any(String), 'origin/main', {
      preserveExistingSummary: true
    })
    expect(h.compare).toHaveBeenCalledTimes(1)
  })

  it('does nothing when disabled, when a summary exists, or without any base', () => {
    seed({ baseRef: 'origin/main' })
    renderHook(() => useReviewBranchCompareFetch(WT, false))
    const store = seed({ baseRef: 'origin/main' })
    store.setState({ gitBranchCompareSummaryByWorktree: { [WT]: { status: 'ready' } } })
    renderHook(() => useReviewBranchCompareFetch(WT, true))
    seed({ baseRef: null })
    renderHook(() => useReviewBranchCompareFetch(WT, true))
    expect(h.compare).not.toHaveBeenCalled()
  })

  it('leaves no error summary behind when the compare fails', async () => {
    const store = seed({}, { worktreeBaseRef: 'origin/main' })
    h.compare.mockRejectedValue(new Error('boom'))
    renderHook(() => useReviewBranchCompareFetch(WT, true))
    await waitFor(() => expect(h.compare).toHaveBeenCalledWith(expect.anything(), 'origin/main'))
    await Promise.resolve()
    expect(store.getState().gitBranchCompareSummaryByWorktree[WT]).toBeUndefined()
  })
})
