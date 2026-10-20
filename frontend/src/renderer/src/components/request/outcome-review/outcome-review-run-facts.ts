/**
 * Outcome Review run facts — FE-OR-T122
 *
 * What the agent's work cost, from the numbers Orca holds. A figure nobody reported is Unknown, never 0.
 *
 * @module components/request/outcome-review/outcome-review-run-facts
 */

import type { OutcomeReviewRun } from '../../../../../shared/outcome-review-types'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'

export type AgentRunFacts = {
  tasksDone: number | null
  tasksTotal: number | null
  tasksFailed: number | null
  reviewRuns: number
  reviewRunsFailed: number
  /** Sum of the figures the agent reported, in USD; null when no run said what it cost. */
  costUsd: number | null
  /** Seconds the review runs took in all, from their start and end times; null when none has ended. */
  seconds: number | null
}

function costOf(run: OutcomeReviewRun): number | null {
  const m = /^\s*([0-9]+(?:\.[0-9]+)?)\s*USD\s*$/.exec(run.costActual ?? '')
  return m ? Number(m[1]) : null
}

function secondsOf(run: OutcomeReviewRun): number | null {
  const start = Date.parse(run.startedAt)
  const end = run.finishedAt ? Date.parse(run.finishedAt) : Number.NaN
  return Number.isNaN(start) || Number.isNaN(end) || end < start ? null : (end - start) / 1000
}

export function agentRunFactsOf(
  runs: OutcomeReviewRun[],
  summary: ExecutionSummary | null
): AgentRunFacts {
  const costs = runs.map(costOf).filter((c): c is number => c !== null)
  const times = runs.map(secondsOf).filter((s): s is number => s !== null)
  return {
    tasksDone: summary ? summary.progress.tasksDone : null,
    tasksTotal: summary ? summary.progress.tasksTotal : null,
    tasksFailed: summary ? summary.progress.tasksFailed : null,
    reviewRuns: runs.length,
    reviewRunsFailed: runs.filter((r) => r.status === 'failed').length,
    costUsd:
      costs.length > 0 ? Math.round(costs.reduce((a, b) => a + b, 0) * 10_000) / 10_000 : null,
    seconds: times.length > 0 ? Math.round(times.reduce((a, b) => a + b, 0)) : null
  }
}
