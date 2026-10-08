import { describe, expect, it } from 'vitest'
import { toPercent } from './coverage-percent-normalization'

// Contract D5: coverage numbers are ratios 0..1 (the solution first assumed 0..100).
describe('toPercent', () => {
  it.each([
    [0, 0],
    [1, 100],
    [0.62, 62],
    [0.07, 7],
    [0.123456, 12.35]
  ])('turns ratio %s into %s percent', (ratio, percent) => {
    expect(toPercent(ratio)).toBe(percent)
  })

  it.each([[62], [1.01], [-1], [-0.0001], [Number.NaN], [Number.POSITIVE_INFINITY]])(
    'rejects %s instead of clamping it',
    (value) => {
      expect(toPercent(value)).toBeNull()
    }
  )

  it('treats missing values as null, never 0', () => {
    expect(toPercent(null)).toBeNull()
    expect(toPercent(undefined)).toBeNull()
  })
})
