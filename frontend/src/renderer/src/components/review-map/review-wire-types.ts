/**
 * review-wire-types.ts
 *
 * Shell-local mirror of CONTRACT-codeintel-ui-api §4.1/§4.3/§4.6 (only the fields the
 * Review shell reads). Why local: shared/code-intel-types.ts (SOL-050) still carries a
 * pre-contract shape for IndexStatus/ChangeOverlay; swap these aliases for the shared
 * types once SOL-050 aligns. Every array is optional-safe via review-shell-data.ts.
 */

export type IndexOverall =
  | 'OFFLINE'
  | 'UNKNOWN'
  | 'NOT_INSTALLED'
  | 'BUILDING'
  | 'MISSING'
  | 'DEGRADED'
  | 'OVERLAY'
  | 'STALE'
  | 'READY'

export type ToolIndexStatusView = {
  tool: string
  available: boolean
  version?: string
  state: string
  indexedCommit?: string
  indexedAt?: string
  headCommit?: string
  stats?: Record<string, number | undefined>
  pendingChanges?: { added: number; modified: number; removed: number } | null
}

export type IndexBasisView = {
  tool: string
  refreshState: string
  indexPolicy: string
  indexedCommit?: string
  headCommit?: string
}

export type IndexStatusView = {
  overall: IndexOverall
  tools: ToolIndexStatusView[]
  scopeMismatch: boolean
  activeJob?: { id: string; stage: string; percent: number | null }
  indexBasis: IndexBasisView[]
  lastStatusAt?: string
  errorCode?: string
}

export type SymbolRefView = {
  key: string
  kind: string
  name: string
  filePath: string
  startLine?: number
  endLine?: number
}

export type ChangedFileView = {
  path: string
  oldPath?: string
  status: 'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'untracked' | 'unknown'
  added?: number
  removed?: number
  componentId?: string
  isTest?: boolean
}

export type ReadingReasonCode =
  | 'contract'
  | 'dependency-of'
  | 'leaf'
  | 'cycle'
  | 'no-edges'
  | 'test'
  | 'doc'
  | 'generated'
  | 'overflow'

export type ReadingStepView = {
  stepKey: string
  n: number
  file: string
  symbols: SymbolRefView[]
  hunks: { startLine: number; endLine: number }[]
  reason: ReadingReasonCode | string
  reasonParams?: Record<string, string>
  dependsOn: string[]
  tests: SymbolRefView[]
  cycleGroup?: string
  layer: string
}

export type ComponentGroupView = {
  componentId: string
  containerId?: string
  label: string
  files: number
  symbols: number
  added: number
  removed: number
  riskPoints: number
  stepKeys: string[]
}

export type RiskReasonView = {
  code: string
  points?: number
  messageKey: string
  params?: Record<string, string>
}

export type RiskAssessmentView = {
  level: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'
  score?: number
  incomplete: boolean
  confidence?: string
  reasons: RiskReasonView[]
}

export type IndexFreshnessView = {
  state: 'fresh' | 'behind' | 'dirty' | 'missing' | string
  indexedCommit?: string
  headOid?: string
  commitsBehind?: number
  dirtyFiles?: number
}

export type OverlayScopeView = {
  baseRef: string
  baseOid?: string
  mergeBase?: string
  headOid?: string
  mode: 'worktree' | 'committed'
  includesUncommitted: boolean
}

export type ViolationRefView = {
  findingKey: string
  rule: string
  severity: string
  file: string
  status: 'touched' | 'introduced' | string
}

export type ChangeOverlayView = {
  scope: OverlayScopeView | null
  emptyReason?: 'unborn-head' | string
  changedFiles: ChangedFileView[]
  changedSymbols: { symbol: SymbolRefView; changeKind: string; tested: string }[]
  affectedFlows: unknown[]
  touchedTables: unknown[]
  touchedContracts: unknown[]
  uncoveredSymbols: SymbolRefView[]
  violations: ViolationRefView[]
  readingOrder: ReadingStepView[]
  components: ComponentGroupView[]
  risk: RiskAssessmentView | null
  indexFreshness: IndexFreshnessView | null
  limits: { truncated: Record<string, boolean>; totalCounts: Record<string, number> }
}

export type ReadingProgressEntry = { state: 'seen' | 'unseen'; at: number }

export type ReadingProgress = {
  version: 1
  entries: Record<string, ReadingProgressEntry>
  lastFocusedKey: string | null
}

/** Whole ReviewState is kept so notes/turnMarkers are re-sent verbatim (SOL-052 open Q1). */
export type ReviewStateView = {
  baseCommit: string
  headCommit: string
  readingProgress: ReadingProgress
  notes?: unknown
  turnMarkers?: unknown
  status?: 'open' | 'reviewed'
  version: number
  [extra: string]: unknown
}

export type ReviewErrorKind =
  | 'no-binding'
  | 'tool-unavailable'
  | 'repo-not-registered'
  | 'path-not-allowed'
  | 'index-missing'
  | 'offline'
  | 'forbidden'
  | 'rate-limited'
  | 'timeout'
  | 'too-large'
  | 'tool-failed'
  | 'conflict'
  | 'reindex-in-progress'
  | 'unsupported'
  | 'disabled'
  | 'unknown'

export type ReviewError = {
  kind: ReviewErrorKind
  message: string
  retryAfterSeconds?: number
  data?: Record<string, unknown> | null
}

export function emptyReadingProgress(): ReadingProgress {
  return { version: 1, entries: {}, lastFocusedKey: null }
}
