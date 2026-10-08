/**
 * BacklogGroupHeaderRow — CR-REQ-023-06
 *
 * Plan/Phase heading above its tasks; backlog pages are grouped by Request.
 *
 * @module components/request/backlog/BacklogGroupHeaderRow
 */

import React from 'react'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'
import { translate } from '@/i18n/i18n'
import { BacklogGateStatusBadge } from './BacklogGateStatusBadge'
import type { BacklogGroupData } from '../../../../../shared/request-backlog-types'
import type { BacklogView, OrcaRequest } from '../../../../../shared/request-types'

const T = 'auto.components.request.backlog.'

export function BacklogGroupHeaderRow({
  group,
  view,
  colSpan,
  request,
  onOpenPlan
}: {
  group: BacklogGroupData
  view: BacklogView
  colSpan: number
  request?: OrcaRequest
  onOpenPlan: (requestId: string) => void
}): React.JSX.Element {
  const heading = group.phaseTitle ?? group.planTitle ?? '-'
  // Inferred (CR-023 C10): the contract has no explicit "plan not split into phases" flag.
  const notSplit = view === 'tasks' && group.gateStatus === 'none' && !group.phaseTaskId
  return (
    <TableRow className="bg-muted/40 hover:bg-muted/40" data-testid="backlog-group-header">
      <TableCell colSpan={colSpan}>
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <span className="text-sm font-medium text-foreground">{heading}</span>
          <BacklogGateStatusBadge status={group.gateStatus} />
          {notSplit && (
            <span className="text-muted-foreground">{translate(`${T}TaskBacklogRow.planNotSplit`, 'Not split into phases yet')}</span>
          )}
          <span className="truncate text-muted-foreground">
            {request ? `#${request.number} ${request.title}` : `#${group.requestId.slice(0, 8)}`}
          </span>
          <Button variant="link" size="sm" className="ml-auto h-auto p-0" onClick={() => onOpenPlan(group.requestId)}>
            {translate(`${T}BacklogGroupHeaderRow.openPlan`, 'Open plan')}
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}
