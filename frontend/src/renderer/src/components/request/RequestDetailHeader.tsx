/**
 * RequestDetailHeader — CR-REQ-019-03
 *
 * Number, title, status and the action buttons the current status allows.
 *
 * @module components/request/RequestDetailHeader
 */

import React, { useState } from 'react'
import { ArrowLeft, Ban, GitBranchPlus, Network, Pencil, Undo2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { RequestGraphSheet } from '../graph/RequestGraphSheet'
import { useAppStore } from '../../store'
import { getRequestActionAvailability } from './request-action-rules'
import { RequestStatusBadge } from './RequestStatusBadge'
import { RequestTypeBadge } from './RequestTypeBadge'
import type { OrcaRequest } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestDetailHeader.'

type Props = {
  request: OrcaRequest
  busy: boolean
  /** Present in the narrow layout, where the detail replaces the list. */
  onBackToList?: () => void
  onCancel: () => void
  onReopen: () => void
  onReturnToBacklog: () => void
  onChangeType: () => void
  onSpawnChild: () => void
}

export function RequestDetailHeader({
  request,
  busy,
  onBackToList,
  onCancel,
  onReopen,
  onReturnToBacklog,
  onChangeType,
  onSpawnChild
}: Props): React.JSX.Element {
  const can = getRequestActionAvailability(request)
  const flowSupport = useAppStore((st) => st.requestFlowSupport)
  const [graphOpen, setGraphOpen] = useState(false)
  return (
    <header className="flex flex-col gap-2 border-b border-border px-4 py-3" data-testid="request-detail-header">
      {onBackToList && (
        <Button variant="ghost" size="xs" className="self-start" onClick={onBackToList}>
          <ArrowLeft className="size-3.5" aria-hidden />
          {translate(`${T}backToList`, 'Back to list')}
        </Button>
      )}
      <div className="flex items-start gap-2">
        <span className="pt-0.5 text-sm text-muted-foreground">#{request.number}</span>
        <h2 className="min-w-0 flex-1 break-words text-base font-semibold text-foreground">{request.title}</h2>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <RequestStatusBadge status={request.status} />
        {request.type !== 'unknown' && <RequestTypeBadge type={request.type} />}
        <div className="ml-auto flex flex-wrap items-center gap-1">
          {flowSupport === 'supported' && (
            <Button size="xs" variant="outline" onClick={() => setGraphOpen(true)}>
              <Network className="size-3" aria-hidden />
              {translate('auto.components.graph.Entry.viewGraph', 'View graph')}
            </Button>
          )}
          {can.canReopen && (
            <Button size="xs" variant="outline" disabled={busy} onClick={onReopen}>
              {translate(`${T}reopen`, 'Reopen')}
            </Button>
          )}
          {can.canReturnToBacklog && (
            <Button size="xs" variant="outline" disabled={busy} onClick={onReturnToBacklog}>
              <Undo2 className="size-3" aria-hidden />
              {translate(`${T}returnToBacklog`, 'Return to backlog')}
            </Button>
          )}
          {can.canChangeType && (
            <Button size="xs" variant="outline" disabled={busy} onClick={onChangeType}>
              <Pencil className="size-3" aria-hidden />
              {translate(`${T}changeType`, 'Change type')}
            </Button>
          )}
          {can.canSpawnChild && (
            <Button size="xs" variant="outline" disabled={busy} onClick={onSpawnChild}>
              <GitBranchPlus className="size-3" aria-hidden />
              {translate(`${T}spawnChild`, 'Create child request')}
            </Button>
          )}
          {can.canCancel && (
            <Button size="xs" variant="outline" disabled={busy} onClick={onCancel} className="text-destructive">
              <Ban className="size-3" aria-hidden />
              {translate(`${T}cancel`, 'Cancel request')}
            </Button>
          )}
        </div>
      </div>
      {graphOpen && (
        <RequestGraphSheet
          request={request}
          open={graphOpen}
          onOpenChange={setGraphOpen}
          subject={{ type: request.planTaskId ? 'plan' : 'solution_option', id: request.planTaskId ?? request.id }}
          impactAssessed={request.planTaskId ? undefined : false}
        />
      )}
    </header>
  )
}
