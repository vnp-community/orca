import { describe, it, expect } from 'vitest'
import { parseWorkspaceSession } from './workspace-session-schema'

// FE-CV-TASK-050-15: persisted sessions may carry review tabs and a review active tab type.
function session(overrides: Record<string, unknown> = {}) {
  return {
    activeRepoId: null,
    activeWorktreeId: 'w1',
    activeTabId: null,
    tabsByWorktree: {},
    terminalLayoutsByTabId: {},
    ...overrides
  }
}

describe('parseWorkspaceSession review tabs', () => {
  it('accepts a unified tab with contentType review', () => {
    const result = parseWorkspaceSession(
      session({
        unifiedTabs: {
          w1: [
            {
              id: 'review-1',
              entityId: 'review-1',
              groupId: 'g1',
              worktreeId: 'w1',
              contentType: 'review',
              label: 'Review',
              customLabel: null,
              color: null,
              sortOrder: 0,
              createdAt: 1
            }
          ]
        },
        tabGroups: { w1: [{ id: 'g1', worktreeId: 'w1', activeTabId: 'review-1', tabOrder: ['review-1'] }] }
      })
    )
    expect(result.ok).toBe(true)
  })

  it('accepts activeTabTypeByWorktree = review', () => {
    const result = parseWorkspaceSession(session({ activeTabTypeByWorktree: { w1: 'review' } }))
    expect(result.ok).toBe(true)
  })

  it('still rejects an unknown content type', () => {
    const result = parseWorkspaceSession(
      session({
        unifiedTabs: {
          w1: [
            {
              id: 'x',
              entityId: 'x',
              groupId: 'g1',
              worktreeId: 'w1',
              contentType: 'not-a-type',
              label: 'X',
              customLabel: null,
              color: null,
              sortOrder: 0,
              createdAt: 1
            }
          ]
        }
      })
    )
    expect(result.ok).toBe(false)
  })
})
