/**
 * SolutionPanel — CR-REQ-020-02
 *
 * Analysis tab body: loads solutions + approvals for a request, shows the
 * current version, and hosts the decision bar.
 *
 * @module components/request/solution/SolutionPanel
 */

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { useApprovals } from '../../../hooks/useApprovals'
import { useRequestCooldown } from '../../../hooks/useRequestCooldown'
import { useRequestActions } from '../../../hooks/useRequestActions'
import { useRequestSubscription } from '../../../hooks/useRequestSubscription'
import { useSolutions } from '../../../hooks/useSolutions'
import { REQUEST_POLL_EVENT_TYPE } from '../../../hooks/useRequestEvents'
import { cooldownMsFor, startRequestCooldown } from '../../../runtime/request-rate-limit'
import { parseBlockingQuestionIds } from '../../../../../shared/request-errors'
import { REQUEST_FLOW_REGISTRY } from '../../../../../shared/request-flow-registry'
import type { OrcaRequest, Solution } from '../../../../../shared/request-types'
import { requestErrorMessage, requestErrorMessageForError } from '../request-error-message'
import { SolutionBody } from './SolutionBody'
import { SolutionDecisionBar } from './SolutionDecisionBar'
import { SolutionAskBackDialog } from './SolutionAskBackDialog'
import { SolutionEmptyState } from './SolutionEmptyState'
import { SolutionGenerationState } from './SolutionGenerationState'
import { SolutionPanelError, SolutionPanelLoading } from './SolutionPanelLoadStates'
import { SolutionOpenQuestions } from './SolutionOpenQuestions'
import { SolutionRunFailure } from './SolutionRunFailure'
import { SolutionRevisionFeedback } from './SolutionRevisionFeedback'
import { focusClarificationPanelWhenReady } from '../clarification/clarification-panel-focus'
import { SolutionStatusBanner } from './SolutionStatusBanner'
import { SolutionVersionSwitcher } from './SolutionVersionSwitcher'
import { OlderVersionBanner } from './OlderVersionBanner'
import { AnalysisRevisionPanel } from './AnalysisRevisionPanel'
import { currentSolutionOf, orderSolutionsNewestFirst } from './solution-version-history'
import { getSolutionPresentation, pickPendingApproval } from './solution-view-model'
import { useAnalysisRevision } from './useAnalysisRevision'
import { useSolutionPanelSync } from './useSolutionPanelSync'
import { useSolutionDecision } from './useSolutionDecision'
import { useSolutionStart } from './useSolutionStart'

const REFETCH_EVENT_RE = /(?:^|\.)(?:solution\.[a-z_]+|approval\.[a-z_]+|request\.status_changed)$/
// Why: a throttled regenerate must not be hammered; this floor keeps the old 5s guard when the server gave no wait.
const MIN_REGENERATE_LOCK_MS = 5000

export type SolutionPanelProps = {
  request: OrcaRequest
  /** Call after any mutation so the detail pane refetches the request. */
  onChanged: () => void
}

