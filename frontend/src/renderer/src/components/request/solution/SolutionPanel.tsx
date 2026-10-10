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
import { Skeleton } from '@/components/ui/skeleton'
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
import { SolutionGenerationState } from './SolutionGenerationState'
import { SolutionOpenQuestions } from './SolutionOpenQuestions'
import { SolutionRunFailure } from './SolutionRunFailure'
import { focusClarificationPanelWhenReady } from '../clarification/clarification-panel-focus'
import { SolutionStartPrompt } from './SolutionStartPrompt'
import { SolutionStatusBanner } from './SolutionStatusBanner'
import { SolutionVersionSwitcher } from './SolutionVersionSwitcher'
import { getSolutionPresentation, pickPendingApproval } from './solution-view-model'
import { useSolutionDecision } from './useSolutionDecision'
import { useSolutionStart } from './useSolutionStart'

const REFETCH_EVENT_RE = /(?:^|\.)(?:solution\.[a-z_]+|approval\.[a-z_]+|request\.status_changed)$/
// Why: a throttled regenerate must not be hammered; this floor keeps the old 5s guard when the server gave no wait.
const MIN_REGENERATE_LOCK_MS = 5000
// Why: a failed run raises no event, so a request that is still analysing must be re-read.
const RUN_POLL_INTERVAL_MS = 5000
// Why: the approval row is written after the Solution; a missed event must not leave the bar missing until reload.
const APPROVAL_POLL_INTERVAL_MS = 3000
const APPROVAL_POLL_MAX_TRIES = 20
// Why: after an answered Clarification the new Solution is written by the backend, with no guaranteed event.
const FRESH_SOLUTION_POLL_INTERVAL_MS = 3000
const FRESH_SOLUTION_POLL_MAX_TRIES = 40

