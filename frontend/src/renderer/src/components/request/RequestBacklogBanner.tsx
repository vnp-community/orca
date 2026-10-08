/**
 * RequestBacklogBanner — CR-REQ-019-03
 *
 * Shown on a request in `request_backlog`: where it was returned from, why,
 * by whom and when, with a Reopen action.
 *
 * @module components/request/RequestBacklogBanner
 */

import React from 'react'
import { Inbox } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { formatRequestRelativeTime } from '@/lib/request-relative-time'
import type { OrcaRequest } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestBacklogBanner.'

type Props = {
  request: OrcaRequest
  reopening: boolean
  onReopen: () => void
}

export function RequestBacklogBanner({ request, reopening, onReopen }: Props): React.JSX.Element {
  const stage = request.returnedFromStage && request.returnedFromStage !== 'unknown' ? request.returnedFromStage : null
  return (
    <div
      role="status"
      data-testid="request-backlog-banner"
      className="flex items-start gap-2 rounded border border-border bg-muted/40 px-3 py-2 text-sm"
    >
      <Inbox className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="font-medium text-foreground">
          {stage
            ? translate(`${T}titleFrom`, 'Returned to backlog from {{stage}}', {
                stage: translate(`auto.components.request.StageTimeline.step.${stage === 'task' ? 'execution' : stage}`, stage)
              })
            : translate(`${T}title`, 'Returned to backlog')}
        </span>
        {request.returnReason && <span className="whitespace-pre-wrap text-muted-foreground">{request.returnReason}</span>}
        <span className="text-xs text-muted-foreground">
          {[request.returnedById, request.returnedAt ? formatRequestRelativeTime(request.returnedAt) : null]
            .filter(Boolean)
            .join(' · ')}
        </span>
      </div>
      <Button size="xs" variant="outline" disabled={reopening} onClick={onReopen}>
        {translate(`${T}reopen`, 'Reopen')}
      </Button>
    </div>
  )
}
