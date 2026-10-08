/**
 * RequestRow — CR-REQ-019-02
 *
 * One selectable row of the Requests list.
 *
 * @module components/request/RequestRow
 */

import React from 'react'
import { Flame } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { formatRequestRelativeTime } from '@/lib/request-relative-time'
import { RequestStatusBadge } from './RequestStatusBadge'
import { RequestTypeBadge } from './RequestTypeBadge'
import { RequestSourceBadge } from './RequestSourceBadge'
import type { OrcaRequest } from '../../../../shared/request-types'

type Props = {
  request: OrcaRequest
  selected: boolean
  onSelect: (id: string) => void
}

export const RequestRow = React.memo(function RequestRow({ request, selected, onSelect }: Props): React.JSX.Element {
  return (
    <button
      type="button"
      role="option"
      aria-selected={selected}
      data-request-id={request.id}
      onClick={() => onSelect(request.id)}
      className={cn(
        'flex w-full flex-col gap-1 border-b border-border px-3 py-2 text-left transition-colors hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring',
        selected && 'bg-accent'
      )}
    >
      <div className="flex items-baseline gap-2">
        <span className="shrink-0 text-xs text-muted-foreground">#{request.number}</span>
        <span className="min-w-0 flex-1 truncate text-sm text-foreground">{request.title}</span>
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        {request.type === 'unknown' ? (
          <span className="text-xs text-muted-foreground">
            {translate('auto.components.request.RequestRow.unclassified', 'Unclassified')}
          </span>
        ) : (
          <RequestTypeBadge type={request.type} size="xs" />
        )}
        <RequestStatusBadge status={request.status} size="xs" />
        {request.source && request.source.provider !== 'manual' && (
          <RequestSourceBadge
            provider={request.source.provider}
            ref={request.source.ref}
            url={request.source.url}
            size="xs"
          />
        )}
        {request.urgency === 'urgent' && (
          <span className="inline-flex items-center gap-0.5 rounded border border-destructive/40 px-1 text-[10px] text-destructive">
            <Flame className="size-2.5" aria-hidden />
            {translate('auto.components.request.RequestRow.urgent', 'Urgent')}
          </span>
        )}
        <span className="ml-auto text-[10px] text-muted-foreground">
          {formatRequestRelativeTime(request.updatedAt)}
        </span>
      </div>
    </button>
  )
})
