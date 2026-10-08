/**
 * Text summary of a graph (always shown next to GraphMini).
 *
 * @module components/graph/graph-summary
 */

import type { GraphPayload } from '../../../../shared/graph-types'

export type GraphSummary = {
  nodeCount: number
  byKind: Record<string, number>
  addedEdges: number
  removedEdges: number
  breaking: number
}

export function summarizeGraph(payload: GraphPayload): GraphSummary {
  const byKind: Record<string, number> = {}
  let breaking = 0
  for (const n of payload.nodes) {
    byKind[n.kind] = (byKind[n.kind] ?? 0) + 1
    if (n.status === 'breaking') {breaking += 1}
  }
  return {
    nodeCount: payload.nodes.length,
    byKind,
    addedEdges: payload.edges.filter((e) => e.change === 'added').length,
    removedEdges: payload.edges.filter((e) => e.change === 'removed').length,
    breaking
  }
}

/** Plain sentence from translated fragments; callers pass `translate`-bound labels. */
export function formatGraphSummary(
  s: GraphSummary,
  t: (key: string, fallback: string, opts?: Record<string, unknown>) => string
): string {
  const parts = [t('auto.components.graph.Mini.nodes', '{{count}} nodes', { count: s.nodeCount })]
  if (s.addedEdges > 0) {parts.push(t('auto.components.graph.Mini.addedEdges', '{{count}} edges added', { count: s.addedEdges }))}
  if (s.removedEdges > 0) {parts.push(t('auto.components.graph.Mini.removedEdges', '{{count}} edges removed', { count: s.removedEdges }))}
  if (s.breaking > 0) {parts.push(t('auto.components.graph.Mini.breaking', '{{count}} breaking', { count: s.breaking }))}
  return parts.join(', ')
}
