/**
 * DecisionHistoryList — FE-REQ-TASK-036-04
 *
 * @module components/request/decision/DecisionHistoryList
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import type { Decision } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.decision.'
const STATUS_FALLBACK: Record<string, string> = { open: 'Open', chosen: 'Chosen', effective: 'Effective', superseded: 'Superseded', unknown: 'Unknown' }

type Props = {
  decisions: readonly Decision[]
  /** Maps option ids to titles for display. */
  optionTitle?: (optionId: string) => string | undefined
}

export function DecisionHistoryList({ decisions, optionTitle }: Props): React.JSX.Element {
  if (decisions.length === 0) {
    return <p className="text-xs text-muted-foreground">{translate(`${T}history.empty`, 'No decisions recorded yet')}</p>
  }
  return (
    <ol className="flex flex-col gap-2" aria-label={translate(`${T}history.title`, 'Decision history')}>
      {decisions.map((d) => (
        <li key={d.id} className={cn('rounded-md border border-border px-3 py-2 text-xs', d.status === 'superseded' && 'opacity-60')}>
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{d.displayId}</span>
            <Badge variant="outline">{translate(`${T}status.${d.status}`, STATUS_FALLBACK[d.status] ?? d.status)}</Badge>
            {d.chosenOptionId ? <span>{optionTitle?.(d.chosenOptionId) ?? d.chosenOptionId}</span> : null}
            {d.chooserId ? <span className="text-muted-foreground">{d.chooserId}</span> : null}
          </div>
          {d.rationale ? <p className="mt-1 whitespace-pre-wrap text-muted-foreground">{d.rationale}</p> : null}
        </li>
      ))}
    </ol>
  )
}
