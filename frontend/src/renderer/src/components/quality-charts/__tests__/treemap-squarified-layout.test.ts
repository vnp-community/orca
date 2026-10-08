import { describe, expect, it } from 'vitest'
import { buildTreemapLayout } from '../../status-bar/workspace-space-layout'
import { squarify, type SquarifyRect } from '../treemap-squarified-layout'

function seeded(seed: number): () => number {
  let s = seed
  return () => {
    s = (s * 1664525 + 1013904223) % 4294967296
    return s / 4294967296
  }
}

function overlaps(a: SquarifyRect, b: SquarifyRect): boolean {
  const e = 1e-6
  return (
    a.x < b.x + b.width - e &&
    b.x < a.x + a.width - e &&
    a.y < b.y + b.height - e &&
    b.y < a.y + a.height - e
  )
}

function checkInvariants(
  rects: SquarifyRect[],
  bounds: { x: number; y: number; width: number; height: number }
): void {
  const area = rects.reduce((s, r) => s + r.width * r.height, 0)
  expect(
    Math.abs(area - bounds.width * bounds.height) / (bounds.width * bounds.height)
  ).toBeLessThan(0.005)
  for (const r of rects) {
    expect(r.x).toBeGreaterThanOrEqual(bounds.x - 1e-6)
    expect(r.y).toBeGreaterThanOrEqual(bounds.y - 1e-6)
    expect(r.x + r.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1e-6)
    expect(r.y + r.height).toBeLessThanOrEqual(bounds.y + bounds.height + 1e-6)
  }
}

const BOUNDS = { x: 0, y: 0, width: 800, height: 500 }

describe('squarify', () => {
  it('handles one and two items', () => {
    const one = squarify([{ id: 'a', size: 5 }], BOUNDS)
    expect(one.rects).toEqual([{ id: 'a', ...BOUNDS }])
    const two = squarify(
      [
        { id: 'a', size: 3 },
        { id: 'b', size: 1 }
      ],
      BOUNDS
    )
    expect(two.rects).toHaveLength(2)
    checkInvariants(two.rects, BOUNDS)
  })

  it('fills the frame with no overlaps for 400 random items', () => {
    const rnd = seeded(7)
    const items = Array.from({ length: 400 }, (_, i) => ({ id: `n${i}`, size: 1 + rnd() * 1000 }))
    const { rects } = squarify(items, BOUNDS)
    expect(rects).toHaveLength(400)
    checkInvariants(rects, BOUNDS)
    for (let i = 0; i < rects.length; i += 7) {
      for (let j = i + 1; j < rects.length; j += 5) {
        expect(overlaps(rects[i], rects[j])).toBe(false)
      }
    }
  })

  it('is deterministic and stable for equal sizes', () => {
    const items = ['c', 'a', 'b', 'd'].map((id) => ({ id, size: 10 }))
    const first = squarify(items, BOUNDS)
    const second = squarify([...items].toReversed(), BOUNDS)
    expect(second).toEqual(first)
    checkInvariants(first.rects, BOUNDS)
  })

  it('handles a dominant item, equal items and thin frames', () => {
    const dominant = squarify(
      [
        { id: 'big', size: 1e6 },
        { id: 'x', size: 1 },
        { id: 'y', size: 1 }
      ],
      BOUNDS
    )
    checkInvariants(dominant.rects, BOUNDS)
    const thin = { x: 0, y: 0, width: 1000, height: 2 }
    checkInvariants(
      squarify(
        Array.from({ length: 20 }, (_, i) => ({ id: `${i}`, size: 1 })),
        thin
      ).rects,
      thin
    )
  })

  it('omits invalid sizes and empty frames without throwing', () => {
    const r = squarify(
      [
        { id: 'a', size: -1 },
        { id: 'b', size: Number.NaN },
        { id: 'c', size: 0 },
        { id: 'd', size: 2 }
      ],
      BOUNDS
    )
    expect(r.omitted).toEqual(['a', 'b', 'c'])
    expect(r.rects.map((x) => x.id)).toEqual(['d'])
    expect(squarify([{ id: 'a', size: 1 }], { x: 0, y: 0, width: 0, height: 10 }).rects).toEqual([])
    expect(squarify([], BOUNDS).rects).toEqual([])
  })

  it('achieves a better mean aspect ratio than the bisecting status-bar treemap', () => {
    const rnd = seeded(11)
    const sizes = Array.from({ length: 120 }, (_, i) => ({
      id: `n${i}`,
      size: 1 + Math.floor(rnd() * rnd() * 5000)
    }))
    const bounds = { x: 0, y: 0, width: 100, height: 100 }
    const aspect = (w: number, h: number): number => Math.max(w / h, h / w)
    const mean = (xs: number[]): number => xs.reduce((a, b) => a + b, 0) / xs.length
    const sq = mean(squarify(sizes, bounds).rects.map((r) => aspect(r.width, r.height)))
    const base = mean(
      buildTreemapLayout(
        sizes.map((s) => ({ id: s.id, label: s.id, sizeBytes: s.size }) as never)
      ).map((r) => aspect(r.width, r.height))
    )
    console.info(`[perf] mean aspect ratio squarified=${sq.toFixed(2)} bisect=${base.toFixed(2)}`)
    expect(sq).toBeLessThanOrEqual(base)
  })
})
