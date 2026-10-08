/**
 * BacklogStates — CR-REQ-023-04
 *
 * Loading / empty / error presentations shared by the three backlog views.
 *
 * @module components/request/backlog/BacklogStates
 */

import React from 'react'
import { AlertTriangle, Inbox, Lock, SearchX } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'
import type { BacklogView } from '../../../../../shared/request-types'

const T = 'auto.components.request.backlog.'

export function BacklogSkeleton({ columns }: { columns: number }): React.JSX.Element {
  return (
    <div className="flex flex-col" data-testid="backlog-skeleton" aria-busy="true">
      {Array.from({ length: 8 }, (_, row) => (
        <div key={row} className="flex gap-3 border-b border-border px-3 py-3">
          {Array.from({ length: columns }, (_, col) => (
            <Skeleton key={col} className="h-4 flex-1" />
          ))}
        </div>
      ))}
    </div>
  )
}

const EMPTY_TEXT: Record<BacklogView, string> = {
  requests: 'No Request has been returned to the backlog.',
  tasks: 'Every task already has an approved Plan.',
  execute: 'No task is waiting to run or failed.'
}

export function BacklogEmptyState({ view }: { view: BacklogView }): React.JSX.Element {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-10 text-center" data-testid="backlog-empty">
      <Inbox className="size-7 text-muted-foreground" aria-hidden />
      <p className="text-sm text-foreground">{translate(`${T}BacklogEmptyState.${view}`, EMPTY_TEXT[view])}</p>
    </div>
  )
}

export function BacklogEmptyFiltered({ onClear }: { onClear: () => void }): React.JSX.Element {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-10 text-center" data-testid="backlog-empty-filtered">
      <SearchX className="size-7 text-muted-foreground" aria-hidden />
      <p className="text-sm text-foreground">{translate(`${T}BacklogEmptyState.filtered`, 'Nothing matches these filters.')}</p>
      <Button variant="outline" size="sm" onClick={onClear}>
        {translate(`${T}BacklogEmptyState.clearFilters`, 'Clear filters')}
      </Button>
    </div>
  )
}

export function BacklogErrorState({
  kind,
  onRetry
}: {
  kind: 'network' | 'forbidden' | 'unknown'
  onRetry: () => void
}): React.JSX.Element {
  const Icon = kind === 'forbidden' ? Lock : AlertTriangle
  return (
    <div
      role="alert"
      className="m-3 flex items-center gap-2 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive"
      data-testid={`backlog-error-${kind}`}
    >
      <Icon className="size-4 shrink-0" aria-hidden />
      <span className="flex-1">
        {kind === 'forbidden'
          ? translate(`${T}BacklogErrorState.forbidden`, 'You do not have permission to see the backlog of this project.')
          : translate(`${T}BacklogErrorState.network`, 'Could not load the backlog. Check your connection.')}
      </span>
      {kind !== 'forbidden' && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          {translate(`${T}BacklogErrorState.retry`, 'Retry')}
        </Button>
      )}
    </div>
  )
}
