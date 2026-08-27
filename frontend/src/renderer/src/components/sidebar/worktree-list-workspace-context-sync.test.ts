import { describe, expect, it } from 'vitest'
import type { Worktree } from '../../../../shared/types'
import { toWorkspaceContextWorktree } from './WorktreeList'

function makeWorktree(overrides: Partial<Worktree> = {}): Worktree {
  return {
    id: 'repo-1::/tmp/wt-1',
    repoId: 'repo-1',
    path: '/tmp/wt-1',
    branch: 'feature/foo',
    head: 'abc123',
    isBare: false,
    isMainWorktree: false,
    displayName: 'wt-1',
    comment: '',
    linkedIssue: null,
    linkedPR: null,
    linkedLinearIssue: null,
    isArchived: false,
    isUnread: false,
    isPinned: false,
    sortOrder: 0,
    lastActivityAt: 1,
    ...overrides
  }
}

describe('toWorkspaceContextWorktree', () => {
  it('maps the sidebar Worktree (& GitWorktreeInfo) into WorkspaceContext\'s smaller shape', () => {
    const worktree = makeWorktree({ createdAt: 42 })

    expect(toWorkspaceContextWorktree(worktree)).toEqual({
      id: 'repo-1::/tmp/wt-1',
      path: '/tmp/wt-1',
      branch: 'feature/foo',
      isMain: false,
      createdAt: 42
    })
  })

  it('carries isMainWorktree through as isMain', () => {
    const worktree = makeWorktree({ isMainWorktree: true })

    expect(toWorkspaceContextWorktree(worktree).isMain).toBe(true)
  })
})
