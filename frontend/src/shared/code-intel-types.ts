/**
 * code-intel-types.ts — FE-CV-TASK-050-01
 *
 * Mirror of UI-API §4.1-§4.6 for code intelligence features.
 * Field names MUST NOT be renamed — they are protocol-level.
 *
 * Enum values outside the known set must fall through to 'unknown' (U4).
 * Parser is in 050-03 (code-intel-parsers.ts).
 *
 * HOA/lowercase note (PQ-32):
 *   risk.level, ImpactGraph.risk, IndexStatus.overall → uppercase strings ('HIGH', etc.)
 *   All other enum values → lowercase
 *
 * @module shared/code-intel-types
 */

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

/** Helper to add 'unknown' to an enum union. Parsers use this to widen valid enums. */
export type WithUnknown<T extends string> = T | 'unknown'

// ---------------------------------------------------------------------------
// §4.1  Worktree selector
// ---------------------------------------------------------------------------

export type WorktreeSel = {
  worktreeId: string
  environmentId?: string | null
}

// ---------------------------------------------------------------------------
// §4.2  Envelope
// ---------------------------------------------------------------------------

export type CodeIntelEnvelope<T> = {
  worktreeId: string
  view: string
  sources: string[]
  headCommit: string | null
  stale: boolean
  truncated: boolean
  totalCount: number
  etag: string | null
  notModified?: boolean
  data?: T
}

// ---------------------------------------------------------------------------
// §4.3  Index status
// ---------------------------------------------------------------------------

export type IndexOverall = 'READY' | 'INDEXING' | 'PARTIAL' | 'ERROR' | 'UNKNOWN'

export type IndexStatus = {
  worktreeId: string
  overall: IndexOverall
  /** ISO-8601 timestamp or null */
  lastIndexedAt: string | null
  fileCoverage: number
  linesIndexed: number
  running: boolean
  percent: number | null
  error: string | null
}

// ---------------------------------------------------------------------------
// §4.4  Symbol kinds (exactly 13)
// ---------------------------------------------------------------------------

export type SymbolKind =
  | 'file'
  | 'namespace'
  | 'module'
  | 'class'
  | 'interface'
  | 'method'
  | 'function'
  | 'variable'
  | 'constant'
  | 'property'
  | 'enum_member'
  | 'type_alias'
  | 'constructor'

// ---------------------------------------------------------------------------
// §4.5  Review types
// ---------------------------------------------------------------------------

export type ChangeOverlay = {
  path: string
  changeType: WithUnknown<'added' | 'modified' | 'deleted' | 'renamed' | 'untracked'>
  oldPath?: string | null
}

export type RiskLevel = 'HIGH' | 'MEDIUM' | 'LOW'

export type ImpactGraph = {
  symbolId: string
  name: string
  kind: WithUnknown<SymbolKind>
  path: string
  /** Uppercase risk string per PQ-32 */
  risk: RiskLevel
  callers: ImpactNode[]
  callees: ImpactNode[]
}

export type ImpactNode = {
  symbolId: string
  name: string
  kind: WithUnknown<SymbolKind>
  path: string
}

export type ReviewComment = {
  id: string
  path: string
  line: number | null
  body: string
  authorId: string | null
  createdAt: string
  resolved: boolean
}

export type ReviewState = {
  id: string
  worktreeId: string
  headCommit: string | null
  overlay: ChangeOverlay[]
  impactGraph: ImpactGraph[]
  comments: ReviewComment[]
  checklist: ReviewChecklistItem[]
  overallRisk: WithUnknown<RiskLevel>
  approved: boolean
  approvedAt: string | null
  approvedBy: string | null
}

export type ReviewChecklistItem = {
  id: string
  text: string
  checked: boolean
  automated: boolean
}

// ---------------------------------------------------------------------------
// §4.5  ContractDiff (CR-059 adds behaviour; declared here)
// ---------------------------------------------------------------------------

export type ContractDiff = {
  breakingChanges: ContractBreakingChange[]
  addedEndpoints: string[]
  removedEndpoints: string[]
  modifiedSchemas: ContractSchemaChange[]
}

export type ContractBreakingChange = {
  path: string
  kind: WithUnknown<'param_removed' | 'param_type_changed' | 'response_removed' | 'response_type_changed'>
  description: string
}

export type ContractSchemaChange = {
  schemaName: string
  diff: string
}

// ---------------------------------------------------------------------------
// §4.6  Finding
// ---------------------------------------------------------------------------

export type FindingSeverity = WithUnknown<'critical' | 'high' | 'medium' | 'low' | 'info'>

export type Finding = {
  id: string
  ruleId: string
  ruleName: string
  severity: FindingSeverity
  path: string
  startLine: number
  endLine: number
  startColumn: number | null
  endColumn: number | null
  message: string
  snippet: string | null
  category: WithUnknown<'security' | 'quality' | 'style' | 'performance' | 'correctness'>
  waived: boolean
  waivedAt: string | null
  waivedBy: string | null
  waivedReason: string | null
  autoFixAvailable: boolean
}

// ---------------------------------------------------------------------------
// §5  Push events
// ---------------------------------------------------------------------------

export type PushBase = {
  event: string
}

export type PushChanged = PushBase & {
  event: 'changed'
  worktreeId: string
  reason: WithUnknown<'commit' | 'file_save' | 'branch_switch' | 'reindex' | 'manual'>
  resync: boolean
}

export type PushReindexProgress = PushBase & {
  event: 'reindexProgress'
  worktreeId: string
  /** null means indeterminate */
  percent: number | null
  running: boolean
}

export type PushQualityProgress = PushBase & {
  event: 'qualityProgress'
  worktreeId: string
  runId: string
  percent: number | null
  phase: WithUnknown<'collect' | 'analyze' | 'report'>
}

export type PushQualityFinished = PushBase & {
  event: 'qualityFinished'
  worktreeId: string
  runId: string
  success: boolean
  error: string | null
}

export type PushGateChanged = PushBase & {
  event: 'gateChanged'
  worktreeId: string
  gate: WithUnknown<'pass' | 'warn' | 'fail' | 'unknown'>
}

export type CodeIntelPushEvent =
  | PushChanged
  | PushReindexProgress
  | PushQualityProgress
  | PushQualityFinished
  | PushGateChanged
