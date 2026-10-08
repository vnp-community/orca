/**
 * Leak regression (FE-CV-TASK-051-01 / 052-03): the bulk purge path must evict
 * review UI and reading-progress state of removed worktrees and keep survivors.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('sonner', () => ({
  toast: { info: vi.fn(), success: vi.fn(), error: vi.fn(), warning: vi.fn() }
}))
vi.mock('@/components/terminal-pane/pty-dispatcher', () => ({
  restorePtyDataHandlersAfterFailedShutdown: vi.fn(),
  unregisterPtyDataHandlers: vi.fn()
}))

// @ts-expect-error -- minimal window.api stub for the store under test
globalThis.window = { api: { worktrees: { list: vi.fn().mockResolvedValue([]) } } }

import { createTestStore, seedStore } from './store-test-helpers'
import { DEFAULT_REVIEW_UI_STATE } from './review-ui'

const WT1 = 'repo1::/path/wt1'
const WT2 = 'repo1::/path/wt2'
const entry = { baseCommit: 'a', headCommit: 'b' } as never

describe('review state bulk purge', () => {
  beforeEach(() => vi.clearAllMocks())

  it('drops removed worktree review state and retains survivors', () => {
    const store = createTestStore()
    seedStore(store, {
      reviewUiByWorktree: { [WT1]: DEFAULT_REVIEW_UI_STATE, [WT2]: DEFAULT_REVIEW_UI_STATE },
      reviewProgressByWorktree: { [WT1]: entry, [WT2]: entry }
    })

    store.getState().purgeWorktreeTerminalState([WT1])

    const s = store.getState()
    expect(s.reviewUiByWorktree[WT1]).toBeUndefined()
    expect(s.reviewProgressByWorktree[WT1]).toBeUndefined()
    expect(s.reviewUiByWorktree[WT2]).toBeDefined()
    expect(s.reviewProgressByWorktree[WT2]).toBeDefined()
  })
})
