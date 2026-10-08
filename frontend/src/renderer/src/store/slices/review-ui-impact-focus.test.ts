import { describe, expect, it } from 'vitest'
import { create } from 'zustand'
import { createReviewUiSlice } from './review-ui'
import type { ReviewUiSlice } from './review-ui'

function store() {
  return create<ReviewUiSlice>()((...a) => (createReviewUiSlice as never as (...x: unknown[]) => ReviewUiSlice)(...a))
}

describe('review-ui impact focus and diff selection', () => {
  it('stores the impact focus per worktree', () => {
    const s = store()
    s.getState().setReviewImpactFocus('a', 'k1')
    expect(s.getState().reviewUiByWorktree.a.impactFocusKey).toBe('k1')
    expect(s.getState().reviewUiByWorktree.b).toBeUndefined()
  })
  it('a diff-driven selection does not open the drawer; a user selection does', () => {
    const s = store()
    s.getState().selectReviewSymbolFromDiff('a', 'k1')
    expect(s.getState().reviewUiByWorktree.a).toMatchObject({
      selectedSymbolKey: 'k1',
      selectedSymbolSource: 'diff',
      drawerOpen: false
    })
    s.getState().selectReviewSymbol('a', 'k2')
    expect(s.getState().reviewUiByWorktree.a).toMatchObject({
      selectedSymbolKey: 'k2',
      selectedSymbolSource: 'user',
      drawerOpen: true
    })
  })
})