function orderNewestFirst(solutions: Solution[]): Solution[] {
  return [...solutions].sort((a, b) => {
    const byVersion = (b.version ?? 0) - (a.version ?? 0)
    if (byVersion !== 0) {
      return byVersion
    }
    return (b.generatedAt ?? '').localeCompare(a.generatedAt ?? '')
  })
}

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
  const readCooldown = useRequestCooldown('read')
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

  const ordered = useMemo(() => orderNewestFirst(solutionsApi.solutions), [solutionsApi.solutions])
  const latest = ordered.find((s) => s.status !== 'superseded') ?? ordered[0]
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

  // Why: a Request that moved on (Clarification answered, regenerated) has a newer Solution; events can be missed,
  // so the Request's own state is the trigger. Approvals are not part of either payload, so they are re-read too.
  const requestKey = `${request.status}|${request.updatedAt ?? ''}|${request.version ?? ''}`
  const syncKey = `${requestKey}|${solution?.id ?? ''}|${solution?.status ?? ''}`
  const lastSyncKey = useRef(syncKey)
  const lastRequestKey = useRef(requestKey)
  useEffect(() => {
    if (lastSyncKey.current === syncKey) {
      return
    }
    lastSyncKey.current = syncKey
    if (lastRequestKey.current !== requestKey) {
      lastRequestKey.current = requestKey
      refetchSolutions()
    }
    refetchApprovals()
  }, [syncKey, requestKey, refetchApprovals, refetchSolutions])

  const awaitingApprovalRow =
    request.status === 'awaiting_analysis_approval' &&
    !!solution &&
    solution.status === 'ready' &&
    solution.id === latest?.id &&
    !approval &&
    !expiredApproval &&
    !readCooldown.coolingDown
  useEffect(() => {
    if (!awaitingApprovalRow) {
      return undefined
    }
    let tries = 0
    const t = setInterval(() => {
      tries += 1
      if (tries > APPROVAL_POLL_MAX_TRIES) {
        clearInterval(t)
        return
      }
      refetchApprovals()
    }, APPROVAL_POLL_INTERVAL_MS)
    return () => clearInterval(t)
  }, [awaitingApprovalRow, solution?.id, refetchApprovals])

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

  const latestRun = solutionsApi.latestRun ?? null
  const failedRun = latestRun?.status === 'failed' ? latestRun : null
  const runPending = latestRun?.status === 'running'
  const { refetch: reloadSolutions } = solutionsApi
  const polling =
    (runPending || (request.status === 'analyzing' && !latest && !failedRun)) &&
    !readCooldown.coolingDown
  useEffect(() => {
    if (!polling) {
      return undefined
    }
    const t = setInterval(reloadSolutions, RUN_POLL_INTERVAL_MS)
    return () => clearInterval(t)
  }, [polling, reloadSolutions])

  // Only superseded/rejected versions are on screen while the backend is producing the next one.
  const awaitingNewer =
    !!latest &&
    !runPending &&
    !failedRun &&
    ((latest.status === 'superseded' &&
      (request.status === 'analyzing' || request.status === 'awaiting_analysis_approval')) ||
      (latest.status === 'rejected' && request.status === 'analyzing'))
  useEffect(() => {
    if (!awaitingNewer || readCooldown.coolingDown) {
      return undefined
    }
    let tries = 0
    const t = setInterval(() => {
      tries += 1
      if (tries > FRESH_SOLUTION_POLL_MAX_TRIES) {
        clearInterval(t)
        return
      }
      reloadSolutions()
      refetchApprovals()
      // Why: the Request itself stays `analyzing` on screen until it is re-read.
      if (request.status === 'analyzing') {
        onChanged()
      }
    }, FRESH_SOLUTION_POLL_INTERVAL_MS)
    return () => clearInterval(t)
  }, [
    awaitingNewer,
    readCooldown.coolingDown,
    request.status,
    latest?.id,
    reloadSolutions,
    refetchApprovals,
    onChanged
  ])

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
    return (
      <div data-testid="solution-panel-loading" aria-busy="true" className="space-y-3 p-4">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }
  if (solutionsApi.error) {
    return (
      <div role="alert" data-testid="solution-panel-error" className="space-y-2 p-4">
        <p className="text-sm text-destructive">
          {solutionsApi.error === 'forbidden'
            ? translate(
                'auto.components.request.SolutionPanel.forbidden',
                'You do not have permission to view the analysis.'
              )
            : requestErrorMessage(solutionsApi.error)}
        </p>
        {solutionsApi.error !== 'forbidden' && (
          <Button size="sm" variant="outline" onClick={refetchSolutions}>
            {translate('auto.components.request.SolutionPanel.retry', 'Retry')}
          </Button>
        )}
      </div>
    )
  }
  if (!solution) {
    return (
      <div data-testid="solution-panel-empty" className="p-4">
        {failure || failedRun ? (
          <SolutionRunFailure
            code={failure ? failure.error.backendCode : failedRun?.errorCode}
            errorKind={failure?.error.kind}
            errorText={failure ? requestErrorMessageForError(failure.error) : null}
            onRegenerate={() => void decision.regenerate()}
            regenerateDisabled={decision.submitting || cooldown}
          />
        ) : start.failure ? (
          <SolutionRunFailure
            code={start.failure.backendCode}
            errorKind={start.failure.kind}
            errorText={requestErrorMessageForError(start.failure)}
            onRegenerate={() => void start.start()}
            regenerateDisabled={start.submitting || cooldown}
          />
        ) : request.status === 'analyzing' ? (
          runPending || start.started ? (
            <SolutionGenerationState variant="analyzing" />
          ) : (
            <SolutionStartPrompt
              onStart={() => void start.start()}
              disabled={start.submitting || cooldown}
              submitting={start.submitting}
            />
          )
        ) : (
          <p className="text-sm text-muted-foreground">
            {translate('auto.components.request.SolutionPanel.empty', 'No analysis yet.')}
          </p>
        )}
      </div>
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
            regenerateDisabled={decision.submitting || cooldown}
          />
        )}
        <SolutionVersionSwitcher solutions={ordered} currentId={solution.id} onSelect={setViewId} />
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
            regenerateDisabled={decision.submitting || cooldown}
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
        />
      )}
    </div>
  )
}

function approvalDecided(solution: Solution): boolean {
  return solution.status === 'chosen' || solution.status === 'rejected'
}
