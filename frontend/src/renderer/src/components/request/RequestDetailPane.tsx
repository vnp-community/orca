/**
 * RequestDetailPane — CR-REQ-019-03
 *
 * Right-hand pane of the Requests tab: header actions, stage timeline and the
 * Overview / Analysis / Plan / Execution / Outcome review / History / Related tabs for one request (the tabs are RequestDetailTabs).
 *
 * @module components/request/RequestDetailPane
 */

import React, { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useRequest } from '../../hooks/useRequest'
import { useRequestActions } from '../../hooks/useRequestActions'
import { useRequestPostAnswerRefresh } from '../../hooks/useRequestPostAnswerRefresh'
import { useRequestRefreshOnReturn } from '../../hooks/useRequestRefreshOnReturn'
import { useRequestSubscription } from '../../hooks/useRequestSubscription'
import { useRequestFlow } from '../../hooks/useRequestFlow'
import { useExecutionSummary } from '../../hooks/useExecutionSummary'
import { useOutcomeReview } from '../../hooks/useOutcomeReview'
import { useOutcomeReviewEnabled } from '../../hooks/useOutcomeReviewEnabled'
import { ChangeTypeConfirmDialog } from './ChangeTypeConfirmDialog'
import { CancelRequestDialog } from './CancelRequestDialog'
import { RequestBacklogBanner } from './RequestBacklogBanner'
import { RequestDetailHeader } from './RequestDetailHeader'
import { NextActionBar } from './NextActionBar'
import { pickNextAction, type NextAction, type NextActionCommand } from './next-action-rules'
import { withReviewHint } from './next-action-review'
import {
  needsDecisionItems,
  reviewHintOf,
  timelineRowStateOf
} from './outcome-review/outcome-review-view-model'
import { RequestDetailTabs } from './RequestDetailTabs'
import { RequestStageTimeline } from './RequestStageTimeline'
import {
  CLARIFICATION_PANEL_DOM_ID,
  ClarificationPanel,
  type ClarificationPanelState
} from './clarification/ClarificationPanel'
import { ReturnToBacklogDialog } from './ReturnToBacklogDialog'
import { SpawnChildRequestDialog } from './SpawnChildRequestDialog'
import { TypeConfirmationCard } from './TypeConfirmationCard'
import { notifyRequestActionFailure } from './request-action-feedback'
import { requestErrorMessage } from './request-error-message'
import { getVisibleDetailTabs } from './request-action-rules'
import type { ReturnedFromStage } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestDetailPane.'

type Dialog = 'cancel' | 'return' | 'changeType' | 'spawnChild' | null

type Props = {
  requestId: string
  onBackToList?: () => void
}

const EXECUTION_POLL_MS = 5000

// Why: the review is offered from the moment a Request has a plan, which is also when the server accepts a review.
const REVIEW_STATUSES: ReadonlySet<string> = new Set([
  'awaiting_plan_approval',
  'executing',
  'completed'
])

function detailTabForFocus(focus: 'type_confirmation' | 'analysis' | 'plan' | undefined): string {
  return focus === 'analysis' || focus === 'plan' ? focus : 'overview'
}

