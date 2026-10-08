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
import { useRequestSubscription } from '../../hooks/useRequestSubscription'
import { ChangeTypeConfirmDialog } from './ChangeTypeConfirmDialog'
import { CancelRequestDialog } from './CancelRequestDialog'
import { RequestBacklogBanner } from './RequestBacklogBanner'
import { RequestDetailHeader } from './RequestDetailHeader'
import { RequestHistoryTab } from './RequestHistoryTab'
import { RequestOverviewTab } from './RequestOverviewTab'
import { RequestRelatedTab } from './RequestRelatedTab'
import { RequestStageTimeline } from './RequestStageTimeline'
import { ClarificationPanel } from './clarification/ClarificationPanel'
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

function detailTabForFocus(focus: 'type_confirmation' | 'analysis' | 'plan' | undefined): string {
  return focus === 'analysis' || focus === 'plan' ? focus : 'overview'
}

export function RequestDetailPane({ requestId, onBackToList }: Props): React.JSX.Element {
  const { request, history, links, linksSupported, isLoading, error, refetch } = useRequest(requestId)
  const actions = useRequestActions()
  const currentUser = useAppStore((st) => st.currentUser)
  const [dialog, setDialog] = useState<Dialog>(null)
  const [busy, setBusy] = useState(false)
  // Why: the approval inbox / backlog deep-link to the tab that holds the gate being decided.
  const [tab, setTab] = useState(() => detailTabForFocus(useAppStore.getState().requestPage.focus))

  // Why: focus is a one-shot hint; clear it so selecting another request does not reuse it.
  useEffect(() => {
    if (useAppStore.getState().requestPage.focus) {useAppStore.getState().setRequestPageData({ focus: undefined })}
  }, [])

  useRequestSubscription({ requestId, onEvent: refetch })

  const closeToList = useCallback(() => {
    useAppStore.getState().setRequestPageRequest(null)
    onBackToList?.()
  }, [onBackToList])

  const run = useCallback(
    async (action: () => Promise<{ ok: boolean; error?: Parameters<typeof notifyRequestActionFailure>[0] }>): Promise<boolean> => {
      setBusy(true)
      const result = await action()
      setBusy(false)
      if (!result.ok) {
        if (result.error) {notifyRequestActionFailure(result.error, refetch)}
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
        <div className="flex flex-col gap-3 p-4" aria-busy="true" data-testid="request-detail-skeleton">
          <Skeleton className="h-6 w-2/3" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      )
    }
    return (
      <div role="alert" className="flex flex-col items-start gap-2 p-4 text-sm" data-testid="request-detail-error">
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

  const tabs = getVisibleDetailTabs(request.type)

  return (
    <div className="flex h-full flex-col overflow-hidden" data-testid="request-detail-pane">
      <RequestDetailHeader
        request={request}
        busy={busy}
        onBackToList={onBackToList}
        onCancel={() => setDialog('cancel')}
        onReopen={() =>
          void run(() => actions.reopen(request.id)).then((ok) => {
            if (ok) {toast.success(translate(`${T}reopened`, 'Request reopened'))}
          })
        }
        onReturnToBacklog={() => setDialog('return')}
        onChangeType={() => setDialog('changeType')}
        onSpawnChild={() => setDialog('spawnChild')}
      />
      <div className="flex-1 overflow-y-auto">
        <div className="flex flex-col gap-3 px-4 pt-3">
          {request.status === 'request_backlog' && (
            <RequestBacklogBanner
              request={request}
              reopening={busy}
              onReopen={() => void run(() => actions.reopen(request.id))}
            />
          )}
          <RequestStageTimeline request={request} />
          {request.status === 'awaiting_information' && (
            <ClarificationPanel
              request={request}
              currentUserId={currentUser?.id ?? null}
              isAdmin={currentUser?.role === 'admin'}
            />
          )}
          {(request.status === 'awaiting_type_confirmation' || request.status === 'classifying') && (
            <TypeConfirmationCard request={request} onChanged={refetch} />
          )}
        </div>
        <Tabs value={tab} onValueChange={setTab} className="mt-3">
          <TabsList className="mx-4">
            <TabsTrigger value="overview">{translate(`${T}tab.overview`, 'Overview')}</TabsTrigger>
            {tabs.analysis && <TabsTrigger value="analysis">{translate(`${T}tab.analysis`, 'Analysis')}</TabsTrigger>}
            {tabs.plan && <TabsTrigger value="plan">{translate(`${T}tab.plan`, 'Plan')}</TabsTrigger>}
            <TabsTrigger value="history">{translate(`${T}tab.history`, 'History')}</TabsTrigger>
            <TabsTrigger value="related">{translate(`${T}tab.related`, 'Related')}</TabsTrigger>
          </TabsList>
          <TabsContent value="overview">
            <RequestOverviewTab request={request} />
          </TabsContent>
          {tabs.analysis && (
            <TabsContent value="analysis">
              <RequestAnalysisTab request={request} onChanged={refetch} />
            </TabsContent>
          )}
          {tabs.plan && (
            <TabsContent value="plan">
              <RequestPlanTab request={request} onChanged={refetch} />
            </TabsContent>
          )}
          <TabsContent value="history">
            <RequestHistoryTab entries={history} isLoading={isLoading} error={error} onRetry={refetch} />
          </TabsContent>
          <TabsContent value="related">
            <RequestRelatedTab requestId={request.id} links={links} linksSupported={linksSupported} />
          </TabsContent>
        </Tabs>
      </div>

      {dialog === 'cancel' && (
        <CancelRequestDialog
          open
          onOpenChange={(open) => !open && setDialog(null)}
          onConfirm={(reason) => run(() => actions.cancel({ id: request.id, reason: reason || undefined }))}
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
