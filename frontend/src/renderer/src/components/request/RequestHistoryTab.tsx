/**
 * RequestHistoryTab — CR-REQ-019-04
 *
 * Type-change history, newest first.
 *
 * @module components/request/RequestHistoryTab
 */

import React from 'react'
import { ArrowRight, Bot, User } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'
import { formatRequestRelativeTime } from '@/lib/request-relative-time'
import { requestErrorMessage } from './request-error-message'
import { RequestTypeBadge } from './RequestTypeBadge'
import type { RequestTypeHistoryEntry } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestHistoryTab.'

type Props = {
  entries: RequestTypeHistoryEntry[]
  isLoading: boolean
  error: string | null
  onRetry: () => void
}

export function RequestHistoryTab({ entries, isLoading, error, onRetry }: Props): React.JSX.Element {
  if (isLoading && entries.length === 0) {
    return (
      <div className="flex flex-col gap-2 p-4" aria-busy="true">
        <Skeleton className="h-4 w-2/3" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    )
  }
  if (error) {
    return (
      <div role="alert" className="m-4 flex items-center gap-2 text-sm">
        <span>{requestErrorMessage(error)}</span>
        <Button size="xs" variant="outline" onClick={onRetry}>
          {translate('auto.components.request.error.retry', 'Retry')}
        </Button>
      </div>
    )
  }
  if (entries.length === 0) {
    return (
      <p className="p-4 text-sm text-muted-foreground" data-testid="request-history-empty">
        {translate(`${T}empty`, 'The type has not been changed yet.')}
      </p>
    )
  }

  const sorted = [...entries].sort((a, b) => Date.parse(b.occurredAt) - Date.parse(a.occurredAt))
  return (
    <ul className="flex flex-col divide-y divide-border" data-testid="request-history-list">
      {sorted.map((entry) => (
        <li key={entry.id} className="flex flex-col gap-1 px-4 py-2">
          <div className="flex items-center gap-2">
            <RequestTypeBadge type={entry.fromType} size="xs" />
            <ArrowRight className="size-3 text-muted-foreground" aria-hidden />
            <RequestTypeBadge type={entry.toType} size="xs" />
            <span className="ml-auto flex items-center gap-1 text-xs text-muted-foreground">
              {entry.actorKind === 'ai' ? <Bot className="size-3" aria-hidden /> : <User className="size-3" aria-hidden />}
              {entry.actorKind === 'ai' ? translate(`${T}ai`, 'AI') : (entry.actorId ?? '')}
              <span>{formatRequestRelativeTime(entry.occurredAt)}</span>
            </span>
          </div>
          {entry.reason && <p className="whitespace-pre-wrap text-sm text-foreground">{entry.reason}</p>}
        </li>
      ))}
    </ul>
  )
}
