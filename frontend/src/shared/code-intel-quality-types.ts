/**
 * code-intel-quality-types.ts — FE-CV-TASK-087-01
 *
 * Mirror of UI-API §4.7 quality-specific types.
 * Field names MUST NOT be renamed (protocol-level contract).
 *
 * Enum values outside known set → 'unknown' (U4, enforced in parsers).
 *
 * @module shared/code-intel-quality-types
 */

import type { WithUnknown } from './code-intel-types'

// ---------------------------------------------------------------------------
// §4.7.1 Gate
// ---------------------------------------------------------------------------

export type GateResult = WithUnknown<'pass' | 'warn' | 'fail'>

export type QualityGateReason = {
  check: string
  observed: number
  threshold: number
  result: GateResult
  code?: string
  params?: Record<string, unknown>
}

export type QualityGate = {
  result: GateResult
  reasons: QualityGateReason[]
  mode: WithUnknown<'block' | 'warn'>
  stale: boolean
  unavailable: boolean
  runId: string | null
}

// ---------------------------------------------------------------------------
// §4.7.2 Quality run
// ---------------------------------------------------------------------------

export type QualityRunStatus = WithUnknown<
  'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'interrupted'
>

export type QualityStep = {
  id: string
  name: string
  status: QualityRunStatus
  startedAt: string | null
  completedAt: string | null
  error: string | null
  percent: number | null
}

export type QualityRun = {
  id: string
  worktreeId: string
  profileId: string
  status: QualityRunStatus
  startedAt: string | null
  completedAt: string | null
  error: string | null
  percent: number | null
  steps: QualityStep[]
  triggeredBy: WithUnknown<'user' | 'auto' | 'ci'>
}

// ---------------------------------------------------------------------------
// §4.7.3 Findings
// ---------------------------------------------------------------------------

export type QualityFindingSeverity = WithUnknown<'critical' | 'high' | 'medium' | 'low' | 'info'>

export type QualityFindingCategory = WithUnknown<
  'security' | 'reliability' | 'maintainability' | 'coverage' | 'duplication' | 'other'
>

export type QualityWaiver = {
  id: string
  findingId: string
  reason: string
  waivedBy: string | null
  waivedAt: string
  expiresAt: string | null
}

export type QualityFinding = {
  id: string
  ruleId: string
  ruleName: string
  severity: QualityFindingSeverity
  category: QualityFindingCategory
  path: string
  startLine: number
  endLine: number
  startColumn: number | null
  endColumn: number | null
  message: string
  snippet: string | null
  waived: boolean
  waiver: QualityWaiver | null
  effortMinutes: number
  /** Whether this finding appeared in the last run (new finding) */
  isNew: boolean
  /** Applicable autofix suggestions */
  autoFixAvailable: boolean
}

// ---------------------------------------------------------------------------
// §4.7.4 Profile
// ---------------------------------------------------------------------------

export type RunnableProfile = {
  id: string
  name: string
  description: string | null
  isDefault: boolean
  checks: string[]
  estimatedMinutes: number | null
}

export type QualityProfile = {
  activeProfileId: string | null
  profiles: RunnableProfile[]
}

// ---------------------------------------------------------------------------
// §4.7.5 Trend
// ---------------------------------------------------------------------------

export type QualityTrendPoint = {
  date: string
  score: number | null
  coverage: number | null
  violations: number
  runId: string | null
}

// ---------------------------------------------------------------------------
// §4.7.6 Coverage
// ---------------------------------------------------------------------------

export type CoverageReport = {
  worktreeId: string
  headCommit: string | null
  lineCoverage: number | null
  branchCoverage: number | null
  statementCoverage: number | null
  /** Per-file coverage entries */
  files: CoverageFile[]
  /** Coverage from CI (if available) */
  ciCoverage: number | null
  generatedAt: string | null
}

export type CoverageFile = {
  path: string
  lineCoverage: number | null
  lines: CoverageLine[]
}

export type CoverageLine = {
  line: number
  covered: boolean | null
  /** null for non-branch lines */
  branchCoverage: number | null
}

// ---------------------------------------------------------------------------
// §4.7.7 CI comparison
// ---------------------------------------------------------------------------

export type CiComparison = {
  runId: string | null
  ciRunId: string | null
  verdict: WithUnknown<'better' | 'same' | 'worse'>
  scoreDelta: number | null
  coverageDelta: number | null
  newViolations: number
  resolvedViolations: number
  generatedAt: string | null
}
