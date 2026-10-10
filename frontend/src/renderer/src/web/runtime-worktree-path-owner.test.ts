import { describe, expect, it, vi } from 'vitest'
import {
  collectWorktreeListsIgnoringFailures,
  findWorktreeOwningPath
} from './runtime-worktree-path-owner'

describe('collectWorktreeListsIgnoringFailures', () => {
  it('keeps the rows of healthy repos when another repo fails', async () => {
    const onFailure = vi.fn()
    const result = await collectWorktreeListsIgnoringFailures(
      ['offline', 'online'],
      async (view) => {
        if (view === 'offline') {
          throw new Error('WORKTREE_DETECT_FAILED: git worktree list failed')
        }
        return [{ path: '/private/var/x/repo-task-1' }]
      },
      onFailure
    )
    expect(result.worktrees).toEqual([{ path: '/private/var/x/repo-task-1' }])
    expect(result.failedCount).toBe(1)
    expect(onFailure).toHaveBeenCalledWith('offline', expect.any(Error))
  })

  it('returns an empty list when every repo fails', async () => {
    const result = await collectWorktreeListsIgnoringFailures(
      ['a', 'b'],
      () => Promise.reject(new Error('down')),
      () => {}
    )
    expect(result).toEqual({ worktrees: [], failedCount: 2 })
  })
})

describe('findWorktreeOwningPath', () => {
  it('picks the deepest worktree root containing the path', () => {
    const owner = findWorktreeOwningPath(
      [{ path: '/work/repo' }, { path: '/work/repo/nested' }],
      '/work/repo/nested/src/a.ts'
    )
    expect(owner).toEqual({ path: '/work/repo/nested' })
  })

  it('does not treat a sibling with the same prefix as owner', () => {
    expect(findWorktreeOwningPath([{ path: '/work/repo' }], '/work/repo-task-1')).toBeNull()
  })

  it('matches Windows paths case-insensitively with either separator', () => {
    const owner = findWorktreeOwningPath(
      [{ path: 'C:\\Work\\repo-task-1' }],
      'c:/work/repo-task-1/src'
    )
    expect(owner).toEqual({ path: 'C:\\Work\\repo-task-1' })
  })

  it('does not equate a symlinked /var path with its /private/var realpath', () => {
    // Why documented: this is the macOS case the bookkept-row fallback exists for.
    expect(findWorktreeOwningPath([{ path: '/private/var/x/repo' }], '/var/x/repo')).toBeNull()
  })
})
