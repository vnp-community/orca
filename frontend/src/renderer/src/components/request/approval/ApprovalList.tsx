/**
 * ApprovalList — CR-REQ-022-05
 *
 * Rows grouped by Request (group order = first row after sorting) with
 * j/k/Enter navigation over the flat row list.
 *
 * @module components/request/approval/ApprovalList
 */

import React, { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { useRowListKeyboardNavigation } from '@/hooks/useRowListKeyboardNavigation'
import { RequestTypeBadge } from '../RequestTypeBadge'
import { ApprovalRow, requestHeading } from './ApprovalRow'
import { canQuickApprove } from './approval-inbox-rules'
import type { Approval, OrcaRequest } from '../../../../../shared/request-types'

export type ApprovalListProps = {
  groups: { requestId: string; rows: Approval[] }[]
  requestsById: Record<string, OrcaRequest>
  now: number
  locale: string
  busyIds: Set<string>
  hasMore: boolean
  isLoadingMore: boolean
  dimmed?: boolean
  onOpen: (approval: Approval) => void
  onQuickApprove: (approval: Approval) => void
  onReject: (approval: Approval) => void
  onLoadMore: () => void
}

export function ApprovalList({
  groups, requestsById, now, locale, busyIds, hasMore, isLoadingMore, dimmed,
  onOpen, onQuickApprove, onReject, onLoadMore
}: ApprovalListProps): React.JSX.Element {
  const flat = useMemo(() => groups.flatMap((g) => g.rows), [groups])
  const { containerProps, getRowProps, activeKey } = useRowListKeyboardNavigation({
    items: flat,
    getKey: (a) => a.id,
    onOpen
  })

  return (
    <div
      {...containerProps}
      aria-label={translate('auto.components.request.approval.ApprovalInboxTab.title', 'Approvals')}
      className={cn('flex-1 overflow-y-auto focus-visible:outline-none', dimmed && 'opacity-60')}
      data-testid="approval-list"
    >
      {groups.map((group) => {
        const request = requestsById[group.requestId]
        return (
          <section key={group.requestId} aria-label={requestHeading(group.requestId, request)}>
            <header className="sticky top-0 z-10 flex items-center gap-2 border-b border-border bg-muted/60 px-3 py-1.5 text-xs text-muted-foreground backdrop-blur">
              <span className="truncate font-medium text-foreground">{requestHeading(group.requestId, request)}</span>
              {request && <RequestTypeBadge type={request.type} size="xs" />}
              <span className="ml-auto tabular-nums">{group.rows.length}</span>
            </header>
            {group.rows.map((row) => (
              <div
                key={row.id}
                {...getRowProps(row.id)}
                role="option"
                className={cn(
                  'border-b border-border focus-visible:outline-none',
                  activeKey === row.id && 'bg-accent/60'
                )}
              >
                <ApprovalRow
                  approval={row}
                  request={request}
                  now={now}
                  locale={locale}
                  canQuick={canQuickApprove(row)}
                  busy={busyIds.has(row.id)}
                  onOpen={onOpen}
                  onQuickApprove={onQuickApprove}
                  onReject={onReject}
                />
              </div>
            ))}
          </section>
        )
      })}
      {hasMore && (
        <div className="flex justify-center p-3">
          <Button variant="outline" size="sm" disabled={isLoadingMore} onClick={onLoadMore}>
            {translate('auto.components.request.approval.ApprovalInboxTab.loadMore', 'Load more')}
          </Button>
        </div>
      )}
    </div>
  )
}
