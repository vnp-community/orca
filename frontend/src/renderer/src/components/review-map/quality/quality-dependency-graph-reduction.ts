/**
 * quality-dependency-graph-reduction.ts — FE-CV-TASK-087-15
 *
 * ModuleGraph -> a DSM-sized graph: only `imports` edges, self-edges dropped, the busiest
 * MAX-1 modules kept and the rest folded into one "Other" node whose edges are merged.
 * Every cut is reported so the UI can say "X/Y".
 *
 * @module components/review-map/quality/quality-dependency-graph-reduction
 */

import type { ModuleGraph } from '../../../../../shared/code-intel-graph-types'
import type { MatrixEdge, MatrixNode } from '../../quality-charts/DependencyMatrix'

export const DEPENDENCY_MAX_NODES = 60
export const DEPENDENCY_OTHER_ID = 'quality-dependency:other'

export type DependencyGraphModel = {
  nodes: MatrixNode[]
  edges: MatrixEdge[]
  /** Distinct modules before folding. */
  total: number
  /** Modules drawn as their own row (excludes the "Other" node). */
  shown: number
  otherCount: number
  selfEdges: number
}

function shortLabel(id: string): string {
  const parts = id.split('/').filter(Boolean)
  return parts.length > 2 ? parts.slice(-2).join('/') : id
}

export function reduceDependencyGraph(
  graph: Partial<ModuleGraph> | null | undefined,
  otherLabel: (count: number) => string,
  maxNodes: number = DEPENDENCY_MAX_NODES
): DependencyGraphModel {
  let selfEdges = 0
  const raw: MatrixEdge[] = []
  for (const edge of graph?.edges ?? []) {
    if (edge.kind !== 'imports' || !Number.isFinite(edge.count) || edge.count <= 0) {
      continue
    }
    if (edge.from === edge.to) {
      selfEdges++
      continue
    }
    raw.push({ from: edge.from, to: edge.to, weight: edge.count })
  }
  const ids = new Set<string>((graph?.nodes ?? []).map((n) => n.id))
  const degree = new Map<string, number>()
  for (const edge of raw) {
    ids.add(edge.from)
    ids.add(edge.to)
    degree.set(edge.from, (degree.get(edge.from) ?? 0) + edge.weight)
    degree.set(edge.to, (degree.get(edge.to) ?? 0) + edge.weight)
  }
  const all = [...ids].sort(
    (a, b) => (degree.get(b) ?? 0) - (degree.get(a) ?? 0) || (a < b ? -1 : 1)
  )
  const fold = all.length > maxNodes
  const kept = fold ? all.slice(0, maxNodes - 1) : all
  const keptSet = new Set(kept)
  const target = (id: string): string => (keptSet.has(id) ? id : DEPENDENCY_OTHER_ID)
  const merged = new Map<string, MatrixEdge>()
  for (const edge of raw) {
    const from = target(edge.from)
    const to = target(edge.to)
    if (from === to) {
      continue
    }
    const key = `${from}\u0000${to}`
    const prev = merged.get(key)
    merged.set(key, { from, to, weight: (prev?.weight ?? 0) + edge.weight })
  }
  const nodes: MatrixNode[] = kept.map((id) => ({ id, label: shortLabel(id) }))
  const otherCount = all.length - kept.length
  if (otherCount > 0) {
    nodes.push({ id: DEPENDENCY_OTHER_ID, label: otherLabel(otherCount) })
  }
  return {
    nodes,
    edges: [...merged.values()],
    total: all.length,
    shown: kept.length,
    otherCount,
    selfEdges
  }
}
