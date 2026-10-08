import { describe, expect, it } from 'vitest'
import type { ModuleGraph } from '../../../../../shared/code-intel-graph-types'
import { buildStructureGraph } from '../../../test-support/code-intel-quality-visualization-fake-data'
import { DEPENDENCY_OTHER_ID, reduceDependencyGraph } from './quality-dependency-graph-reduction'

const other = (n: number): string => `Other (${n})`
const node = (id: string) => ({ id, kind: 'file' as const, symbolCount: 1 })

describe('reduceDependencyGraph', () => {
  it('keeps only import edges and drops self edges, counting them', () => {
    const model = reduceDependencyGraph(buildStructureGraph('dag'), other)
    expect(model.edges.every((e) => e.from !== e.to)).toBe(true)
    expect(model.edges).toHaveLength(7)
    const withSelf: ModuleGraph = {
      nodes: [node('a'), node('b')],
      edges: [
        { from: 'a', to: 'a', kind: 'imports', count: 2 },
        { from: 'a', to: 'b', kind: 'imports', count: 1 }
      ]
    }
    expect(reduceDependencyGraph(withSelf, other).selfEdges).toBe(1)
  })

  it('handles an empty or missing graph', () => {
    expect(reduceDependencyGraph({ nodes: [], edges: [] }, other)).toMatchObject({
      nodes: [],
      edges: [],
      total: 0,
      shown: 0
    })
    expect(reduceDependencyGraph(null, other).nodes).toEqual([])
  })

  it('keeps a cycle as two opposite edges', () => {
    const model = reduceDependencyGraph(buildStructureGraph('cycle'), other)
    const pair = model.edges.filter(
      (e) =>
        ['src/mod-1', 'src/mod-3'].includes(e.from) && ['src/mod-1', 'src/mod-3'].includes(e.to)
    )
    expect(pair).toHaveLength(1)
    expect(model.edges.some((e) => e.from === 'src/mod-3' && e.to === 'src/mod-1')).toBe(true)
  })

  it('folds 90 modules into 59 busiest plus one Other node of the right size', () => {
    const model = reduceDependencyGraph(buildStructureGraph('large'), other)
    expect(model.nodes).toHaveLength(60)
    expect(model.shown).toBe(59)
    expect(model.total).toBe(90)
    expect(model.otherCount).toBe(31)
    expect(model.nodes.at(-1)).toEqual({ id: DEPENDENCY_OTHER_ID, label: 'Other (31)' })
  })

  it('merges edges into Other with summed weights and no Other self edge', () => {
    const graph: ModuleGraph = {
      nodes: ['a', 'b', 'c', 'd'].map(node),
      edges: [
        { from: 'a', to: 'b', kind: 'imports', count: 5 },
        { from: 'a', to: 'c', kind: 'imports', count: 2 },
        { from: 'a', to: 'd', kind: 'imports', count: 3 },
        { from: 'c', to: 'd', kind: 'imports', count: 9 }
      ]
    }
    // Degrees: d=12, c=11, a=10, b=5, so a and b fold into Other.
    const model = reduceDependencyGraph(graph, other, 3)
    expect(model.nodes.map((n) => n.id)).toEqual(['d', 'c', DEPENDENCY_OTHER_ID])
    expect(model.edges.find((e) => e.from === DEPENDENCY_OTHER_ID && e.to === 'c')?.weight).toBe(2)
    expect(model.edges.find((e) => e.from === DEPENDENCY_OTHER_ID && e.to === 'd')?.weight).toBe(3)
    expect(
      model.edges.some((e) => e.from === DEPENDENCY_OTHER_ID && e.to === DEPENDENCY_OTHER_ID)
    ).toBe(false)
  })

  it('uses the last two path segments as the label', () => {
    const model = reduceDependencyGraph(
      { nodes: [node('src/renderer/components/Foo.tsx')], edges: [] },
      other
    )
    expect(model.nodes[0].label).toBe('components/Foo.tsx')
  })
})
