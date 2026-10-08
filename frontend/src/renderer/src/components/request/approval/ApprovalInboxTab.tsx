/**
 * ApprovalInboxTab — CR-REQ-022-06
 *
 * Pending approvals for the current user: filter, quick approve, reject and
 * deep link into the Request. Keeps no approval list in the store.
 *
 * @module components/request/approval/ApprovalInboxTab
 */

import React, { useCallback, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { i18n, translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useApprovalInbox, type ApprovalDecisionResult } from '../../../hooks/useApprovalInbox'
import { useMinuteClock } from '../../../hooks/useMinuteClock'
import { useRequestSummaries } from '../../../hooks/useRequestSummaries'
import { requestErrorMessage } from '../request-error-message'
import { openRequestPage } from '../request-page-navigation'
import { RejectReasonDialog, type RejectReasonSubmitResult } from '../solution/RejectReasonDialog'
import { ApprovalInboxToolbar } from './ApprovalInboxToolbar'
import { ApprovalList } from './ApprovalList'
import {
  ApprovalInboxEmptyFiltered, ApprovalInboxEmptyState, ApprovalInboxErrorState, ApprovalInboxSkeleton
} from './ApprovalInboxStates'
import { approvalSubjectLabel } from './ApprovalSubjectIcon'
import { requestHeading } from './ApprovalRow'
import {
  compareApprovals, groupByRequest, openTargetFor, quickApproveConsequenceKey, subjectKindOf,
  canQuickApprove, type ApprovalSubjectGroup
} from './approval-inbox-rules'
import type { Approval } from '../../../../../shared/request-types'

const T = 'auto.components.request.approval.'

export function ApprovalInboxTab(): React.JSX.Element | null {
  const projectId = useAppStore((s) => s.requestPage.listFilters.projectId)
  const confirm = useConfirmationDialog()
  const [subjectGroup, setSubjectGroup] = useState<ApprovalSubjectGroup>('all')
  const [overdueOnly, setOverdueOnly] = useState(false)
  const [busyIds, setBusyIds] = useState<Set<string>>(new Set())
  const [rejectTarget, setRejectTarget] = useState<Approval | null>(null)
  const now = useMinuteClock()
  const locale = i18n.language || 'en'

  const inbox = useApprovalInbox({ subjectGroup, projectId, overdueOnly, active: true, now })
  const requestIds = useMemo(() => [...new Set(inbox.rows.map((r) => r.requestId))], [inbox.rows])
  const summaries = useRequestSummaries(requestIds)

  // Rows whose Request no longer exists cannot be opened; hide them until the server list catches up.
  const visibleRows = useMemo(
    () => inbox.rows.filter((r) => !summaries.notFoundIds.has(r.requestId)),
    [inbox.rows, summaries.notFoundIds]
  )
  const groups = useMemo(
    () => groupByRequest([...visibleRows].sort(compareApprovals(now))),
    [visibleRows, now]
  )

  const filtered = subjectGroup !== 'all' || overdueOnly || Boolean(projectId)
  const clearFilters = useCallback(() => {
    setSubjectGroup('all')
    setOverdueOnly(false)
    useAppStore.getState().setRequestPageData({
      listFilters: { ...useAppStore.getState().requestPage.listFilters, projectId: undefined }
    })
  }, [])

  const withBusy = useCallback(async <R,>(id: string, fn: () => Promise<R>): Promise<R> => {
    setBusyIds((prev) => new Set(prev).add(id))
    try {
      return await fn()
    } finally {
      setBusyIds((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    }
  }, [])

  const open = useCallback((approval: Approval) => {
    if (summaries.notFoundIds.has(approval.requestId)) {
      toast(translate(`${T}ApprovalRow.requestGone`, 'This request no longer exists.'))
      inbox.refetch()
      return
    }
    openRequestPage(openTargetFor(approval))
  }, [summaries.notFoundIds, inbox])

  // Shared by approve and reject; returns whether the caller's dialog may close.
  const announce = useCallback((approval: Approval, result: ApprovalDecisionResult): boolean => {
    switch (result.outcome) {
      case 'ok':
        toast.success(translate(`${T}ApprovalRow.decided`, 'Decision recorded.'))
        return true
      case 'closed':
        // Neutral on purpose: someone else got there first, nothing went wrong.
        toast(translate(`${T}ApprovalRow.alreadyDecided`, 'Already decided by someone else.'))
        return true
      case 'changed':
        toast(translate(`${T}ApprovalRow.changed`, 'This item changed after it was listed. Open it to review.'), {
          action: { label: translate(`${T}ApprovalRow.open`, 'Open'), onClick: () => openRequestPage(openTargetFor(approval)) }
        })
        return true
      case 'forbidden':
        toast.error(requestErrorMessage('forbidden'))
        return true
      case 'unsupported':
        return true
      default:
        toast.error(requestErrorMessage(result.outcome === 'network' ? 'network' : 'unknown'))
        return false
    }
  }, [])

  const quickApprove = useCallback(async (approval: Approval) => {
    if (!canQuickApprove(approval)) {return}
    const request = summaries.byId[approval.requestId]
    const confirmed = await confirm({
      title: `${requestHeading(approval.requestId, request)} · ${approvalSubjectLabel(subjectKindOf(approval))}`,
      description: translate(quickApproveConsequenceKey(approval), 'The request moves to the next step.'),
      confirmLabel: translate(`${T}ApprovalRow.approve`, 'Quick approve'),
      confirmVariant: 'default'
    })
    if (!confirmed) {return}
    const result = await withBusy(approval.id, () => inbox.approve(approval))
    announce(approval, result)
  }, [announce, confirm, inbox, summaries.byId, withBusy])

  const submitReject = useCallback(async (comment: string): Promise<RejectReasonSubmitResult> => {
    const approval = rejectTarget
    if (!approval) {return { ok: true }}
    const result = await withBusy(approval.id, () => inbox.reject(approval, comment))
    if (result.outcome === 'validation') {return { ok: false, error: { kind: 'validation' } }}
    const close = announce(approval, result)
    return close ? { ok: true } : { ok: false, error: { kind: result.outcome === 'network' ? 'network' : 'unknown' } }
  }, [announce, inbox, rejectTarget, withBusy])

  if (!inbox.supported) {return null}

  const errorKind = inbox.error?.kind
  const forbidden = errorKind === 'forbidden'
  const showSkeleton = inbox.isLoading && groups.length === 0 && !inbox.error
  const empty = !inbox.isLoading && groups.length === 0 && !inbox.error

  return (
    <div className="flex h-full flex-col overflow-hidden" data-testid="request-tab-approvals">
      <ApprovalInboxToolbar
        subjectGroup={subjectGroup}
        overdueOnly={overdueOnly}
        onSubjectGroupChange={setSubjectGroup}
        onOverdueChange={setOverdueOnly}
      />
      {forbidden ? (
        <ApprovalInboxErrorState kind="forbidden" onRetry={inbox.refetch} />
      ) : (
        <>
          {inbox.error && <ApprovalInboxErrorState kind="network" onRetry={inbox.refetch} />}
          {showSkeleton ? (
            <ApprovalInboxSkeleton />
          ) : empty ? (
            filtered ? <ApprovalInboxEmptyFiltered onClear={clearFilters} /> : <ApprovalInboxEmptyState />
          ) : (
            groups.length > 0 && (
              <ApprovalList
                groups={groups}
                requestsById={summaries.byId}
                now={now}
                locale={locale}
                busyIds={busyIds}
                hasMore={inbox.hasMore}
                isLoadingMore={inbox.isLoadingMore}
                dimmed={Boolean(inbox.error)}
                onOpen={open}
                onQuickApprove={(a) => void quickApprove(a)}
                onReject={setRejectTarget}
                onLoadMore={inbox.loadMore}
              />
            )
          )}
        </>
      )}
      <RejectReasonDialog
        open={rejectTarget !== null}
        onOpenChange={(next) => { if (!next) {setRejectTarget(null)} }}
        title={
          rejectTarget
            ? `${translate(`${T}ApprovalRow.reject`, 'Reject')} · ${requestHeading(rejectTarget.requestId, summaries.byId[rejectTarget.requestId])}`
            : undefined
        }
        onSubmit={submitReject}
      />
    </div>
  )
}
