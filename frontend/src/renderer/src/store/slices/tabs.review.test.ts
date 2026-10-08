import { describe, it, expect, vi } from 'vitest'
import type { WorkspaceSessionState } from '../../../../shared/types'
import { buildHydratedTabState } from './tabs-hydration'

vi.stubGlobal('crypto', { randomUUID: () => `uuid-${Math.random().toString(36).slice(2, 8)}` })

// FE-CV-TASK-050-15: a review tab has no backing record, so hydration must never prune it.
describe('buildHydratedTabState keeps review tabs', () => {
  it('restores a review tab next to a terminal tab', () => {
    const session: WorkspaceSessionState = {
      activeRepoId: null,
      activeWorktreeId: null,
      activeTabId: null,
      tabsByWorktree: {},
      terminalLayoutsByTabId: {},
      unifiedTabs: {
        w1: [
          {
            id: 'terminal-1',
            entityId: 'terminal-1',
            groupId: 'g1',
            worktreeId: 'w1',
            contentType: 'terminal',
            label: 'Terminal',
            customLabel: null,
            color: null,
            sortOrder: 0,
            createdAt: 1
          },
          {
            id: 'review-1',
            entityId: 'review-1',
            groupId: 'g1',
            worktreeId: 'w1',
            contentType: 'review',
            label: 'Review',
            customLabel: null,
            color: null,
            sortOrder: 1,
            createdAt: 2
          }
        ]
      },
      tabGroups: {
        w1: [{ id: 'g1', worktreeId: 'w1', activeTabId: 'review-1', tabOrder: ['terminal-1', 'review-1'] }]
      },
      activeGroupIdByWorktree: { w1: 'g1' }
    }

    const result = buildHydratedTabState(session, new Set(['w1']))

    expect(result.unifiedTabsByWorktree.w1.map((t) => t.contentType)).toEqual(['terminal', 'review'])
    expect(result.groupsByWorktree.w1[0].activeTabId).toBe('review-1')
    expect(result.groupsByWorktree.w1[0].tabOrder).toEqual(['terminal-1', 'review-1'])
  })
})
