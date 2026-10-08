/**
 * quality-scorecard-model.ts — FE-CV-TASK-087-05
 *
 * Pure reductions of the runs behind a gate for the scorecard: severity totals, flattened
 * steps, warning flags and the sources to name in the provenance line. A step that failed,
 * timed out or was not ready is surfaced as such; it is never folded into "0 findings".
 *
 * @module components/review-map/quality/quality-scorecard-model
 */

import type {
  CiComparison,
  QualityRun,
  QualityStep
} from '../../../../../shared/code-intel-quality-types'

export type ScorecardStepRow = {
  key: string
  runId: string
  step: QualityStep
  /** The step produced no usable result (failed, timeout, env not ready). */
  incomplete: boolean
}

export type ScorecardRunSummary = {
  ran: boolean
  counts: { error: number; warning: number; info: number }
  outsideScope: number
  steps: ScorecardStepRow[]
  dirty: boolean
  changedDuringRun: boolean
  scopeWidened: boolean
  truncated: boolean
  sources: {
    runId: string
    source: QualityRun['source']
    finishedAt: string | null
    headCommit: string
    indexCommit: string
  }[]
}

const INCOMPLETE_STEP_STATUSES = new Set(['failed', 'timeout', 'env_not_ready', 'cancelled'])

export function isIncompleteStep(step: QualityStep): boolean {
  return INCOMPLETE_STEP_STATUSES.has(step.status) || step.status === 'unknown'
}

export function summarizeRuns(runs: readonly QualityRun[]): ScorecardRunSummary {
  const counts = { error: 0, warning: 0, info: 0 }
  let outsideScope = 0
  const steps: ScorecardStepRow[] = []
  for (const run of runs) {
    counts.error += run.summary.error
    counts.warning += run.summary.warning
    counts.info += run.summary.info
    outsideScope += run.summary.outsideScope
    run.steps.forEach((step, i) => {
      steps.push({
        key: `${run.id}:${step.id || i}`,
        runId: run.id,
        step,
        incomplete: isIncompleteStep(step)
      })
    })
  }
  return {
    ran: runs.length > 0,
    counts,
    outsideScope,
    steps,
    dirty: runs.some((r) => r.dirty === true),
    changedDuringRun: runs.some((r) => r.workTreeChangedDuringRun),
    scopeWidened: runs.some((r) => r.scopeWidened),
    truncated: runs.some((r) => r.summary.truncated),
    sources: runs.map((r) => ({
      runId: r.id,
      source: r.source,
      finishedAt: r.finishedAt,
      headCommit: r.headCommit,
      indexCommit: r.indexCommit
    }))
  }
}

export function shortCommit(commit: string | null | undefined): string | null {
  return commit ? commit.slice(0, 7) : null
}

/** Reason codes become catalog key segments only when they are plain identifiers. */
export function reasonCodeKeySegment(code: string | undefined): string | null {
  return code && /^[A-Za-z0-9_]+$/.test(code) ? code : null
}

/** `local_pass_ci_fail` must never read as an overall pass. */
export function comparisonIsDisagreement(comparison: CiComparison): boolean {
  return (
    comparison.relation === 'local_pass_ci_fail' || comparison.relation === 'local_fail_ci_pass'
  )
}

/** A reason can open the findings list only when it names a category or a tool to filter by. */
export function reasonFilter(reason: {
  category?: string
  tool?: string
}): { category?: string; tool?: string } | null {
  if (reason.category) {
    return { category: reason.category }
  }
  if (reason.tool) {
    return { tool: reason.tool }
  }
  return null
}
