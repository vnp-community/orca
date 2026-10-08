import { describe, expect, it } from 'vitest'
import { create } from 'zustand'
import { createReviewUiSlice, DEFAULT_REVIEW_UI_STATE, type ReviewUiSlice } from './review-ui'

function makeStore() {
  return create<ReviewUiSlice>()(
    (...a) =>
      createReviewUiSlice(
        ...(a as unknown as Parameters<typeof createReviewUiSlice>)
      ) as ReviewUiSlice
  )
}

describe('review-ui slice', () => {
  const scope = { kind: 'branch', baseRef: 'main', includeUncommitted: true } as const

  it('toggling the same chip twice clears it; only one filter at a time', () => {
    const s = makeStore()
    s.getState().toggleReviewChipFilter('w', 'files')
    expect(s.getState().reviewUiByWorktree.w.chipFilter).toBe('files')
    s.getState().toggleReviewChipFilter('w', 'untested')
    expect(s.getState().reviewUiByWorktree.w.chipFilter).toBe('untested')
    s.getState().toggleReviewChipFilter('w', 'untested')
    expect(s.getState().reviewUiByWorktree.w.chipFilter).toBeNull()
  })

  it('changing scope clears chip filter, selection and drawer', () => {
    const s = makeStore()
    s.getState().toggleReviewChipFilter('w', 'files')
    s.getState().selectReviewSymbol('w', 'sym')
    expect(s.getState().reviewUiByWorktree.w.drawerOpen).toBe(true)
    s.getState().setReviewScope('w', scope)
    expect(s.getState().reviewUiByWorktree.w).toMatchObject({
      scope,
      chipFilter: null,
      selectedSymbolKey: null,
      drawerOpen: false
    })
  })

  it('worktrees are independent and defaults are not mutated', () => {
    const s = makeStore()
    s.getState().setReviewLens('a', 'erd')
    expect(s.getState().reviewUiByWorktree.b).toBeUndefined()
    expect(DEFAULT_REVIEW_UI_STATE.lens).toBeNull()
  })

  it('selecting null closes the drawer', () => {
    const s = makeStore()
    s.getState().selectReviewSymbol('w', 'x')
    s.getState().selectReviewSymbol('w', null)
    expect(s.getState().reviewUiByWorktree.w).toMatchObject({
      selectedSymbolKey: null,
      drawerOpen: false
    })
  })
})

describe('review-ui slice: architecture and data-flow selection', () => {
  it('keeps the c4 container and data-flow id per worktree', () => {
    const s = makeStore()
    s.getState().setReviewC4Container('w', 'svc')
    s.getState().setReviewDataFlowId('w', 'flow-1')
    expect(s.getState().reviewUiByWorktree.w).toMatchObject({ c4ContainerId: 'svc', dataFlowId: 'flow-1' })
    expect(s.getState().reviewUiByWorktree.other).toBeUndefined()
  })

  it('caps c4 drafts at 8 containers, evicting the least recently edited', () => {
    const s = makeStore()
    for (let i = 0; i < 9; i += 1) {
      s.getState().setReviewC4Draft('w', `c${i}`, `a: ${i}`)
    }
    const drafts = s.getState().reviewUiByWorktree.w.c4Drafts ?? {}
    expect(Object.keys(drafts)).toHaveLength(8)
    expect(drafts.c0).toBeUndefined()
    expect(drafts.c8).toBe('a: 8')
  })

  it('rejects oversize drafts and clears a draft with null', () => {
    const s = makeStore()
    s.getState().setReviewC4Draft('w', 'c', 'a: 1')
    s.getState().setReviewC4Draft('w', 'c', 'x'.repeat(64 * 1024 + 1))
    expect(s.getState().reviewUiByWorktree.w.c4Drafts?.c).toBe('a: 1')
    s.getState().setReviewC4Draft('w', 'c', null)
    expect(s.getState().reviewUiByWorktree.w.c4Drafts?.c).toBeUndefined()
  })
})
