/**
 * OutcomeAgentRunFacts — FE-OR-T122
 *
 * What the agent's work cost, from the numbers Orca holds: tasks done, and the runs of the review itself with the cost the agent
 * reported. A figure nobody reported is Unknown rather than zero.
 *
 * @module components/request/outcome-review/OutcomeAgentRunFacts
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { unknownText } from './outcome-review-copy'
import { agentRunFactsOf } from './outcome-review-run-facts'
import type { OutcomeReviewRun } from '../../../../../shared/outcome-review-types'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'

const T = 'auto.components.request.outcomeReview.OutcomeAgentRunFacts.'

function durationText(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}s`
  }
  const minutes = Math.floor(seconds / 60)
  return `${minutes}m ${String(seconds % 60).padStart(2, '0')}s`
}

type Props = { runs: OutcomeReviewRun[]; summary: ExecutionSummary | null }

export function OutcomeAgentRunFacts({ runs, summary }: Props): React.JSX.Element {
  const facts = agentRunFactsOf(runs, summary)
  const title = translate(`${T}title`, 'Agent run')
  return (
    <section
      className="flex flex-col gap-2 rounded-md border border-border p-3"
      aria-label={title}
      data-testid="outcome-agent-run-facts"
    >
      <h4 className="text-sm font-medium">{title}</h4>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-1 text-sm sm:grid-cols-4">
        <div>
          <dt className="text-xs text-muted-foreground">{translate(`${T}tasks`, 'Tasks done')}</dt>
          <dd className="tabular-nums" data-testid="outcome-facts-tasks">
            {facts.tasksDone === null || facts.tasksTotal === null
              ? unknownText()
              : translate(`${T}tasksValue`, '{{done}} of {{total}}', {
                  done: facts.tasksDone,
                  total: facts.tasksTotal
                })}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">
            {translate(`${T}reviewRuns`, 'Review runs')}
          </dt>
          <dd className="tabular-nums" data-testid="outcome-facts-runs">
            {translate(`${T}reviewRunsValue`, '{{count}} ({{failed}} failed)', {
              count: facts.reviewRuns,
              failed: facts.reviewRunsFailed
            })}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">
            {translate(`${T}cost`, 'Cost reported')}
          </dt>
          <dd className="tabular-nums" data-testid="outcome-facts-cost">
            {facts.costUsd === null
              ? unknownText()
              : translate(`${T}costValue`, '{{usd}} USD', { usd: facts.costUsd.toFixed(2) })}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">{translate(`${T}time`, 'Time')}</dt>
          <dd className="tabular-nums" data-testid="outcome-facts-time">
            {facts.seconds === null ? unknownText() : durationText(facts.seconds)}
          </dd>
        </div>
      </dl>
    </section>
  )
}
