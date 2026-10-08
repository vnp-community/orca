/**
 * Leak regression (FE-CV-TASK-087-02): both worktree-removal paths (`removeWorktree` and the
 * bulk `purgeWorktreeTerminalState` -> `buildWorktreePurgeState`) must evict
 * `codeIntelQualityByWorktree` for the removed worktree and keep the survivor.
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

const WT1 = 'repo1::/path/wt1'
const WT2 = 'repo1::/path/wt2'

describe('quality state worktree purge', () => {
  beforeEach(() => vi.clearAllMocks())

  it('removeWorktree drops the removed worktree and keeps the other', async () => {
    const store = createTestStore()
    seedStore(store, {
      worktreesByRepo: {
        repo1: [
          makeWorktree({ id: WT1, repoId: 'repo1', path: '/path/wt1' }),
          makeWorktree({ id: WT2, repoId: 'repo1', path: '/path/wt2' })
        ]
      }
    })
    store.getState().setQualityUi(WT1, { annotationsOn: false })
    store.getState().setQualityUi(WT2, { annotationsOn: false })

    const result = await store.getState().removeWorktree(WT1)

    expect(result.ok).toBe(true)
    expect(store.getState().codeIntelQualityByWorktree[WT1]).toBeUndefined()
    expect(store.getState().codeIntelQualityByWorktree[WT2]).toBeDefined()
  })

  it('bulk purge drops the removed worktree and keeps survivors', () => {
    const store = createTestStore()
    store.getState().setQualityUi(WT1, { annotationsOn: false })
    store.getState().setQualityUi(WT2, { annotationsOn: false })

    store.getState().purgeWorktreeTerminalState([WT1])

    expect(store.getState().codeIntelQualityByWorktree[WT1]).toBeUndefined()
    expect(store.getState().codeIntelQualityByWorktree[WT2]).toBeDefined()
  })
})
