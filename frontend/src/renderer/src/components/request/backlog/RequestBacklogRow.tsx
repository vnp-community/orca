/**
 * RequestBacklogRow — CR-REQ-023-05
 *
 * One Request returned to the backlog. Writes (reopen/cancel) are emitted;
 * the table owns the dialogs.
 *
 * @module components/request/backlog/RequestBacklogRow
 */

import React from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { TableCell, TableRow } from '@/components/ui/table'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { formatAbsoluteTime, formatRelativeTime } from '@/lib/request-relative-time'
import { RequestSourceBadge } from '../RequestSourceBadge'
import { RequestTypeBadge } from '../RequestTypeBadge'
import { stageLabel } from './ReopenRequestDialog'
import type { RequestBacklogRowData } from '../../../../../shared/request-backlog-types'

const T = 'auto.components.request.backlog.'

const CATEGORY_LABELS: Record<string, string> = {
  missing_info: 'Missing information', infeasible: 'Not feasible', blocked_dependency: 'Blocked by dependency',
  rejected: 'Rejected', other: 'Other'
}

export type RequestBacklogRowProps = {
  row: RequestBacklogRowData
  now: number
  locale: string
  active: boolean
  rowProps: Record<string, unknown>
  onOpen: (row: RequestBacklogRowData) => void
  onReopen: (row: RequestBacklogRowData) => void
  onCancel: (row: RequestBacklogRowData) => void
}

export function RequestBacklogRow({
  row, now, locale, active, rowProps, onOpen, onReopen, onCancel
}: RequestBacklogRowProps): React.JSX.Element {
  const actor =
    !row.returnedBy || row.returnedBy === 'system'
      ? translate(`${T}RequestBacklogTable.actorSystem`, 'AI / System')
      : row.returnedBy
  return (
    <TableRow {...rowProps} role="row" className={cn(active && 'bg-accent/60')} data-testid="request-backlog-row">
      <TableCell className="max-w-72">
        <button
          type="button"
          className="block w-full truncate text-left font-medium text-foreground hover:underline"
          onClick={() => onOpen(row)}
        >
          #{row.number} {row.title}
        </button>
      </TableCell>
      <TableCell>
        {row.sourceProvider === 'unknown' ? (
          <span className="text-muted-foreground">-</span>
        ) : (
          <RequestSourceBadge provider={row.sourceProvider} ref={row.sourceRef} url={row.sourceUrl} size="xs" />
        )}
      </TableCell>
      <TableCell>{row.type ? <RequestTypeBadge type={row.type} size="xs" /> : <span className="text-muted-foreground">-</span>}</TableCell>
      <TableCell>{stageLabel(row.returnedFromStage)}</TableCell>
      <TableCell>
        {row.returnedCategory === 'unknown' ? (
          <span className="text-muted-foreground">-</span>
        ) : (
          <Badge variant="outline">
            {translate(`${T}ReturnedCategory.${row.returnedCategory}`, CATEGORY_LABELS[row.returnedCategory])}
          </Badge>
        )}
      </TableCell>
      <TableCell className="max-w-64">
        <span className="line-clamp-2 text-sm" title={row.returnReason}>
          {row.returnReason ?? '-'}
        </span>
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">{actor}</TableCell>
      <TableCell className="whitespace-nowrap text-sm text-muted-foreground" title={formatAbsoluteTime(row.returnedAt, locale)}>
        {formatRelativeTime(row.returnedAt, now, locale)}
      </TableCell>
      <TableCell>
        <div className="flex items-center gap-1.5">
          <Button variant="outline" size="sm" onClick={() => onReopen(row)}>
            {translate(`${T}RequestBacklogTable.reopen`, 'Reopen')}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => onCancel(row)}>
            {translate(`${T}RequestBacklogTable.cancel`, 'Cancel')}
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}
