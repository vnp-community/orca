/**
 * Leak regression (FE-CV-TASK-050-10): the bulk purge path
 * (`purgeWorktreeTerminalState` -> `buildWorktreePurgeState`) must evict the
 * code-intel result cache of removed worktrees and keep survivors.
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

import { createTestStore } from './store-test-helpers'

const WT1 = 'repo1::/path/wt1'
const WT2 = 'repo1::/path/wt2'

describe('code-intel cache bulk purge', () => {
  beforeEach(() => vi.clearAllMocks())

  it('drops removed worktree cache and retains survivors', () => {
    const store = createTestStore()
    store.getState().setCacheResult(WT1, 'k', 1)
    store.getState().setCacheResult(WT2, 'k', 2)

    store.getState().purgeWorktreeTerminalState([WT1])

    const s = store.getState()
    expect(s.codeIntelWorktreeState[WT1]).toBeUndefined()
    expect(s.getCacheResult(WT2, 'k')).toBe(2)
  })
})
