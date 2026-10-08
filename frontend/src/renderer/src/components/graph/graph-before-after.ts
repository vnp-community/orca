/**
 * Before/after change view — FE-REQ-TASK-032-04
 *
 * One graph, two projections. Never mutates the input payload.
 *
 * @module components/graph/graph-before-after
 */

import type { GraphEdge, GraphPayload } from '../../../../shared/graph-types'

export type GraphChangeView = 'before' | 'after'
export type GraphViewPayload = Omit<GraphPayload, 'edges'> & { edges: (GraphEdge & { dim?: boolean })[] }

export function hasChangeAxis(payload: GraphPayload): boolean {
  return payload.edges.some((e) => e.change !== 'unchanged')
}

export function applyChangeView(payload: GraphPayload, view: GraphChangeView): GraphViewPayload {
  if (!hasChangeAxis(payload)) {return payload}

  if (view === 'after') {
    return { ...payload, edges: payload.edges.map((e) => (e.change === 'removed' ? { ...e, dim: true } : { ...e })) }
  }

  const edges = payload.edges
    .filter((e) => e.change !== 'added')
    .map((e) => (e.change === 'removed' ? { ...e, change: 'unchanged' as const } : { ...e }))
  const touched = new Set<string>()
  for (const e of edges) {
    touched.add(e.from)
    touched.add(e.to)
  }
  const onlyAddedEdges = new Set<string>()
  for (const e of payload.edges) {
    if (e.change === 'added') {
      onlyAddedEdges.add(e.from)
      onlyAddedEdges.add(e.to)
    }
  }
  // Why: a node introduced by this change has no "before" form; keep it only if something else still links to it.
  const nodes = payload.nodes.filter((n) => !(n.status === 'added' && onlyAddedEdges.has(n.id) && !touched.has(n.id)))
  return { ...payload, nodes, edges }
}
