/**
 * RequestDetailPane — CR-REQ-019-03
 *
 * Right-hand pane of the Requests tab: header actions, stage timeline and the
 * Overview / Analysis / Plan / History / Related tabs for one request.
 *
 * @module components/request/RequestDetailPane
 */

import React, { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useRequest } from '../../hooks/useRequest'
import { useRequestActions } from '../../hooks/useRequestActions'
import { useRequestPostAnswerRefresh } from '../../hooks/useRequestPostAnswerRefresh'
import { useRequestRefreshOnReturn } from '../../hooks/useRequestRefreshOnReturn'
import { useRequestSubscription } from '../../hooks/useRequestSubscription'
import { useRequestFlow } from '../../hooks/useRequestFlow'
import { useExecutionSummary } from '../../hooks/useExecutionSummary'
import { ChangeTypeConfirmDialog } from './ChangeTypeConfirmDialog'
import { CancelRequestDialog } from './CancelRequestDialog'
import { RequestBacklogBanner } from './RequestBacklogBanner'
import { RequestDetailHeader } from './RequestDetailHeader'
import { RequestActivityTab } from './activity/RequestActivityTab'
import { ExecutionTab } from './execution/ExecutionTab'
import { NextActionBar } from './NextActionBar'
import { pickNextAction, type NextAction, type NextActionCommand } from './next-action-rules'
import { RequestOverviewTab } from './RequestOverviewTab'
import { RequestRelatedTab } from './RequestRelatedTab'
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
import { RequestPlanTab } from './plan/RequestPlanTab'
import { RequestAnalysisTab } from './solution/RequestAnalysisTab'
import type { ReturnedFromStage } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestDetailPane.'

type Dialog = 'cancel' | 'return' | 'changeType' | 'spawnChild' | null

type Props = {
  requestId: string
  onBackToList?: () => void
}

const EXECUTION_POLL_MS = 5000

function TabDot({ label }: { label: string }): React.JSX.Element {
  return (
    <span
      role="img"
      aria-label={label}
      className="ml-1.5 inline-block size-1.5 rounded-full bg-primary"
      data-testid="tab-attention-dot"
    />
  )
}

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
  const nextAction = pickNextAction({
    request,
    summary: execution.summary,
    clarificationAnswerable: clarification.answerable,
    tabs: { ...tabs, execution: showExecution }
  })
  const onNextActionCommand = (command: NextActionCommand, action: NextAction): void => {
    if (command === 'reopen') {
      void run(() => actions.reopen(request.id))
    } else if (command === 'answer') {
      const panel = document.getElementById(CLARIFICATION_PANEL_DOM_ID)
      panel?.scrollIntoView({ block: 'start' })
      panel?.focus({ preventScroll: true })
    } else if (action.tab) {
      setTab(action.tab)
    }
  }
  const executionNeedsAttention =
    nextAction.kind === 'failed' ||
    (nextAction.kind === 'needs_you' && nextAction.tab === 'execution')
  const planNeedsAttention = nextAction.kind === 'needs_you' && nextAction.tab === 'plan'
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
        <Tabs value={tab} onValueChange={setTab} className="mt-3">
          <TabsList className="mx-4">
            <TabsTrigger value="overview">{translate(`${T}tab.overview`, 'Overview')}</TabsTrigger>
            {tabs.analysis && (
              <TabsTrigger value="analysis">
                {translate(`${T}tab.analysis`, 'Analysis')}
              </TabsTrigger>
            )}
            {tabs.plan && (
              <TabsTrigger value="plan">
                {translate(`${T}tab.plan`, 'Plan')}
                {planNeedsAttention && (
                  <TabDot label={translate(`${T}tab.needsAttention`, 'Needs attention')} />
                )}
              </TabsTrigger>
            )}
            {showExecution && (
              <TabsTrigger value="execution">
                {translate(`${T}tab.execution`, 'Execution')}
                {executionNeedsAttention && (
                  <TabDot label={translate(`${T}tab.needsAttention`, 'Needs attention')} />
                )}
              </TabsTrigger>
            )}
            <TabsTrigger value="history">{translate(`${T}tab.history`, 'History')}</TabsTrigger>
            <TabsTrigger value="related">{translate(`${T}tab.related`, 'Related')}</TabsTrigger>
          </TabsList>
          <TabsContent value="overview">
            <RequestOverviewTab request={request} onChanged={refetch} />
          </TabsContent>
          {tabs.analysis && (
            <TabsContent value="analysis">
              <RequestAnalysisTab request={request} onChanged={refetch} />
            </TabsContent>
          )}
          {tabs.plan && (
            <TabsContent value="plan">
              <RequestPlanTab
                request={request}
                onChanged={refetch}
                flow={flow}
                onOpenExecution={showExecution ? () => setTab('execution') : undefined}
              />
            </TabsContent>
          )}
          {showExecution && (
            <TabsContent value="execution">
              <ExecutionTab
                request={request}
                flow={flow}
                execution={execution}
                busy={busy}
                onChanged={refetch}
                onOpenPlan={tabs.plan ? () => setTab('plan') : undefined}
                onReopen={() => void run(() => actions.reopen(request.id))}
                onStop={() => setDialog('cancel')}
                onAnswerClarification={() => onNextActionCommand('answer', nextAction)}
              />
            </TabsContent>
          )}
          <TabsContent value="history">
            <RequestActivityTab
              requestId={request.id}
              onOpenTab={setTab}
              active={tab === 'history'}
              typeHistory={history}
              typeHistoryLoading={isLoading}
              typeHistoryError={error}
              onRetryTypeHistory={refetch}
            />
          </TabsContent>
          <TabsContent value="related">
            <RequestRelatedTab
              requestId={request.id}
              links={links}
              linksSupported={linksSupported}
            />
          </TabsContent>
        </Tabs>
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
