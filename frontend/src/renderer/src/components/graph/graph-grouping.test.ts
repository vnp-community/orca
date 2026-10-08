import { describe, expect, it } from 'vitest'
import { VISIBLE_NODE_LIMIT, buildGroupEdges, pickVisibleNodes } from './graph-grouping'
import { gEdge, gNode, gPayload } from './graph-test-fixtures'

const many = (n: number, over: (i: number) => Partial<ReturnType<typeof gNode>> = () => ({})) =>
  Array.from({ length: n }, (_, i) => gNode(`n${String(i).padStart(3, '0')}`, { group: `svc${i % 3}/mod`, ...over(i) }))

describe('pickVisibleNodes', () => {
  it('shows everything at or under the limit', () => {
    const r = pickVisibleNodes(gPayload(many(VISIBLE_NODE_LIMIT)), { openGroups: new Set() })
    expect(r.visible).toHaveLength(VISIBLE_NODE_LIMIT)
    expect(r.groups).toEqual([])
  })

  it('caps at the limit and folds the rest by group with counts', () => {
    const r = pickVisibleNodes(gPayload(many(80)), { openGroups: new Set() })
    expect(r.visible).toHaveLength(50)
    expect(r.groups.reduce((s, g) => s + g.count, 0)).toBe(30)
    expect(r.groups[0].byKind.module).toBeGreaterThan(0)
  })

  it('prioritises changed nodes, then risk, then selected neighbors, then id', () => {
    const nodes = many(60, (i) => ({ risk: i === 59 ? 'critical' : 'low' }))
    const edges = [gEdge('n058', 'n057', { change: 'added' })]
    const r = pickVisibleNodes(gPayload(nodes, edges), { openGroups: new Set(), limit: 3 })
    expect(r.visible.map((n) => n.id).sort()).toEqual(['n057', 'n058', 'n059'])
  })

  it('keeps the selected node and its neighbors ahead of id order', () => {
    const nodes = many(10)
    const r = pickVisibleNodes(gPayload(nodes, [gEdge('n009', 'n008')]), { openGroups: new Set(), selectedId: 'n009', limit: 2 })
    const ids = r.visible.map((n) => n.id)
    expect(ids).toContain('n009')
    expect(ids).toContain('n008')
  })

  it('is stable for the same input and does not count open groups against the limit', () => {
    const p = gPayload(many(80))
    const a = pickVisibleNodes(p, { openGroups: new Set(['svc0/mod']) })
    const b = pickVisibleNodes(p, { openGroups: new Set(['svc0/mod']) })
    expect(a).toEqual(b)
    const open = p.nodes.filter((n) => n.group === 'svc0/mod').length
    expect(a.visible.length).toBeGreaterThan(50)
    expect(a.visible.filter((n) => n.group === 'svc0/mod')).toHaveLength(open)
  })

  it('group maxRisk keeps unknown distinct from low', () => {
    const nodes = many(5, () => ({ risk: 'unknown', group: 'g' }))
    const r = pickVisibleNodes(gPayload(nodes), { openGroups: new Set(), limit: 2 })
    expect(r.groups[0].maxRisk).toBe('unknown')
  })
})

describe('buildGroupEdges', () => {
  it('lifts edges to group nodes, merges duplicates and drops intra-group edges', () => {
    const nodes = [gNode('a'), gNode('b', { group: 'g' }), gNode('c', { group: 'g' })]
    const edges = [gEdge('a', 'b'), gEdge('a', 'c'), gEdge('b', 'c')]
    const groups = [{ id: 'g', label: 'g', count: 2, byKind: { module: 2 }, memberIds: ['b', 'c'], maxRisk: 'low' as const }]
    const out = buildGroupEdges(gPayload(nodes, edges), [nodes[0]], groups)
    expect(out).toEqual([{ from: 'a', to: 'group:g', kind: 'calls', change: 'unchanged', weight: 2 }])
  })
})
