/**
 * RequestListStates — CR-REQ-019-02
 *
 * Loading / empty / error presentations for the Requests list.
 *
 * @module components/request/RequestListStates
 */

import React from 'react'
import { AlertTriangle, Inbox, Lock, SearchX } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'
import { requestErrorMessage } from './request-error-message'

const T = 'auto.components.request.RequestListStates.'

export function RequestListSkeleton(): React.JSX.Element {
  return (
    <div className="flex flex-col" data-testid="request-list-skeleton" aria-busy="true">
      {Array.from({ length: 8 }, (_, i) => (
        <div key={i} className="flex flex-col gap-1.5 border-b border-border px-3 py-2.5">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-3 w-1/2" />
        </div>
      ))}
    </div>
  )
}

export function RequestListEmpty({
  filtered,
  onClearFilters,
  onCreate
}: {
  filtered: boolean
  onClearFilters: () => void
  onCreate?: () => void
}): React.JSX.Element {
  const Icon = filtered ? SearchX : Inbox
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-10 text-center" data-testid="request-list-empty">
      <Icon className="size-8 text-muted-foreground" aria-hidden />
      <p className="text-sm text-foreground">
        {filtered
          ? translate(`${T}emptyFiltered`, 'No requests match these filters.')
          : translate(`${T}emptyNone`, 'No requests yet.')}
      </p>
      {filtered ? (
        <Button variant="outline" size="sm" onClick={onClearFilters}>
          {translate(`${T}clearFilters`, 'Clear filters')}
        </Button>
      ) : (
        onCreate && (
          <Button size="sm" onClick={onCreate}>
            {translate(`${T}create`, 'Create request')}
          </Button>
        )
      )}
    </div>
  )
}

export function RequestListError({ kind, onRetry }: { kind: string; onRetry: () => void }): React.JSX.Element {
  const forbidden = kind === 'forbidden'
  const Icon = forbidden ? Lock : AlertTriangle
  return (
    <div
      role="alert"
      className="m-3 flex items-start gap-2 rounded border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm"
      data-testid="request-list-error"
    >
      <Icon className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden />
      <div className="flex flex-1 flex-col items-start gap-2">
        <span className="text-foreground">{requestErrorMessage(kind)}</span>
        {!forbidden && (
          <Button variant="outline" size="xs" onClick={onRetry}>
            {translate('auto.components.request.error.retry', 'Retry')}
          </Button>
        )}
      </div>
    </div>
  )
}
