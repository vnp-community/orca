/**
 * code-intel-quality-visualization-types.ts — FE-CV-TASK-087-15
 *
 * Trend, coverage, channel results and §5 push wire shapes of the quality contract (§4.7).
 * Re-exported from code-intel-quality-types.ts. Coverage ratios are 0..1 (contract D5).
 *
 * @module shared/code-intel-quality-visualization-types
 */

import type { WithUnknown } from './code-intel-enum-fallback'
import type {
  GateResult,
  QualityFinding,
  QualityRun,
  QualityRunSummary
} from './code-intel-quality-types'

// ---------------------------------------------------------------------------
// Trend and coverage
// ---------------------------------------------------------------------------

export type QualityTrendCounts = {
  error: number
  warning: number
  info: number
  byCategory: Record<string, number>
}

/** A missing key means "no number", never zero (PQ-33). */
export type QualityTrendMetrics = {
  diffCoverage?: number
  newLayerViolations?: number
  newCycles?: number
  testsFailed?: number
}

export type QualityTrendPoint = {
  turnKey: string
  headCommit: string
  baseCommit: string
  profileRef: string
  verdict: GateResult
  counts: QualityTrendCounts
  metrics: QualityTrendMetrics
  runIds: string[]
  indexCommit: string
  source: WithUnknown<'local' | 'ci'>
  createdAt: string
}

export type CoverageFile = {
  path: string
  stmts: number
  covered: number
  /** Ratio 0..1 (D5). */
  pct: number
  uncoveredRanges?: [number, number][]
}

export type CoverageDiff = {
  changedExecutable: number
  covered: number
  uncovered: number
  /** Ratio 0..1 (D5) or null when there is nothing to measure. */
  diffCoverage: number | null
  reason?: string
  partial: boolean
  excludedFiles: { path: string; reason: string }[]
}

export type CoverageReport = {
  source: WithUnknown<'measured' | 'estimated'>
  language: WithUnknown<'go' | 'ts' | 'mixed'>
  mode?: string
  headCommit: string
  baseCommit: string
  dirty: boolean
  totals: {
    stmts?: number
    covered?: number
    pct?: number
    changedSymbolsTested?: number
    changedSymbolsUntested?: number
    changedSymbolsUnknown?: number
  }
  diff: CoverageDiff | null
  files: CoverageFile[]
  truncated: boolean
  totalCount: number
  toolVersions: Record<string, string>
  estimatedNote?: string
}

// ---------------------------------------------------------------------------
// Channel results (the quality channels return flat objects, no CodeIntelEnvelope)
// ---------------------------------------------------------------------------

export type QualityRunsResponse = { runs: QualityRun[]; nextPageToken?: string }

export type QualityFindingsResponse = {
  findings: QualityFinding[]
  totalCount: number
  truncated: boolean
  outsideScopeCount: number
  nextPageToken?: string
}

export type QualityTrendResponse = {
  points: QualityTrendPoint[]
  truncated: boolean
  totalCount: number
}

export type QualityCoverageResponse = { report: CoverageReport | null; reason?: string }

// ---------------------------------------------------------------------------
// §5 push payloads (wire shapes; the renderer's normalized events live in code-intel-push-types)
// ---------------------------------------------------------------------------

export type QualityPushFinishedStatus = 'succeeded' | 'failed' | 'cancelled' | 'interrupted'

export type PushQualityProgressWire = {
  event: 'quality.progress'
  projectId: string
  worktreeId: string
  occurredAt: string
  runId: string
  stage: string
  stepIndex?: number
  stepCount?: number
  /** null = unknown */
  percent: number | null
  message: string
}

export type PushQualityFinishedWire = {
  event: 'quality.finished'
  projectId: string
  worktreeId: string
  occurredAt: string
  runId: string
  status: QualityPushFinishedStatus
  summary: QualityRunSummary
  headCommit: string
}

export type PushGateChangedWire = {
  event: 'quality.gateChanged'
  projectId: string
  worktreeId: string
  occurredAt: string
  headCommit: string
  previousVerdict: GateResult | null
  verdict: GateResult
  profile: string
}
