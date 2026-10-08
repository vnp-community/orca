import { describe, expect, it } from 'vitest'
import { bucketIntensity, bucketIntensityByQuantile } from '../heat-intensity-scale'

describe('heat-intensity-scale', () => {
  it('maps domain edges to 1 and 5', () => {
    const d = { min: 0, max: 100 }
    expect(bucketIntensity(0, d)).toBe(1)
    expect(bucketIntensity(100, d)).toBe(5)
    expect(bucketIntensity(50, d)).toBe(3)
    expect(bucketIntensity(-20, d)).toBe(1)
    expect(bucketIntensity(500, d)).toBe(5)
  })

  it('returns 3 for a degenerate domain and null for missing values', () => {
    expect(bucketIntensity(7, { min: 7, max: 7 })).toBe(3)
    expect(bucketIntensity(null, { min: 0, max: 1 })).toBeNull()
    expect(bucketIntensity(Number.NaN, { min: 0, max: 1 })).toBeNull()
  })

  it('quantile buckets are stable for ties and skip nulls', () => {
    const out = bucketIntensityByQuantile([1, 1, 1, null, 10, 20])
    expect(out[0]).toBe(out[1])
    expect(out[3]).toBeNull()
    expect(out[5]).toBe(5)
    expect(out[0]).toBeLessThan(out[5] as number)
    expect(bucketIntensityByQuantile([4])).toEqual([3])
  })
})
