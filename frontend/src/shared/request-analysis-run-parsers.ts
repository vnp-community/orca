/**
 * Parser for the AI runs behind a Solution (`solution.list` → `runs`).
 *
 * Kept apart from request-wire-parsers (at its line cap). Never throws; unknown input
 * becomes a run with status 'unknown'.
 *
 * @module shared/request-analysis-run-parsers
 */

import type { AnalysisRun, AnalysisRunStatus, SolutionKind } from './request-types'

const RUN_STATUSES: readonly AnalysisRunStatus[] = ['running', 'succeeded', 'failed']
const SOLUTION_KINDS: readonly SolutionKind[] = ['solution', 'diagnosis', 'findings', 'answer']

function optString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined
}

export function parseAnalysisRun(raw: unknown): AnalysisRun {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const status = RUN_STATUSES.find((s) => s === r.status) ?? 'unknown'
  const kind = SOLUTION_KINDS.find((k) => k === r.kind) ?? 'unknown'
  return {
    id: typeof r.id === 'string' ? r.id : '',
    kind,
    status,
    errorCode: optString(r.errorCode),
    errorMessage: optString(r.errorMessage),
    startedAt: optString(r.startedAt),
    finishedAt: optString(r.finishedAt)
  }
}

/** The newest run decides the state: a failure only counts while nothing newer has started. */
export function latestAnalysisRun(runs: readonly AnalysisRun[]): AnalysisRun | null {
  let latest: AnalysisRun | null = null
  for (const run of runs) {
    if (!latest || (run.startedAt ?? '') >= (latest.startedAt ?? '')) {
      latest = run
    }
  }
  return latest
}
