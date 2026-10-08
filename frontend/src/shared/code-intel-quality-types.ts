/**
 * code-intel-quality-types.ts — FE-CV-TASK-087-01
 *
 * Mirror of CONTRACT-codeintel-ui-api §4.7 (quality) and the §5 quality push events.
 * Field names are protocol-level: do not rename. Enum values outside the known set fall
 * through to 'unknown' in the parsers (U4); `unknown` is never rendered as a pass.
 *
 * Resolved by contract D5: coverage ratios are 0..1; QualityFinding.column/endColumn are
 * 1-based (0 = none); GateReason observed/threshold are pre-formatted strings;
 * RunnableProfile.id equals the part of QualityGate.profile before '@'.
 *
 * @module shared/code-intel-quality-types
 */

import type { WithUnknown } from './code-intel-enum-fallback'
import type { IndexBasis } from './code-intel-index-types'

// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

export type QualitySeverity = WithUnknown<'error' | 'warning' | 'info'>

export type QualityCategory = WithUnknown<
  | 'lint'
  | 'typecheck'
  | 'test'
  | 'coverage'
  | 'complexity'
  | 'security'
  | 'dependency'
  | 'convention'
  | 'architecture'
  | 'ai'
>

export type QualityFindingWaiver = { by: string; reason: string; expiresAt: string }

export type QualityFinding = {
  fingerprint: string
  fpVersion: number
  ruleId: string
  severity: QualitySeverity
  category: QualityCategory
  file: string
  /** 1-based; <= 0 means a file-level finding. */
  line: number
  endLine: number
  /** 1-based (LSP style); 0 = no column. */
  column: number
  endColumn: number
  message: string
  tool: string
  toolVersion: string
  fixHint?: string
  stepId: string
  inScope: boolean
  waiver?: QualityFindingWaiver
}

// ---------------------------------------------------------------------------
// Run and steps
// ---------------------------------------------------------------------------

export type QualityStepStatus = WithUnknown<
  'passed' | 'findings' | 'failed' | 'timeout' | 'cancelled' | 'skipped' | 'env_not_ready'
>

export type QualityStepFailureKind = WithUnknown<
  '' | 'format_drift' | 'output_too_large' | 'exit_unexpected' | 'parser_error' | 'env'
>

export type QualityStep = {
  id: string
  profileId: string
  status: QualityStepStatus
  failureKind: QualityStepFailureKind
  envReason?: string
  exitCode: number
  durationMs: number
  tool: string
  toolVersion: string
  errorCount: number
  warningCount: number
  infoCount: number
  totalCount: number
  truncated: boolean
  outsideScopeCount: number
}

export type QualityRunScope = WithUnknown<'worktree' | 'changed' | 'commitRange'>

export type QualityRunStatus = WithUnknown<
  'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'interrupted'
>

export type QualityRunSummary = {
  error: number
  warning: number
  info: number
  stepsTotal: number
  stepsWithFindings: number
  stepsFailed: number
  stepsEnvNotReady: number
  outsideScope: number
  truncated: boolean
}

export type QualityRunCi = {
  provider: string
  headSha: string
  url?: string
  fetchedAt: string
  staleAfter?: string
}

export type QualityRun = {
  id: string
  worktreeId: string
  headCommit: string
  indexCommit: string
  indexBasis: IndexBasis[]
  scope: QualityRunScope
  baseCommit?: string
  profile: string
  status: QualityRunStatus
  source: WithUnknown<'local' | 'ci'>
  startedAt: string | null
  finishedAt: string | null
  summary: QualityRunSummary
  steps: QualityStep[]
  errorCode?: string
  treeFingerprint?: string
  dirty?: boolean
  workTreeChangedDuringRun: boolean
  scopeWidened: boolean
  ci?: QualityRunCi
}

// ---------------------------------------------------------------------------
// Gate, waiver, CI comparison
// ---------------------------------------------------------------------------

export type GateResult = WithUnknown<'pass' | 'warn' | 'fail'>

export type QualityGateReason = {
  check: string
  /** Pre-formatted by the backend (D5); rendered verbatim as plain text. */
  observed: string
  threshold: string
  result: GateResult
  code?: string
  params?: Record<string, string>
  runId?: string
  waivedCount?: number
  category?: string
  tool?: string
}

export type QualityGateBasedOn = {
  runIds: string[]
  indexCommit: string
  stale: boolean
  headCommit?: string
  baseCommit?: string
  evaluatedAt?: string
  profileVersion?: number
}

export type QualityGate = {
  verdict: GateResult
  reasons: QualityGateReason[]
  mode: WithUnknown<'inform' | 'block'>
  /** "<name>@<scope>/v<version>" */
  profile: string
  basedOn: QualityGateBasedOn
}

export type QualityWaiverSubjectKind = WithUnknown<'finding' | 'structure_finding' | 'check'>

export type QualityWaiver = {
  id: string
  subjectKind: QualityWaiverSubjectKind
  subjectKey: string
  scope: string
  reason: string
  createdBy: string
  createdAt: string
  expiresAt: string
  revokedAt?: string
}

export type CiRelation = WithUnknown<
  | 'agree_pass'
  | 'agree_fail'
  | 'local_pass_ci_fail'
  | 'local_fail_ci_pass'
  | 'local_only'
  | 'ci_only'
  | 'ci_pending'
  | 'sha_mismatch'
  | 'not_comparable'
>

export type CiComparison = {
  profile: string
  headCommit: string
  local: { runId?: string; status?: string; finishedAt?: string; dirty?: boolean }
  ci: { runId?: string; status?: string; url?: string; fetchedAt?: string; sha?: string }
  relation: CiRelation
  reasonsHint?: string[]
}

/** Result of `quality.gate`. */
export type QualityGateResponse = {
  gate: QualityGate
  waivers: QualityWaiver[]
  evaluatedAt: string
  profileDefinitionDigest: string
  comparison: CiComparison[]
}

// ---------------------------------------------------------------------------
// Profiles
// ---------------------------------------------------------------------------

export type MissingCheck = { check: string; reason: string; hint?: string }

export type RunnableProfile = {
  /** Profile name (e.g. "go-test"); equals QualityGate.profile before '@'. Not a UUID. */
  id: string
  title: string
  kind: string
  ready: boolean
  heavy: boolean
  scopes: string[]
  missing: MissingCheck[]
  suite?: string[]
}

export type QualityProfileDefinition = {
  schemaVersion: number
  checks: {
    id: string
    profile: string
    category: string
    required: boolean
    maxErrors?: number
    maxWarnings?: number
    maxFailed?: number
  }[]
  findings: { countScope: WithUnknown<'changedFiles' | 'all'>; blockingSeverities: string[]; warnBudget: number }
  coverage: { required: boolean; diffCoverageWarnBelow: number | null; diffCoverageFailBelow: number | null }
  structure: {
    newLayerViolationErrorFails: boolean
    newLayerViolationWarningWarns: boolean
    newCyclesWarn: boolean
  }
  freshness: { indexMustMatchHead: boolean }
}

export type QualityProfile = {
  name: string
  mode: WithUnknown<'inform' | 'block'>
  definition: QualityProfileDefinition
  version: number
}

export type QualityProfileResponse = {
  profile: QualityProfile
  origin: WithUnknown<'repo' | 'tenant' | 'builtin'>
  version: number
  runnableProfiles: RunnableProfile[]
}

export type * from './code-intel-quality-visualization-types'
