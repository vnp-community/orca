/**
 * code-intel-graph-types.ts — CONTRACT-codeintel-ui-api §4.2 structure, impact and symbol graphs
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

import type { SymbolRef } from './code-intel-index-types'

// ---------------------------------------------------------------------------
// §4.2  Structure and impact graphs
// ---------------------------------------------------------------------------

export type ModuleNode = {
  id: string
  kind: 'file' | 'folder'
  language?: string
  symbolCount: number
  loc?: number
  cluster?: string
  area?: string
}
export type ModuleEdge = { from: string; to: string; kind: 'imports' | 'contains'; count: number }
export type ModuleGraph = { nodes: ModuleNode[]; edges: ModuleEdge[] }

export type SymbolEdge = {
  fromKey: string
  toKey: string
  kind: string
  confidence: number
  reason?: string
  line?: number
  sources: ('gitnexus' | 'codegraph')[]
}
export type SymbolGraph = {
  center: SymbolRef
  nodes: (SymbolRef & { cluster?: string })[]
  edges: SymbolEdge[]
  depth: number
}

export type FlowSummary = {
  id: string
  label: string
  processType: string
  stepCount: number
  communities: string[]
  entry: SymbolRef | null
  terminal: SymbolRef | null
}

export type RiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'UNKNOWN'

export type ImpactSymbol = { symbol: SymbolRef; via: string; confidence?: number; direct: boolean }

/** v7 limitation: the graph has NO edges. */
export type ImpactGraph = {
  target: SymbolRef
  direction: 'upstream' | 'downstream'
  risk: RiskLevel
  impactedCount: number
  levels: { depth: number; symbols: ImpactSymbol[] }[]
  affectedFlows: { flowId: string; label: string; stepCount: number; changedStep?: number }[]
  affectedClusters: { id: string | null; label: string; hits: number; impact: string }[]
  testsCovering: SymbolRef[]
}

export type RouteMap = {
  routes: {
    id: string
    path: string
    method: string
    filePath: string
    side: 'server' | 'client'
    handler: SymbolRef | null
    middleware?: string[]
    responseKeys?: string[]
    errorKeys?: string[]
  }[]
  edges: { route: string; handler: string; kind: 'handles_route' | 'fetches' }[]
}

export type SymbolNeighbor = { uid?: string; key?: string; name: string; filePath: string; line?: number }

export type SymbolDetail = {
  symbol: SymbolRef
  incoming: Record<string, SymbolNeighbor[]>
  outgoing: Record<string, SymbolNeighbor[]>
  flows: { id: string; label: string; stepCount: number; step: number }[]
  source: { text: string; startLine: number; endLine: number; truncated: boolean } | null
  sourceOmitted: 'gitignored' | 'binary' | 'sensitive_path' | 'not_requested' | null
}
