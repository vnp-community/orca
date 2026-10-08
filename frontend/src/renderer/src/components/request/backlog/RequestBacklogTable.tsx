/**
 * RequestBacklogTable — CR-REQ-023-05
 *
 * Requests returned to the backlog, with Reopen and Cancel. Server order is
 * kept (updated_at DESC); rows are never re-sorted client side.
 *
 * @module components/request/backlog/RequestBacklogTable
 */

import React, { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { useRowListKeyboardNavigation } from '@/hooks/useRowListKeyboardNavigation'
import { useRequestActions } from '../../../hooks/useRequestActions'
import { CancelRequestDialog } from '../CancelRequestDialog'
import { requestErrorMessage } from '../request-error-message'
import { openRequestPage } from '../request-page-navigation'
import { BACKLOG_COLUMNS } from './backlog-view-columns'
import { ReopenRequestDialog } from './ReopenRequestDialog'
import { RequestBacklogRow } from './RequestBacklogRow'
import type { RequestBacklogRowData } from '../../../../../shared/request-backlog-types'

const T = 'auto.components.request.backlog.'

export type RequestBacklogTableProps = {
  rows: RequestBacklogRowData[]
  now: number
  locale: string
  hasMore: boolean
  isLoadingMore: boolean
  dimmed?: boolean
  onLoadMore: () => void
  /** Called after a successful write so the owner can refetch. */
  onChanged: () => void
}

export function RequestBacklogTable({
  rows, now, locale, hasMore, isLoadingMore, dimmed, onLoadMore, onChanged
}: RequestBacklogTableProps): React.JSX.Element {
  const actions = useRequestActions()
  const [removed, setRemoved] = useState<Set<string>>(new Set())
  const [reopenTarget, setReopenTarget] = useState<RequestBacklogRowData | null>(null)
  const [cancelTarget, setCancelTarget] = useState<RequestBacklogRowData | null>(null)
  const [reasonRequired, setReasonRequired] = useState<Set<string>>(new Set())

  const visible = rows.filter((r) => !removed.has(r.requestId))
  const open = useCallback((row: RequestBacklogRowData) => {
    openRequestPage({ section: 'requests', requestId: row.requestId })
  }, [])
  const { containerProps, getRowProps, activeKey } = useRowListKeyboardNavigation({
    items: visible, getKey: (r) => r.requestId, onOpen: open, containerRole: 'grid'
  })

  const dropRow = useCallback((requestId: string) => {
    setRemoved((prev) => new Set(prev).add(requestId))
    onChanged()
  }, [onChanged])

  const onReopened = useCallback((requestId: string) => {
    dropRow(requestId)
    toast(translate(`${T}BacklogRow.reopened`, 'Request reopened. It will be classified again.'), {
      action: {
        label: translate(`${T}ReopenRequestDialog.viewRequest`, 'View request'),
        onClick: () => openRequestPage({ section: 'requests', requestId })
      }
    })
  }, [dropRow])

  const confirmCancel = useCallback(async (reason: string): Promise<boolean> => {
    const target = cancelTarget
    if (!target) {return true}
    const result = await actions.cancel({ id: target.requestId, reason: reason || undefined })
    if (result.ok) {
      dropRow(target.requestId)
      return true
    }
    const { kind } = result.error
    if (kind === 'validation') {
      // Server says a reason is mandatory: flip the field to required and keep the dialog open.
      setReasonRequired((prev) => new Set(prev).add(target.requestId))
      return false
    }
    if (kind === 'invalid_state' || kind === 'conflict' || kind === 'not_found') {
      toast(translate(`${T}BacklogRow.alreadyHandled`, 'This request was already handled.'))
      dropRow(target.requestId)
      return true
    }
    toast.error(requestErrorMessage(kind))
    return false
  }, [actions, cancelTarget, dropRow])

  const columns = BACKLOG_COLUMNS.requests
  return (
    <div className={cn('flex-1 overflow-auto focus-visible:outline-none', dimmed && 'opacity-60')} data-testid="request-backlog-table" {...containerProps}>
      <Table>
        <TableHeader className="sticky top-0 z-10 bg-background">
          <TableRow>
            {columns.map((c) => (
              <TableHead key={c.id} scope="col">{translate(c.labelKey, c.fallback)}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {visible.map((row) => (
            <RequestBacklogRow
              key={row.requestId}
              row={row}
              now={now}
              locale={locale}
              active={activeKey === row.requestId}
              rowProps={getRowProps(row.requestId)}
              onOpen={open}
              onReopen={setReopenTarget}
              onCancel={setCancelTarget}
            />
          ))}
        </TableBody>
      </Table>
      {hasMore && (
        <div className="flex justify-center p-3">
          <Button variant="outline" size="sm" disabled={isLoadingMore} onClick={onLoadMore}>
            {translate(`${T}BacklogTab.loadMore`, 'Load more')}
          </Button>
        </div>
      )}
      {reopenTarget && (
        <ReopenRequestDialog
          open
          onOpenChange={(next) => { if (!next) {setReopenTarget(null)} }}
          request={{
            id: reopenTarget.requestId,
            number: reopenTarget.number,
            title: reopenTarget.title,
            returnedFromStage: reopenTarget.returnedFromStage
          }}
          onReopened={onReopened}
        />
      )}
      {cancelTarget && (
        <CancelRequestDialog
          open
          onOpenChange={(next) => { if (!next) {setCancelTarget(null)} }}
          reasonRequired={reasonRequired.has(cancelTarget.requestId)}
          onConfirm={confirmCancel}
        />
      )}
    </div>
  )
}
