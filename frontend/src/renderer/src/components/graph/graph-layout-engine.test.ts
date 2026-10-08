import { describe, expect, it } from 'vitest'
import { HORIZONTAL_GAP, VERTICAL_GAP, computeWaveLayout, layoutCacheKey, waveLayoutEngine } from './graph-layout-engine'
import { gEdge, gNode } from './graph-test-fixtures'

const opts = { direction: 'LR' as const, groupOf: () => null }

describe('wave layout', () => {
  it('places a chain in three waves', () => {
    const p = computeWaveLayout([gNode('a'), gNode('b'), gNode('c')], [gEdge('a', 'b'), gEdge('b', 'c')], opts)
    expect([p.a.x, p.b.x, p.c.x]).toEqual([0, HORIZONTAL_GAP, 2 * HORIZONTAL_GAP])
  })

  it('handles a diamond and isolated nodes (wave 0)', () => {
    const nodes = ['a', 'b', 'c', 'd', 'z'].map((id) => gNode(id))
    const p = computeWaveLayout(nodes, [gEdge('a', 'b'), gEdge('a', 'c'), gEdge('b', 'd'), gEdge('c', 'd')], opts)
    expect(p.d.x).toBe(2 * HORIZONTAL_GAP)
    expect(p.b.x).toBe(p.c.x)
    expect(p.z.x).toBe(0)
  })

  it('terminates on cycles', () => {
    const p = computeWaveLayout([gNode('a'), gNode('b')], [gEdge('a', 'b'), gEdge('b', 'a')], opts)
    expect(Object.keys(p)).toHaveLength(2)
  })

  it('is deterministic, groups nodes within a wave and supports TB', async () => {
    const nodes = [gNode('b', { group: 'y' }), gNode('a', { group: 'y' }), gNode('c', { group: 'x' })]
    const o = { direction: 'TB' as const, groupOf: (id: string) => nodes.find((n) => n.id === id)?.group ?? null }
    const first = await waveLayoutEngine(nodes, [], o)
    expect(await waveLayoutEngine(nodes.toReversed(), [], o)).toEqual(first)
    expect(first.c.x).toBe(0)
    expect(first.a.x).toBe(HORIZONTAL_GAP)
    expect(first.a.y).toBe(0)
    expect(VERTICAL_GAP).toBeGreaterThan(0)
  })

  it('builds cache keys', () => {
    expect(layoutCacheKey('p', 'impact', 'g1')).toBe('p|impact|g1')
  })
})