export function RequestDetailPane({ requestId, onBackToList }: Props): React.JSX.Element {
  const { request, history, links, linksSupported, isLoading, error, refetch } =
    useRequest(requestId)
  const actions = useRequestActions()
  const flow = useRequestFlow(request?.type, request?.size)
  // Why: events refresh the summary on their own; the timer only runs while tasks are being executed.
  const execution = useExecutionSummary(requestId, {
    pollMs: request?.status === 'executing' ? EXECUTION_POLL_MS : 0
  })
  const reviewFlag = useOutcomeReviewEnabled(request?.projectId)
  const reviewActive = reviewFlag.enabled && !!request && REVIEW_STATUSES.has(request.status)
  // Why: with the flag off or no plan yet the hook reads nothing, so there is no review traffic at all.
  const review = useOutcomeReview(request?.id ?? null, { enabled: reviewActive })
  const currentUser = useAppStore((st) => st.currentUser)
  const [dialog, setDialog] = useState<Dialog>(null)
  const [busy, setBusy] = useState(false)
  const [clarification, setClarification] = useState<ClarificationPanelState>({ answerable: false })
  // Why: the approval inbox / backlog deep-link to the tab that holds the gate being decided.
  const [tab, setTab] = useState(() => detailTabForFocus(useAppStore.getState().requestPage.focus))

  // Why: focus is a one-shot hint; clear it so selecting another request does not reuse it.
  useEffect(() => {
    if (useAppStore.getState().requestPage.focus) {
      useAppStore.getState().setRequestPageData({ focus: undefined })
    }
  }, [])

  useRequestSubscription({ requestId, onEvent: refetch })
  // Why: no gateway event announces a changed starting branch, so coming back to the window is the refresh point.
  useRequestRefreshOnReturn(refetch)
  const armPostAnswerRefresh = useRequestPostAnswerRefresh(request?.status, refetch)
  const onClarificationAnswered = useCallback(() => {
    refetch()
    armPostAnswerRefresh()
  }, [refetch, armPostAnswerRefresh])

  const closeToList = useCallback(() => {
    useAppStore.getState().setRequestPageRequest(null)
    onBackToList?.()
  }, [onBackToList])

  const run = useCallback(
    async (
      action: () => Promise<{
        ok: boolean
        error?: Parameters<typeof notifyRequestActionFailure>[0]
      }>
    ): Promise<boolean> => {
      setBusy(true)
      const result = await action()
      setBusy(false)
      if (!result.ok) {
        if (result.error) {
          notifyRequestActionFailure(result.error, refetch)
        }
        return false
      }
      refetch()
      return true
    },
    [refetch]
  )

  if (!request) {
    if (isLoading || !error) {
      return (
        <div
          className="flex flex-col gap-3 p-4"
          aria-busy="true"
          data-testid="request-detail-skeleton"
        >
          <Skeleton className="h-6 w-2/3" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      )
    }
    return (
      <div
        role="alert"
        className="flex flex-col items-start gap-2 p-4 text-sm"
        data-testid="request-detail-error"
      >
        <span>
          {error === 'not_found'
            ? translate(`${T}notFound`, 'This request no longer exists.')
            : requestErrorMessage(error)}
        </span>
        {error === 'not_found' ? (
          <Button size="xs" variant="outline" onClick={closeToList}>
            {translate(`${T}backToList`, 'Back to list')}
          </Button>
        ) : (
          <Button size="xs" variant="outline" onClick={refetch}>
            {translate('auto.components.request.error.retry', 'Retry')}
          </Button>
        )}
      </div>
    )
  }

  const tabs = getVisibleDetailTabs(flow)
  // Why: a runtime without the summary channel cannot show a run, so the tab is not offered (and no red error).
  const showExecution = tabs.execution && !execution.unsupported
  const openDecisions = review.review ? needsDecisionItems(review.review).length : 0
  const nextAction = withReviewHint(
    pickNextAction({
      request,
      summary: execution.summary,
      clarificationAnswerable: clarification.answerable,
      tabs: { ...tabs, execution: showExecution }
    }),
    reviewHintOf({
      enabled: reviewActive,
      requestStatus: request.status,
      review: review.review,
      loaded: review.status === 'ready' || review.status === 'none'
    }),
    openDecisions
  )
  const onNextActionCommand = (command: NextActionCommand, action: NextAction): void => {
    if (command === 'reopen') {
      void run(() => actions.reopen(request.id))
    } else if (command === 'answer') {
      const panel = document.getElementById(CLARIFICATION_PANEL_DOM_ID)
      panel?.scrollIntoView({ block: 'start' })
      panel?.focus({ preventScroll: true })
    } else if (action.primaryTab ?? action.tab) {
      setTab((action.primaryTab ?? action.tab)!)
    }
  }
  const executionNeedsAttention =
    nextAction.kind === 'failed' ||
    (nextAction.kind === 'needs_you' && nextAction.tab === 'execution')
  const planNeedsAttention = nextAction.kind === 'needs_you' && nextAction.tab === 'plan'
  const reviewNeedsAttention =
    nextAction.kind === 'needs_you' && nextAction.tab === 'outcome-review'
  // Why: the failure card and the next-action bar already say a task ran out of attempts; the long banner would repeat it.
  const taskFailureStop =
    request.status === 'request_backlog' && (execution.summary?.failed.length ?? 0) > 0

  return (
    <div className="flex h-full flex-col overflow-hidden" data-testid="request-detail-pane">
      <RequestDetailHeader
        request={request}
        busy={busy}
        onBackToList={onBackToList}
        onCancel={() => setDialog('cancel')}
        onReopen={() =>
          void run(() => actions.reopen(request.id)).then((ok) => {
            if (ok) {
              toast.success(translate(`${T}reopened`, 'Request reopened'))
            }
          })
        }
        onReturnToBacklog={() => setDialog('return')}
        onChangeType={() => setDialog('changeType')}
        onSpawnChild={() => setDialog('spawnChild')}
        onAnswerClarification={
          clarification.answerable
            ? () => {
                const panel = document.getElementById(CLARIFICATION_PANEL_DOM_ID)
                panel?.scrollIntoView({ block: 'start' })
                panel?.focus({ preventScroll: true })
              }
            : undefined
        }
      />
      <div className="flex-1 overflow-y-auto">
        <div className="flex flex-col gap-3 px-4 pt-3">
          {request.status === 'request_backlog' && !taskFailureStop && (
            <RequestBacklogBanner
              request={request}
              reopening={busy}
              onReopen={() => void run(() => actions.reopen(request.id))}
            />
          )}
          <RequestStageTimeline
            request={request}
            flow={flow}
            summary={execution.summary}
            resumeStatus={
              request.status === 'awaiting_information' ? clarification.resumeStatus : undefined
            }
            review={{
              enabled: reviewFlag.enabled,
              row: timelineRowStateOf(review.review),
              openDecisions
            }}
          />
          <NextActionBar action={nextAction} busy={busy} onCommand={onNextActionCommand} />
          {request.status === 'awaiting_information' && (
            <ClarificationPanel
              request={request}
              currentUserId={currentUser?.id ?? null}
              isAdmin={currentUser?.role === 'admin'}
              onStateChange={setClarification}
              onAnswered={onClarificationAnswered}
            />
          )}
          {(request.status === 'awaiting_type_confirmation' ||
            request.status === 'classifying') && (
            <TypeConfirmationCard request={request} onChanged={refetch} />
          )}
        </div>
        <RequestDetailTabs
          tab={tab}
          onTabChange={setTab}
          request={request}
          flow={flow}
          tabs={tabs}
          showExecution={showExecution}
          showReview={reviewActive}
          review={review}
          execution={execution}
          busy={busy}
          planNeedsAttention={planNeedsAttention}
          executionNeedsAttention={executionNeedsAttention}
          reviewNeedsAttention={reviewNeedsAttention}
          history={history}
          links={links}
          linksSupported={linksSupported}
          isLoading={isLoading}
          error={error}
          onChanged={refetch}
          onReopen={() => void run(() => actions.reopen(request.id))}
          onStop={() => setDialog('cancel')}
          onAnswerClarification={() => onNextActionCommand('answer', nextAction)}
        />
      </div>

      {dialog === 'cancel' && (
        <CancelRequestDialog
          open
          onOpenChange={(open) => !open && setDialog(null)}
          onConfirm={(reason) =>
            run(() => actions.cancel({ id: request.id, reason: reason || undefined }))
          }
        />
      )}
      {dialog === 'return' && (
        <ReturnToBacklogDialog
          open
          status={request.status}
          onOpenChange={(open) => !open && setDialog(null)}
          onConfirm={(stage: ReturnedFromStage, reason) =>
            run(() => actions.returnToBacklog({ id: request.id, stage, reason }))
          }
        />
      )}
      {dialog === 'changeType' && (
        <ChangeTypeConfirmDialog
          open
          request={request}
          onOpenChange={(open) => !open && setDialog(null)}
          onChanged={refetch}
          onSpawnChild={() => setDialog('spawnChild')}
        />
      )}
      {dialog === 'spawnChild' && (
        <SpawnChildRequestDialog
          open
          request={request}
          onOpenChange={(open) => !open && setDialog(null)}
          onChanged={refetch}
        />
      )}
    </div>
  )
}
