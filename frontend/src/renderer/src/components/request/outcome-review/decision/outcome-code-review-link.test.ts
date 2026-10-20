import { describe, expect, it } from 'vitest'
import { makeItem, makeReview } from '../outcome-review.fixture'
import {
  codeReviewAvailability,
  lensForItem,
  lensForSection,
  worktreesOf
} from './outcome-code-review-link'

describe('lenses', () => {
  it('each section goes to the lens of the same thing, and sections with none go nowhere', () => {
    expect(
      [
        'architecture',
        'flow',
        'data',
        'contracts',
        'behavior',
        'requirements',
        'config',
        'weird'
      ].map(lensForSection)
    ).toEqual(['architecture', 'dataflow', 'erd', 'contract', null, null, null, null])
  })

  it('a row that names a file opens the impact of that change', () => {
    expect(
      lensForItem(makeItem({ section: 'flow', evidence: [{ kind: 'file', ref: 'a.go' }] }))
    ).toBe('impact')
    expect(
      lensForItem(makeItem({ section: 'flow', evidence: [{ kind: 'kg', ref: 'CT-02' }] }))
    ).toBe('dataflow')
    expect(lensForItem(makeItem({ section: 'config', evidence: [] }))).toBeNull()
  })
})

describe('worktrees', () => {
  const review = makeReview({
    model: {
      subject: {
        baseRef: 'main',
        heads: [
          { branch: 'a', commit: '1', taskId: 't1' },
          { branch: 'b', commit: '2', taskId: 't2' },
          { branch: 'c', commit: '3', taskId: 't3' },
          { branch: 'd', commit: '4' },
          { branch: 'e', commit: '5', taskId: 't1' }
        ]
      }
    }
  })

  it('takes the worktree of each task the review was made from, once, in order, skipping tasks that have none', () => {
    const map = new Map([
      ['t1', 'wt-1'],
      ['t2', 'wt-1'],
      ['t3', 'wt-3']
    ])
    expect(worktreesOf(review, map)).toEqual([
      { taskId: 't1', worktreeId: 'wt-1', branch: 'a' },
      { taskId: 't3', worktreeId: 'wt-3', branch: 'c' }
    ])
    expect(worktreesOf(review, new Map())).toEqual([])
  })

  it('says why the code review cannot be opened, in a fixed order', () => {
    const wt = [{ taskId: 't1', worktreeId: 'wt-1', branch: 'a' }]
    expect(
      codeReviewAvailability({ codeReviewEnabled: false, lens: 'architecture', worktrees: wt })
    ).toEqual({ state: 'unsupported' })
    expect(codeReviewAvailability({ codeReviewEnabled: true, lens: null, worktrees: wt })).toEqual({
      state: 'no_lens'
    })
    expect(codeReviewAvailability({ codeReviewEnabled: true, lens: 'erd', worktrees: [] })).toEqual(
      { state: 'no_worktree' }
    )
    expect(codeReviewAvailability({ codeReviewEnabled: true, lens: 'erd', worktrees: wt })).toEqual(
      { state: 'ready', worktrees: wt }
    )
  })
})
