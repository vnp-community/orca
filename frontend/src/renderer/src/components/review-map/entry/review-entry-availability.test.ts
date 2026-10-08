import { describe, expect, it } from 'vitest'
import { resolveReviewEntryAvailability } from './review-entry-availability'

const ready = { state: 'ready', worktreeId: 'w', projectId: 'p', environmentId: null } as const
const folder = { state: 'unsupported', reason: 'workspace-scope' } as const

describe('resolveReviewEntryAvailability', () => {
  it('is visible only when enabled and the worktree is addressable', () => {
    expect(resolveReviewEntryAvailability('w', { state: 'enabled' }, ready)).toEqual({ visible: true })
  })
  it.each(['unknown', 'disabled', 'unsupported'] as const)('hides for %s', (state) => {
    expect(resolveReviewEntryAvailability('w', { state }, ready)).toEqual({
      visible: false,
      reason: 'not-enabled'
    })
  })
  it('hides folder workspaces and missing worktrees', () => {
    expect(resolveReviewEntryAvailability('w', { state: 'enabled' }, folder)).toEqual({
      visible: false,
      reason: 'not-git'
    })
    expect(resolveReviewEntryAvailability(null, { state: 'enabled' }, ready)).toEqual({
      visible: false,
      reason: 'no-active-worktree'
    })
  })
})
