/**
 * Graph lens registry — FE-REQ-TASK-032-03
 *
 * Pure configuration: adding a lens must not require touching GraphCanvas.
 *
 * @module components/graph/graph-lens-registry
 */

import { Activity, Boxes, Database, FileCode, GitBranch, ListTree, Radar } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { GRAPH_RISK_ORDER } from '../../../../shared/graph-types'
import type { GraphLens, GraphPayload } from '../../../../shared/graph-types'

export type GraphLensConfig = {
  id: GraphLens
  labelKey: string
  labelFallback: string
  Icon: LucideIcon
  nodeKinds: string[]
  edgeKinds: string[]
  legendKeys: string[]
  source: 'client' | 'backend'
}

const P = 'auto.components.graph.lens.'

export const GRAPH_LENSES: readonly GraphLensConfig[] = [
  { id: 'flow', labelKey: `${P}flow`, labelFallback: 'Flow', Icon: GitBranch, nodeKinds: ['stage'], edgeKinds: ['next'], legendKeys: [], source: 'client' },
  { id: 'architecture', labelKey: `${P}architecture`, labelFallback: 'Architecture', Icon: Boxes, nodeKinds: ['service', 'module', 'package'], edgeKinds: ['depends_on', 'calls'], legendKeys: [], source: 'backend' },
  { id: 'contract', labelKey: `${P}contract`, labelFallback: 'Contracts', Icon: FileCode, nodeKinds: ['rpc', 'event', 'schema'], edgeKinds: ['uses', 'produces'], legendKeys: [], source: 'backend' },
  { id: 'data', labelKey: `${P}data`, labelFallback: 'Data', Icon: Database, nodeKinds: ['table', 'column', 'store'], edgeKinds: ['reads', 'writes'], legendKeys: [], source: 'backend' },
  { id: 'impact', labelKey: `${P}impact`, labelFallback: 'Impact', Icon: Radar, nodeKinds: ['file', 'symbol', 'service'], edgeKinds: ['affects'], legendKeys: [], source: 'backend' },
  { id: 'plan', labelKey: `${P}plan`, labelFallback: 'Plan', Icon: ListTree, nodeKinds: ['phase', 'task'], edgeKinds: ['contains', 'depends_on'], legendKeys: [], source: 'client' },
  { id: 'execution', labelKey: `${P}execution`, labelFallback: 'Execution', Icon: Activity, nodeKinds: ['phase', 'task'], edgeKinds: ['depends_on'], legendKeys: [], source: 'client' }
]

export function getLensConfig(id: GraphLens): GraphLensConfig {
  return GRAPH_LENSES.find((l) => l.id === id) ?? GRAPH_LENSES[0]
}

function maxRank(payload: GraphPayload | undefined): number {
  if (!payload) {return 0}
  let max = 0
  for (const n of payload.nodes) {max = Math.max(max, GRAPH_RISK_ORDER[n.risk])}
  return max
}

/** Backend lens with the highest node risk wins (unknown ignored); ties follow registry order; else `flow`. */
export function pickDefaultLens(summaries: Partial<Record<GraphLens, GraphPayload>>): GraphLens {
  let best: GraphLens = 'flow'
  let bestRank = 0
  for (const lens of GRAPH_LENSES) {
    if (lens.source !== 'backend') {continue}
    const rank = maxRank(summaries[lens.id])
    if (rank > bestRank) {
      best = lens.id
      bestRank = rank
    }
  }
  return best
}
