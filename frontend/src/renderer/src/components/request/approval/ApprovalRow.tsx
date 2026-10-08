/**
 * ApprovalRow — CR-REQ-022-05
 *
 * One pending approval. Decisions are emitted, not performed: confirmation and
 * the reject dialog belong to ApprovalInboxTab.
 *
 * @module components/request/approval/ApprovalRow
 */

import React from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { formatAbsoluteTime, formatRelativeTime } from '@/lib/request-relative-time'
import { RequestTypeBadge } from '../RequestTypeBadge'
import { ApprovalDueLabel } from './ApprovalDueLabel'
import { ApprovalSubjectIcon } from './ApprovalSubjectIcon'
import { dueAtOf, subjectKindOf } from './approval-inbox-rules'
import type { Approval, OrcaRequest } from '../../../../../shared/request-types'

const T = 'auto.components.request.approval.'

export type ApprovalRowProps = {
  approval: Approval
  request?: OrcaRequest
  now: number
  locale: string
  canQuick: boolean
  busy: boolean
  onOpen: (approval: Approval) => void
  onQuickApprove: (approval: Approval) => void
  onReject: (approval: Approval) => void
}

export function requestHeading(requestId: string, request?: OrcaRequest): string {
  return request ? `#${request.number} ${request.title}` : `#${requestId.slice(0, 8)}`
}

export function ApprovalRow({
  approval, request, now, locale, canQuick, busy, onOpen, onQuickApprove, onReject
}: ApprovalRowProps): React.JSX.Element {
  const kind = subjectKindOf(approval)
  const requester =
    !approval.requestedBy || approval.requestedBy === 'system'
      ? translate(`${T}ApprovalRow.systemActor`, 'AI / System')
      : approval.requestedBy
  return (
    <div
      className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-3 py-2.5"
      aria-busy={busy || !request}
      data-testid="approval-row"
    >
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex min-w-0 items-center gap-2">
          <ApprovalSubjectIcon kind={kind} />
          <span className="truncate text-sm font-medium text-foreground">
            {requestHeading(approval.requestId, request)}
          </span>
          {request && <RequestTypeBadge type={request.type} size="xs" />}
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
          <span>{translate(`${T}ApprovalRow.requestedBy`, 'Requested by {{name}}', { name: requester })}</span>
          <span title={formatAbsoluteTime(approval.createdAt, locale)}>
            {formatRelativeTime(approval.createdAt, now, locale)}
          </span>
          <ApprovalDueLabel dueAt={dueAtOf(approval)} now={now} locale={locale} />
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        <Button variant="outline" size="sm" disabled={busy} onClick={() => onOpen(approval)}>
          {translate(`${T}ApprovalRow.open`, 'Open')}
        </Button>
        {canQuick && (
          <Button size="sm" disabled={busy} onClick={() => onQuickApprove(approval)}>
            {busy && <Loader2 className="size-3.5 animate-spin" aria-hidden />}
            {translate(`${T}ApprovalRow.approve`, 'Quick approve')}
          </Button>
        )}
        <Button variant="ghost" size="sm" disabled={busy} onClick={() => onReject(approval)}>
          {translate(`${T}ApprovalRow.reject`, 'Reject')}
        </Button>
      </div>
    </div>
  )
}
