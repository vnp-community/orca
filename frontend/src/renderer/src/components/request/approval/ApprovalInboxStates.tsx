/**
 * ApprovalInboxStates — CR-REQ-022-05
 *
 * Loading / empty / error presentations for the approval inbox.
 *
 * @module components/request/approval/ApprovalInboxStates
 */

import React from 'react'
import { AlertTriangle, Inbox, Lock, SearchX } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'

const T = 'auto.components.request.approval.'

export function ApprovalInboxSkeleton(): React.JSX.Element {
  return (
    <div className="flex flex-col" data-testid="approval-skeleton" aria-busy="true">
      {Array.from({ length: 6 }, (_, i) => (
        <div key={i} className="flex flex-col gap-1.5 border-b border-border px-3 py-3">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-3 w-1/3" />
        </div>
      ))}
    </div>
  )
}

export function ApprovalInboxEmptyState(): React.JSX.Element {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-10 text-center" data-testid="approval-empty">
      <Inbox className="size-7 text-muted-foreground" aria-hidden />
      <p className="text-sm text-foreground">{translate(`${T}ApprovalInboxTab.empty`, 'Nothing is waiting for your approval.')}</p>
    </div>
  )
}

export function ApprovalInboxEmptyFiltered({ onClear }: { onClear: () => void }): React.JSX.Element {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-10 text-center" data-testid="approval-empty-filtered">
      <SearchX className="size-7 text-muted-foreground" aria-hidden />
      <p className="text-sm text-foreground">{translate(`${T}ApprovalInboxTab.emptyFiltered`, 'No approvals match these filters.')}</p>
      <Button variant="outline" size="sm" onClick={onClear}>
        {translate(`${T}ApprovalInboxTab.clearFilters`, 'Clear filters')}
      </Button>
    </div>
  )
}

export function ApprovalInboxErrorState({
  kind,
  onRetry
}: {
  kind: 'network' | 'forbidden'
  onRetry: () => void
}): React.JSX.Element {
  const Icon = kind === 'forbidden' ? Lock : AlertTriangle
  return (
    <div
      role="alert"
      className="m-3 flex items-center gap-2 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive"
      data-testid={`approval-error-${kind}`}
    >
      <Icon className="size-4 shrink-0" aria-hidden />
      <span className="flex-1">
        {kind === 'forbidden'
          ? translate(`${T}ApprovalInboxErrorState.forbidden`, 'You do not have permission to see these approvals.')
          : translate(`${T}ApprovalInboxErrorState.network`, 'Could not load approvals. Check your connection.')}
      </span>
      {kind === 'network' && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          {translate(`${T}ApprovalInboxErrorState.retry`, 'Retry')}
        </Button>
      )}
    </div>
  )
}
