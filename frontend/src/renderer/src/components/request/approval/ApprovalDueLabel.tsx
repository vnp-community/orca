/**
 * ApprovalDueLabel — CR-REQ-022-05
 *
 * Relative deadline; overdue adds a warning icon so it is not colour-only.
 *
 * @module components/request/approval/ApprovalDueLabel
 */

import React from 'react'
import { AlertTriangle, Clock } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { formatAbsoluteTime, formatDueState } from '@/lib/request-relative-time'

const T = 'auto.components.request.approval.'

export function ApprovalDueLabel({
  dueAt,
  now,
  locale
}: {
  dueAt: string | undefined
  now: number
  locale: string
}): React.JSX.Element | null {
  const due = formatDueState(dueAt, now, locale)
  if (due.kind === 'none') {return null}
  const absolute = formatAbsoluteTime(dueAt, locale)
  if (due.kind === 'overdue') {
    return (
      <span className="inline-flex items-center gap-1 text-xs text-destructive" title={absolute} data-testid="approval-overdue">
        <AlertTriangle className="size-3.5" aria-hidden />
        {translate(`${T}ApprovalRow.overdueBy`, 'Overdue {{relative}}', { relative: due.text })}
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground" title={absolute}>
      <Clock className="size-3.5" aria-hidden />
      {translate(`${T}ApprovalRow.dueIn`, 'Due {{relative}}', { relative: due.text })}
    </span>
  )
}
