/**
 * code-intel-overlay-types.ts — CONTRACT-codeintel-ui-api §4.3 ChangeOverlay (plus Owner from §4.5)
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

import type { WithUnknown } from './code-intel-enum-fallback'
import type { SymbolRef } from './code-intel-index-types'
import type { FlowSummary, RiskLevel } from './code-intel-graph-types'

// ---------------------------------------------------------------------------
// §4.5 Owner (declared first: ChangedFile references it)
// ---------------------------------------------------------------------------

export type Owner = { source: 'codeowners' | 'history'; names: string[]; share?: number }

// ---------------------------------------------------------------------------
// §4.3  ChangeOverlay
// ---------------------------------------------------------------------------

export type ChangedFile = {
  path: string
  oldPath?: string
  status: WithUnknown<'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'untracked'>
  added?: number
  removed?: number
  area: string
  componentId?: string
  isTest: boolean
  isGenerated: boolean
  isDoc: boolean
  mappingConfidence: WithUnknown<'exact' | 'approx' | 'none'>
  owner?: Owner
}

export type ChangedSymbol = {
  symbol: SymbolRef
  changeKind: WithUnknown<'added' | 'modified' | 'renamed'>
  linesChanged: number
  directCallers?: number
  flows: number
  tested: WithUnknown<'yes' | 'no'>
}

export type ReasonCode =
  | 'contract'
  | 'dependency-of'
  | 'leaf'
  | 'cycle'
  | 'no-edges'
  | 'test'
  | 'doc'
  | 'generated'
  | 'overflow'

export type ReadingStep = {
  stepKey: string
  n: number
  file: string
  symbols: SymbolRef[]
  hunks: { startLine: number; endLine: number }[]
  reason: WithUnknown<ReasonCode>
  reasonParams?: Record<string, string>
  dependsOn: string[]
  tests: SymbolRef[]
  cycleGroup?: string
  layer: string
}

export type ComponentGroup = {
  componentId: string
  containerId: string
  label: string
  files: number
  symbols: number
  added: number
  removed: number
  riskPoints: number
  stepKeys: string[]
}

export type RiskReason = {
  code: string
  points: number
  messageKey: string
  params?: Record<string, string>
  evidence: (SymbolRef | string)[]
}

export type RiskAssessment = {
  level: Exclude<RiskLevel, 'UNKNOWN'>
  score: number
  incomplete: boolean
  confidence: WithUnknown<'high' | 'medium' | 'low'>
  reasons: RiskReason[]
  modelVersion: '1'
  toolRisk?: string
}

export type IndexFreshness = {
  state: WithUnknown<'fresh' | 'behind' | 'dirty' | 'missing'>
  indexedCommit?: string
  headOid?: string
  commitsBehind?: number
  dirtyFiles: number
  unindexedFiles: string[]
  generatedAt: string
}

export type TouchedTable = {
  table: string
  service: string
  via: 'migration' | 'code'
  migrations: string[]
  accessors: SymbolRef[]
}

export type TouchedContract = {
  kind: WithUnknown<'proto-rpc' | 'ws-channel' | 'route' | 'file'>
  name: string
  change: WithUnknown<'added' | 'modified' | 'removed'>
  files: string[]
  compatibility: WithUnknown<'breaking' | 'risky' | 'compatible'>
  breaking?: boolean
}

export type ViolationRef = {
  findingKey: string
  rule: string
  severity: WithUnknown<'error' | 'warning' | 'info'>
  file: string
  status: WithUnknown<'touched' | 'introduced'>
}

export type OverlayScope = {
  baseRef: string
  baseOid?: string
  mergeBase?: string
  headOid?: string
  mode: 'worktree' | 'committed'
  includesUncommitted: boolean
}

/** detail:'summary' returns only scope, limits.totalCounts, risk, components, indexFreshness. */
export type ChangeOverlay = {
  scope: OverlayScope
  emptyReason?: 'unborn-head'
  changedFiles: ChangedFile[]
  changedSymbols: ChangedSymbol[]
  affectedFlows: FlowSummary[]
  affectedClusters: { id: string; label: string }[]
  touchedTables: TouchedTable[]
  touchedContracts: TouchedContract[]
  uncoveredSymbols: SymbolRef[]
  violations: ViolationRef[]
  readingOrder: ReadingStep[]
  components: ComponentGroup[]
  risk: RiskAssessment
  indexFreshness: IndexFreshness
  limits: {
    truncated: { files: boolean; symbols: boolean; flows: boolean; steps: boolean; impact: boolean }
    totalCounts: Record<string, number>
  }
}
