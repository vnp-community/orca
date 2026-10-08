// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  DEFAULT_REVIEW_LAYOUT,
  layoutModeForWidth,
  readReviewLayout,
  REVIEW_LAYOUT_STORAGE_KEY,
  writeReviewLayout
} from './review-layout-storage'

afterEach(() => {
  vi.restoreAllMocks()
  window.localStorage.clear()
})

describe('review layout storage', () => {
  it('round-trips and clamps out-of-range sizes', () => {
    writeReviewLayout({ leftSize: 30, rightSize: 33, leftOpen: false })
    expect(readReviewLayout()).toEqual({ leftSize: 30, rightSize: 33, leftOpen: false })
    window.localStorage.setItem(
      REVIEW_LAYOUT_STORAGE_KEY,
      JSON.stringify({ leftSize: 90, rightSize: 1 })
    )
    expect(readReviewLayout()).toMatchObject({ leftSize: 35, rightSize: 20, leftOpen: true })
  })
  it('falls back to defaults on garbage', () => {
    window.localStorage.setItem(REVIEW_LAYOUT_STORAGE_KEY, '{nope')
    expect(readReviewLayout()).toEqual(DEFAULT_REVIEW_LAYOUT)
  })
  it('blocked storage neither throws on read nor on write', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    expect(readReviewLayout()).toEqual(DEFAULT_REVIEW_LAYOUT)
    expect(() => writeReviewLayout(DEFAULT_REVIEW_LAYOUT)).not.toThrow()
  })
  it('chooses column mode by tab width', () => {
    expect([1400, 1100, 1099, 720, 719].map(layoutModeForWidth)).toEqual([
      'three-column',
      'three-column',
      'two-column',
      'two-column',
      'one-column'
    ])
  })
})
