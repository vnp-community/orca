/**
 * Graph wire types — FE-REQ-TASK-032-03
 *
 * Shape of the `impact.graph` payload and of the client-built lens payloads.
 * Pure types and constants only: shared/ must not import React or the DOM.
 *
 * @module shared/graph-types
 */

export const GRAPH_LENSES_BACKEND = ['architecture', 'contract', 'data', 'impact'] as const
export const GRAPH_LENSES_CLIENT = ['flow', 'plan', 'execution'] as const

export type GraphLens =
  | (typeof GRAPH_LENSES_BACKEND)[number]
  | (typeof GRAPH_LENSES_CLIENT)[number]

export type GraphRisk = 'low' | 'medium' | 'high' | 'critical' | 'unknown'
export type GraphChange = 'added' | 'removed' | 'unchanged'

// Why: `unknown` ranks lowest but is a distinct value; it must never render as `low`.
export const GRAPH_RISK_ORDER: Record<GraphRisk, number> = {
  unknown: 0,
  low: 1,
  medium: 2,
  high: 3,
  critical: 4
}

export type GraphNode = {
  id: string
  kind: string
  label: string
  group: string | null
  risk: GraphRisk
  /** Free-form per lens (e.g. `modified`, `breaking`, task status). */
  status: string | null
  meta?: { findingIds?: string[]; taskId?: string; optionId?: string; drifted?: boolean }
}

export type GraphEdge = {
  from: string
  to: string
  kind: string
  change: GraphChange
}

export type GraphPayload = {
  lens: GraphLens
  nodes: GraphNode[]
  edges: GraphEdge[]
  totalNodes: number
  truncated: boolean
  assessedAt: string | null
  tool: string | null
  stale: boolean
  /** Frontend-computed: edges referencing a node that is not in `nodes`. */
  droppedEdges: number
}

export type GraphSubjectType = 'solution_option' | 'plan' | 'task'