export function SolutionPanel({ request, onChanged }: SolutionPanelProps): React.JSX.Element {
  const solutionsApi = useSolutions(request.id)
  const approvalsApi = useApprovals({ requestId: request.id })
  const requestActions = useRequestActions()
  const { refetch: refetchSolutions } = solutionsApi
  const { refetch: refetchApprovals } = approvalsApi

  const [viewId, setViewId] = useState<string | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [askBackOpen, setAskBackOpen] = useState(false)
  const aiCooldown = useRequestCooldown('ai')
  const cooldown = aiCooldown.coolingDown
  // Why: later polls also set isLoading; only the very first read should replace the panel with a skeleton.
  const loadedOnce = useRef(false)
  if (!solutionsApi.isLoading) {
    loadedOnce.current = true
  }

  const refetchAll = useCallback(() => {
    refetchSolutions()
    refetchApprovals()
    onChanged()
  }, [refetchSolutions, refetchApprovals, onChanged])

  useRequestSubscription({
    requestId: request.id,
    onEvent: (event) => {
      if (REFETCH_EVENT_RE.test(event.eventType) || event.eventType === REQUEST_POLL_EVENT_TYPE) {
        refetchSolutions()
        refetchApprovals()
      }
    }
  })

  const ordered = useMemo(
    () => orderSolutionsNewestFirst(solutionsApi.solutions),
    [solutionsApi.solutions]
  )
  const latest = currentSolutionOf(ordered)
  const solution = ordered.find((s) => s.id === viewId) ?? latest

  const approval = useMemo(
    () => (solution ? pickPendingApproval(approvalsApi.approvals, solution) : null),
    [approvalsApi.approvals, solution]
  )
  const expiredApproval = useMemo(
    () =>
      !!solution &&
      approvalsApi.approvals.some((a) => a.status === 'expired' && a.subjectId === solution.id),
    [approvalsApi.approvals, solution]
  )

  const decision = useSolutionDecision({
    solution:
      solution ??
      ({ id: '', requestId: request.id, kind: 'unknown', status: 'unknown' } as Solution),
    approval,
    actions: {
      choose: solutionsApi.choose,
      generate: solutionsApi.generate,
      approve: approvalsApi.approve,
      reject: approvalsApi.reject
    },
    onSettled: refetchAll
  })

  const start = useSolutionStart({
    generateWhenIdle: solutionsApi.generateWhenIdle,
    onSettled: refetchAll
  })

  const { failedRun, runPending, awaitingNewer } = useSolutionPanelSync({
    request,
    solution,
    latest,
    approval,
    expiredApproval,
    latestRun: solutionsApi.latestRun ?? null,
    refetchSolutions,
    refetchApprovals,
    onChanged
  })

  const revision = useAnalysisRevision({
    request,
    solution,
    actions: { generate: solutionsApi.generate },
    onSettled: refetchAll
  })

  // Keep the local selection across refetches only while that option still exists.
  const optionIds = (solution?.options ?? []).map((o) => o.id).join('|')
  useEffect(() => {
    setSelectedId((cur) => (cur && optionIds.split('|').includes(cur) ? cur : null))
  }, [optionIds])
  useEffect(() => {
    setSelectedId(null)
  }, [solution?.id])

  const failure = decision.failure
  useEffect(() => {
    if (!failure) {
      return
    }
    if (failure.errorClass === 'forbidden' || failure.errorClass === 'notApprover') {
      toast.error(requestErrorMessage('forbidden'))
    } else if (failure.errorClass === 'rateLimited') {
      toast.error(requestErrorMessageForError(failure.error))
      startRequestCooldown('ai', Math.max(cooldownMsFor(failure.error), MIN_REGENERATE_LOCK_MS))
    }
  }, [failure])

  const flow = request.type === 'unknown' ? undefined : REQUEST_FLOW_REGISTRY[request.type]
  const canGeneratePlan = !!flow && flow.plan !== 'none' && !request.planTaskId
  const generatePlan = (): void => {
    void requestActions.generatePlan(request.id).then((res) => {
      if (!res.ok) {
        toast.error(requestErrorMessageForError(res.error))
      } else {
        refetchAll()
      }
    })
  }

  // ---- load / empty / error states ----
  if (solutionsApi.isLoading && !loadedOnce.current && solutionsApi.solutions.length === 0) {
    return <SolutionPanelLoading />
  }
  if (solutionsApi.error) {
    return <SolutionPanelError error={solutionsApi.error} onRetry={refetchSolutions} />
  }
  if (!solution) {
    return (
      <SolutionEmptyState
        requestStatus={request.status}
        runPending={runPending}
        failedRun={failedRun}
        decision={decision}
        start={start}
        cooldown={cooldown}
      />
    )
  }

  const presentation = getSolutionPresentation({
    solution,
    requestStatus: request.status,
    requestType: request.type,
    hasPendingApproval: approval !== null
  })
  const isCurrent = solution.id === latest?.id
  // Older versions are always read-only; the decision bar only belongs to the current one.
  const readOnly = presentation.readOnly || !isCurrent || approvalDecided(solution)
  // Why: a reply only makes sense while this version can still be approved; a superseded one is replaced on answer.
  const canAskBack =
    isCurrent &&
    !readOnly &&
    solution.status === 'ready' &&
    (request.status === 'awaiting_analysis_approval' || request.status === 'analyzing')
  // Why: only a refused approval proves which questions the server still counts as unanswered.
  // Why: a superseded Solution's questions were already asked back; only the current version's still gate approval.
  const showQuestions = isCurrent && solution.status !== 'superseded'
  const barShown = isCurrent && !presentation.readOnly && presentation.actions.length > 0
  // Why: once the revision runs the Request leaves the gate, yet its progress must stay visible until the new version lands.
  const showRevision = isCurrent && (revision.busy || (revision.canShow && barShown))
  const refusedIds =
    showQuestions && failure?.errorClass === 'blockingQuestions'
      ? parseBlockingQuestionIds(failure.error.message)
      : []

  return (
    <div data-testid="request-tab-analysis" className="flex h-full min-h-0 flex-col">
      <div className="flex-1 space-y-4 overflow-y-auto p-4 scrollbar-sleek">
        {failedRun && (
          <SolutionRunFailure
            code={failedRun.errorCode}
            onRegenerate={() => void decision.regenerate()}
            regenerateDisabled={decision.submitting || cooldown || revision.busy}
          />
        )}
        <SolutionVersionSwitcher
          solutions={ordered}
          currentId={solution.id}
          latestId={latest?.id}
          // Why: selecting the current version means "follow the newest", so a revision that lands is shown.
          onSelect={(id) => setViewId(id === latest?.id ? null : id)}
        />
        {!isCurrent && <OlderVersionBanner onBack={() => setViewId(null)} />}
        <SolutionRevisionFeedback feedback={solution.revisionFeedback} />
        <SolutionStatusBanner
          solution={solution}
          presentation={presentation}
          approval={approval}
          conflictNotice={decision.conflicted}
        />
        {solution.status === 'generating' ? (
          <SolutionGenerationState variant="drafting" />
        ) : (
          <SolutionBody
            solution={solution}
            requestType={request.type}
            selectedId={
              selectedId ??
              (solution.status === 'chosen' ? (solution.chosenOptionId ?? null) : null)
            }
            onSelect={setSelectedId}
            readOnly={readOnly}
            onRegenerate={() => void decision.regenerate()}
            regenerateDisabled={decision.submitting || cooldown || revision.busy}
          />
        )}
        {awaitingNewer && request.status === 'analyzing' && (
          <SolutionGenerationState variant="analyzing" />
        )}
        <SolutionOpenQuestions
          questions={showQuestions ? (solution.openQuestions ?? []) : []}
          refusedIds={refusedIds}
          actionable={!readOnly}
          renderAction={
            canAskBack
              ? (q) =>
                  q.blocking ? (
                    <Button
                      size="xs"
                      variant="outline"
                      className="self-start"
                      data-testid="solution-ask-back"
                      data-question-id={q.id}
                      onClick={() => setAskBackOpen(true)}
                    >
                      {translate('auto.components.request.SolutionAskBackDialog.ask', 'Ask back')}
                    </Button>
                  ) : null
              : undefined
          }
        />
        {showRevision && <AnalysisRevisionPanel revision={revision} kind={solution.kind} />}
      </div>
      {canAskBack && (
        <SolutionAskBackDialog
          open={askBackOpen}
          onOpenChange={setAskBackOpen}
          requestId={request.id}
          solutionId={solution.id}
          questions={solution.openQuestions ?? []}
          refusedIds={refusedIds}
          onAsked={() => {
            refetchAll()
            focusClarificationPanelWhenReady()
          }}
        />
      )}
      {isCurrent && (
        <SolutionDecisionBar
          solution={solution}
          requestType={request.type}
          presentation={presentation}
          approval={approval}
          expiredApproval={expiredApproval}
          selectedId={selectedId}
          decision={decision}
          onGeneratePlan={canGeneratePlan ? generatePlan : undefined}
          regenerateCooldown={cooldown}
          locked={revision.busy}
        />
      )}
    </div>
  )
}

function approvalDecided(solution: Solution): boolean {
  return solution.status === 'chosen' || solution.status === 'rejected'
}
