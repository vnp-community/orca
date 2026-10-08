import { describe, expect, it } from 'vitest'
import type { GitBranchCompareSummary } from '../../../../shared/types'
import {
  resolveDefaultReviewScope,
  reviewStateKeyFromOverlayScope,
  scopeKey,
  toChangeOverlayParams,
  type ReviewScope
} from './review-scope-model'

const summary = (over: Partial<GitBranchCompareSummary>): GitBranchCompareSummary => ({
  baseRef: 'origin/main',
  baseOid: 'a',
  compareRef: 'feat',
  headOid: 'b',
  mergeBase: 'a',
  changedFiles: 1,
  status: 'ready',
  ...over
})

describe('resolveDefaultReviewScope', () => {
  it('ready summary gives a branch scope that includes uncommitted work', () => {
    expect(resolveDefaultReviewScope(summary({}))).toEqual({
      status: 'ready',
      scope: { kind: 'branch', baseRef: 'origin/main', includeUncommitted: true }
    })
  })
  it('null and loading are loading', () => {
    expect(resolveDefaultReviewScope(null).status).toBe('loading')
    expect(resolveDefaultReviewScope(summary({ status: 'loading' })).status).toBe('loading')
  })
  it.each(['invalid-base', 'unborn-head', 'no-merge-base', 'error'] as const)(
    '%s is blocked',
    (status) => {
      expect(resolveDefaultReviewScope(summary({ status }))).toMatchObject({
        status: 'blocked',
        reason: status
      })
    }
  )
})

describe('toChangeOverlayParams', () => {
  it('always sends an explicit mode', () => {
    expect(
      toChangeOverlayParams({ kind: 'branch', baseRef: 'm', includeUncommitted: true })
    ).toEqual({ base: 'm', mode: 'worktree' })
    expect(
      toChangeOverlayParams({ kind: 'branch', baseRef: 'm', includeUncommitted: false })
    ).toEqual({ base: 'm', mode: 'committed' })
  })
  it('range and hosted review are committed', () => {
    expect(toChangeOverlayParams({ kind: 'range', baseCommit: 'a', headCommit: 'b' })).toEqual({
      base: 'a',
      head: 'b',
      mode: 'committed'
    })
    expect(
      toChangeOverlayParams({
        kind: 'hostedReview',
        provider: 'github',
        number: 4,
        headSha: 'h',
        baseRefName: 'main'
      })
    ).toEqual({ base: 'main', head: 'h', mode: 'committed' })
  })
})

describe('scopeKey', () => {
  const branch: ReviewScope = { kind: 'branch', baseRef: 'origin/main', includeUncommitted: true }
  it('is stable and distinguishes the three kinds', () => {
    const keys = [
      scopeKey(branch),
      scopeKey({ kind: 'range', baseCommit: 'a', headCommit: 'b' }),
      scopeKey({
        kind: 'hostedReview',
        provider: 'github',
        number: 1,
        headSha: null,
        baseRefName: null
      })
    ]
    expect(new Set(keys).size).toBe(3)
    expect(scopeKey(branch)).toBe(scopeKey({ ...branch }))
  })
  it('uses resolved commits once the overlay scope is known', () => {
    const a = scopeKey(branch, {
      baseRef: 'x',
      mergeBase: 'm1',
      headOid: 'h1',
      mode: 'worktree',
      includesUncommitted: true
    })
    const b = scopeKey(branch, {
      baseRef: 'x',
      mergeBase: 'm2',
      headOid: 'h1',
      mode: 'worktree',
      includesUncommitted: true
    })
    expect(a).not.toBe(b)
    expect(a).toContain('m1..h1')
  })
})

describe('reviewStateKeyFromOverlayScope', () => {
  it('prefers mergeBase, falls back to baseOid, null when incomplete', () => {
    expect(
      reviewStateKeyFromOverlayScope({
        baseRef: 'x',
        baseOid: 'b',
        mergeBase: 'm',
        headOid: 'h',
        mode: 'worktree',
        includesUncommitted: true
      })
    ).toEqual({ baseCommit: 'm', headCommit: 'h' })
    expect(
      reviewStateKeyFromOverlayScope({
        baseRef: 'x',
        baseOid: 'b',
        headOid: 'h',
        mode: 'committed',
        includesUncommitted: false
      })
    ).toEqual({ baseCommit: 'b', headCommit: 'h' })
    expect(
      reviewStateKeyFromOverlayScope({
        baseRef: 'x',
        mode: 'committed',
        includesUncommitted: false
      })
    ).toBeNull()
    expect(reviewStateKeyFromOverlayScope(null)).toBeNull()
  })
})
