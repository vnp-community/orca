import { describe, expect, it } from 'vitest'
import { createLinearScale, niceDomain } from '../chart-linear-scale'

describe('chart-linear-scale', () => {
  it('maps and inverts', () => {
    const s = createLinearScale([0, 10], [100, 0])
    expect(s.scale(0)).toBe(100)
    expect(s.scale(10)).toBe(0)
    for (const v of [0, 2.5, 7, 10]) {
      expect(s.invert(s.scale(v))).toBeCloseTo(v, 9)
    }
  })

  it('widens a degenerate domain by one unit', () => {
    const s = createLinearScale([5, 5], [0, 10])
    expect(s.domain).toEqual([5, 6])
    expect(niceDomain(5, 5)[1]).toBeGreaterThan(5)
  })

  it('produces non-repeating nice ticks inside the domain', () => {
    const s = createLinearScale([0, 10], [0, 100])
    expect(s.ticks(5)).toEqual([0, 2, 4, 6, 8, 10])
    const decimals = createLinearScale([0, 0.3], [0, 1]).ticks(3)
    expect(new Set(decimals).size).toBe(decimals.length)
    expect(decimals).toContain(0.1)
    expect(decimals.every((t) => t >= 0 && t <= 0.3)).toBe(true)
  })

  it('handles negative, huge and reversed domains', () => {
    expect(createLinearScale([-10, 10], [0, 1]).ticks(4)).toContain(0)
    const big = createLinearScale([0, 1e9], [0, 1]).ticks(5)
    expect(big.length).toBeGreaterThan(1)
    expect(createLinearScale([10, 0], [0, 1]).ticks(5)).toEqual([0, 2, 4, 6, 8, 10])
  })

  it('does not throw on NaN or Infinity', () => {
    const s = createLinearScale([Number.NaN, 1], [0, 1])
    expect(s.valid).toBe(false)
    expect(s.ticks(5)).toEqual([])
    expect(Number.isNaN(s.scale(1))).toBe(false)
    expect(createLinearScale([0, Infinity], [0, 1]).ticks()).toEqual([])
    expect(niceDomain(Number.NaN, 2)).toEqual([0, 1])
  })
})
