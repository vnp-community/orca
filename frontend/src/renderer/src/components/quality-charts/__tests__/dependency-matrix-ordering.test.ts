import { describe, expect, it } from 'vitest'
import { orderByStrongComponents } from '../dependency-matrix-ordering'

describe('orderByStrongComponents', () => {
  it('orders a DAG topologically with no blocks', () => {
    const nodes = ['d', 'c', 'b', 'a']
    const edges = [
      { from: 'a', to: 'b' },
      { from: 'b', to: 'c' },
      { from: 'a', to: 'd' },
      { from: 'd', to: 'c' }
    ]
    const { order, blocks } = orderByStrongComponents(nodes, edges)
    expect(blocks).toEqual([])
    for (const e of edges) {
      expect(order.indexOf(e.from)).toBeLessThan(order.indexOf(e.to))
    }
    expect([...order].sort()).toEqual(['a', 'b', 'c', 'd'])
  })

  it('groups a 3-node cycle into one block', () => {
    const { order, blocks } = orderByStrongComponents(
      ['x', 'a', 'b', 'c'],
      [
        { from: 'a', to: 'b' },
        { from: 'b', to: 'c' },
        { from: 'c', to: 'a' },
        { from: 'x', to: 'a' }
      ]
    )
    expect(blocks).toEqual([['a', 'b', 'c']])
    expect(order[0]).toBe('x')
    expect(order.slice(1)).toEqual(['a', 'b', 'c'])
  })

  it('keeps two disjoint cycles as two blocks', () => {
    const { blocks } = orderByStrongComponents(
      ['a', 'b', 'c', 'd'],
      [
        { from: 'a', to: 'b' },
        { from: 'b', to: 'a' },
        { from: 'c', to: 'd' },
        { from: 'd', to: 'c' }
      ]
    )
    expect(blocks).toEqual([
      ['a', 'b'],
      ['c', 'd']
    ])
  })

  it('marks self loops as blocks and ignores unknown nodes', () => {
    const { order, blocks } = orderByStrongComponents(
      ['a', 'b'],
      [
        { from: 'a', to: 'a' },
        { from: 'a', to: 'ghost' }
      ]
    )
    expect(blocks).toEqual([['a']])
    expect(order).toEqual(['a', 'b'])
  })

  it('handles empty graphs and duplicate node ids', () => {
    expect(orderByStrongComponents([], [])).toEqual({ order: [], blocks: [] })
    expect(orderByStrongComponents(['a', 'a'], []).order).toEqual(['a'])
  })

  it('handles a complete 60-node graph as one block', () => {
    const nodes = Array.from({ length: 60 }, (_, i) => `n${i}`)
    const edges = nodes.flatMap((a) =>
      nodes.filter((b) => b !== a).map((b) => ({ from: a, to: b }))
    )
    const { order, blocks } = orderByStrongComponents(nodes, edges)
    expect(blocks).toHaveLength(1)
    expect(order).toEqual(nodes)
  })

  it('survives a long chain without recursion limits', () => {
    const nodes = Array.from({ length: 20000 }, (_, i) => `n${i}`)
    const edges = nodes.slice(1).map((to, i) => ({ from: nodes[i], to }))
    expect(orderByStrongComponents(nodes, edges).order).toEqual(nodes)
  })
})
