/**
 * ClarificationDeadlineNote — FE-REQ-TASK-036-03
 *
 * @module components/request/clarification/ClarificationDeadlineNote
 */

import React from 'react'
import { Clock } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { dueState } from './clarification-answer-validation'

const T = 'auto.components.request.clarification.'

export function ClarificationDeadlineNote({ dueAt, now }: { dueAt?: string; now: number }): React.JSX.Element | null {
  const state = dueState(dueAt, now)
  if (state.kind === 'none') {return null}
  return (
    <p className="flex items-center gap-1 text-xs text-muted-foreground" data-testid="clarification-deadline">
      <Clock className="size-3" aria-hidden />
      {state.kind === 'overdue'
        ? translate(`${T}expired`, 'Overdue. The request will return to the backlog (missing information).')
        : translate(`${T}dueIn`, '{{count}} days left', { count: state.days })}
    </p>
  )
}
