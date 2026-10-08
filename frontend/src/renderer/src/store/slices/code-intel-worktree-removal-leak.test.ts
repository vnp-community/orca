/**
 * Leak regression (FE-CV-TASK-050-10): the single `removeWorktree` path must
 * evict the removed worktree's code-intel cache and keep other worktrees.
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

describe('code-intel cache single-removal purge', () => {
  beforeEach(() => vi.clearAllMocks())

  it('drops removed worktree cache and retains the other worktree', async () => {
    const store = createTestStore()
    seedStore(store, {
      worktreesByRepo: {
        repo1: [
          makeWorktree({ id: WT1, repoId: 'repo1', path: '/path/wt1' }),
          makeWorktree({ id: WT2, repoId: 'repo1', path: '/path/wt2' })
        ]
      }
    })
    store.getState().setCacheResult(WT1, 'k', 1)
    store.getState().setCacheResult(WT2, 'k', 2)

    const result = await store.getState().removeWorktree(WT1)

    expect(result.ok).toBe(true)
    const s = store.getState()
    expect(s.codeIntelWorktreeState[WT1]).toBeUndefined()
    expect(s.getCacheResult(WT2, 'k')).toBe(2)
  })
})
