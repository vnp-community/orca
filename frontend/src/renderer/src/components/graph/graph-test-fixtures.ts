import { emptyGraphPayload } from '../../../../shared/graph-wire-parsers'
import type { GraphEdge, GraphLens, GraphNode, GraphPayload, GraphRisk } from '../../../../shared/graph-types'

export function gNode(id: string, over: Partial<GraphNode> = {}): GraphNode {
  return { id, kind: 'module', label: id, group: null, risk: 'low' as GraphRisk, status: null, ...over }
}

export function gEdge(from: string, to: string, over: Partial<GraphEdge> = {}): GraphEdge {
  return { from, to, kind: 'calls', change: 'unchanged', ...over }
}

export function gPayload(nodes: GraphNode[], edges: GraphEdge[] = [], lens: GraphLens = 'impact'): GraphPayload {
  return { ...emptyGraphPayload(lens), nodes, edges, totalNodes: nodes.length }
}
