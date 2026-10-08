/**
 * Leak regression (FE-CV-TASK-051-01 / 052-03): the single `removeWorktree` path
 * must evict review UI and reading-progress state of the removed worktree only.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('sonner', () => ({
  toast: { info: vi.fn(), success: vi.fn(), error: vi.fn(), warning: vi.fn() }
}))
vi.mock('@/components/terminal-pane/pty-dispatcher', () => ({
  restorePtyDataHandlersAfterFailedShutdown: vi.fn(),
  unregisterPtyDataHandlers: vi.fn()
}))

const mockApi = {
  worktrees: { list: vi.fn().mockResolvedValue([]), remove: vi.fn().mockResolvedValue(undefined) },
  pty: { kill: vi.fn().mockResolvedValue(undefined) }
}
// @ts-expect-error -- minimal window.api stub for the store under test
globalThis.window = { api: mockApi }

import { createTestStore, seedStore, makeWorktree } from './store-test-helpers'
import { DEFAULT_REVIEW_UI_STATE } from './review-ui'

const WT1 = 'repo1::/path/wt1'
const WT2 = 'repo1::/path/wt2'
const entry = { baseCommit: 'a', headCommit: 'b' } as never

describe('review state single-removal purge', () => {
  beforeEach(() => vi.clearAllMocks())

  it('drops the removed worktree and retains the other', async () => {
    const store = createTestStore()
    seedStore(store, {
      worktreesByRepo: {
        repo1: [
          makeWorktree({ id: WT1, repoId: 'repo1', path: '/path/wt1' }),
          makeWorktree({ id: WT2, repoId: 'repo1', path: '/path/wt2' })
        ]
      },
      reviewUiByWorktree: { [WT1]: DEFAULT_REVIEW_UI_STATE, [WT2]: DEFAULT_REVIEW_UI_STATE },
      reviewProgressByWorktree: { [WT1]: entry, [WT2]: entry }
    })

    const result = await store.getState().removeWorktree(WT1)

    expect(result.ok).toBe(true)
    const s = store.getState()
    expect(s.reviewUiByWorktree[WT1]).toBeUndefined()
    expect(s.reviewProgressByWorktree[WT1]).toBeUndefined()
    expect(s.reviewUiByWorktree[WT2]).toBeDefined()
    expect(s.reviewProgressByWorktree[WT2]).toBeDefined()
  })
})
