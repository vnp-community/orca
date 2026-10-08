// @vitest-environment happy-dom
import { act, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

const storeState = {
  activeWorktreeId: 'wt',
  codeIntelSupportState: { state: 'enabled' as string },
  gitBranchCompareSummaryByWorktree: {
    wt: {
      baseRef: 'main',
      baseOid: 'a',
      compareRef: 'f',
      headOid: 'b',
      mergeBase: 'a',
      changedFiles: 2,
      status: 'ready'
    }
  } as Record<string, unknown>,
  codeIntelResyncCounter: 0,
  agentStatusByPaneKey: {},
  retainedAgentsByPaneKey: {},
  tabsByWorktree: {}
}
vi.mock('@/store', () => ({
  useAppStore: Object.assign((sel: (s: typeof storeState) => unknown) => sel(storeState), {
    getState: () => storeState
  })
}))
vi.mock('./open-review-entry', () => ({ openReviewFromEntryPoint: vi.fn() }))

import { ReviewSummaryPanel } from './ReviewSummaryPanel'
import type { ReviewSummaryFetchers } from './useCodeIntelReviewSummary'

const overlay = {
  scope: { baseRef: 'main', mode: 'worktree', includesUncommitted: true },
  changedFiles: [],
  changedSymbols: [],
  affectedFlows: [],
  touchedTables: [],
  touchedContracts: [],
  uncoveredSymbols: [],
  violations: [],
  readingOrder: [],
  components: [],
  risk: null,
  indexFreshness: { state: 'fresh' },
  limits: { truncated: {}, totalCounts: { files: 7 } }
}

describe('ReviewSummaryPanel', () => {
  it('fetches only while visible and renders counts', async () => {
    const fetchOverlaySummary = vi.fn(async () => ({ ok: true as const, value: overlay as never }))
    const fetchers: ReviewSummaryFetchers = {
      fetchOverlaySummary,
      fetchProgress: async () => null
    }
    const { rerender } = render(<ReviewSummaryPanel isVisible={false} fetchers={fetchers} />)
    await act(async () => {})
    expect(fetchOverlaySummary).not.toHaveBeenCalled()

    rerender(<ReviewSummaryPanel isVisible fetchers={fetchers} />)
    await waitFor(() => expect(screen.getByText(/7/)).toBeTruthy())
    expect(fetchOverlaySummary).toHaveBeenCalledTimes(1)
  })

  it('shows a disabled message and never fetches when the flag is off', async () => {
    storeState.codeIntelSupportState = { state: 'disabled' }
    const fetchOverlaySummary = vi.fn()
    render(
      <ReviewSummaryPanel
        isVisible
        fetchers={{ fetchOverlaySummary, fetchProgress: async () => null }}
      />
    )
    await act(async () => {})
    expect(fetchOverlaySummary).not.toHaveBeenCalled()
    expect(screen.getByText(/isn't enabled/)).toBeTruthy()
  })
})
