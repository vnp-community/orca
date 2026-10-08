/**
 * LayoutEngine contract — FE-REQ-TASK-032-05 / 032-07
 *
 * Any engine plugged into GraphCanvas (`layout` prop) must pass this suite. It runs
 * against `waveLayoutEngine` and an async, worker-like stub that stands in for an
 * `elkjs` engine until that dependency is approved (032-07 stays BLOCKED).
 */
import { describe, expect, it } from 'vitest'
import { computeWaveLayout, waveLayoutEngine, type LayoutEngine } from './graph-layout-engine'
import { gEdge, gNode } from './graph-test-fixtures'
import type { GraphEdge, GraphNode } from '../../../../shared/graph-types'

const opts = {
  direction: 'LR' as const,
  groupOf: (id: string) => (id.startsWith('g') ? 'grp' : null)
}

/** Simulates an off-thread engine: resolves on a later macrotask, like a Worker reply. */
const deferredEngine: LayoutEngine = (nodes, edges, options) =>
  new Promise((resolve) => setTimeout(() => resolve(computeWaveLayout(nodes, edges, options)), 5))

function chain(n: number): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const nodes = Array.from({ length: n }, (_, i) =>
    gNode(`n${i}`, { group: `s${i % 7}/m${i % 3}` })
  )
  const edges = nodes.slice(1).map((node, i) => gEdge(nodes[i].id, node.id))
  return { nodes, edges }
}

function layoutEngineContract(name: string, engine: LayoutEngine): void {
  describe(`LayoutEngine contract: ${name}`, () => {
    it('returns a finite position for every node and nothing else', async () => {
      const nodes = ['a', 'b', 'c', 'g1'].map((id) => gNode(id))
      const p = await engine(nodes, [gEdge('a', 'b')], opts)
      expect(Object.keys(p).sort()).toEqual(['a', 'b', 'c', 'g1'])
      for (const pos of Object.values(p)) {
        expect(Number.isFinite(pos.x) && Number.isFinite(pos.y)).toBe(true)
      }
    })

    it('is deterministic and does not mutate its inputs', async () => {
      const { nodes, edges } = chain(40)
      const before = JSON.stringify({ nodes, edges })
      const [p1, p2] = await Promise.all([engine(nodes, edges, opts), engine(nodes, edges, opts)])
      expect(p1).toEqual(p2)
      expect(JSON.stringify({ nodes, edges })).toBe(before)
    })

    it('terminates on cycles and ignores edges to unknown nodes', async () => {
      const nodes = ['a', 'b', 'c'].map((id) => gNode(id))
      const p = await engine(
        nodes,
        [gEdge('a', 'b'), gEdge('b', 'c'), gEdge('c', 'a'), gEdge('a', 'ghost')],
        opts
      )
      expect(Object.keys(p).sort()).toEqual(['a', 'b', 'c'])
    })

    it('gives distinct positions to distinct nodes and handles an empty graph', async () => {
      const { nodes, edges } = chain(25)
      const p = await engine(nodes, edges, opts)
      expect(new Set(Object.values(p).map((v) => `${v.x},${v.y}`)).size).toBe(25)
      expect(await engine([], [], opts)).toEqual({})
    })
  })
}

layoutEngineContract('waveLayoutEngine', waveLayoutEngine)
layoutEngineContract('deferred (worker-like) engine', deferredEngine)

describe('waveLayoutEngine size budget (proposal: 2,000 nodes, CR-REQ-032 2.6)', () => {
  it.each([50, 500, 2000])('lays out a %i-node graph well under one second', async (n) => {
    const { nodes, edges } = chain(n)
    const t0 = performance.now()
    const p = await waveLayoutEngine(nodes, edges, opts)
    const ms = performance.now() - t0
    expect(Object.keys(p)).toHaveLength(n)
    // Generous bound so CI noise never flakes; the measured number is printed for the spec.
    expect(ms).toBeLessThan(1000)
    console.info(`[graph-layout] wave ${n} nodes: ${ms.toFixed(1)} ms`)
  })
})
