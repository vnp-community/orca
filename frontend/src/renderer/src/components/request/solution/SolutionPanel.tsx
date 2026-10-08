/**
 * SolutionPanel — CR-REQ-020-02
 *
 * Analysis tab body: loads solutions + approvals for a request, shows the
 * current version, and hosts the decision bar.
 *
 * @module components/request/solution/SolutionPanel
 */

import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useApprovals } from '../../../hooks/useApprovals'
import { useRequestActions } from '../../../hooks/useRequestActions'
import { useRequestSubscription } from '../../../hooks/useRequestSubscription'
import { useSolutions } from '../../../hooks/useSolutions'
import { REQUEST_FLOW_REGISTRY } from '../../../../../shared/request-flow-registry'
import type { OrcaRequest, Solution } from '../../../../../shared/request-types'
import { requestErrorMessage } from '../request-error-message'
import { SolutionBody } from './SolutionBody'
import { SolutionDecisionBar } from './SolutionDecisionBar'
import { SolutionGenerationState } from './SolutionGenerationState'
import { SolutionStatusBanner } from './SolutionStatusBanner'
import { SolutionVersionSwitcher } from './SolutionVersionSwitcher'
import { getSolutionPresentation, pickPendingApproval } from './solution-view-model'
import { useSolutionDecision } from './useSolutionDecision'

const REFETCH_EVENT_RE = /(?:^|\.)(?:solution\.[a-z_]+|approval\.[a-z_]+|request\.status_changed)$/
const RATE_LIMIT_COOLDOWN_MS = 5000

function orderNewestFirst(solutions: Solution[]): Solution[] {
  return [...solutions].sort((a, b) => {
    const byVersion = (b.version ?? 0) - (a.version ?? 0)
    if (byVersion !== 0) {return byVersion}
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
  const [cooldown, setCooldown] = useState(false)

  const refetchAll = useCallback(() => {
    refetchSolutions()
    refetchApprovals()
    onChanged()
  }, [refetchSolutions, refetchApprovals, onChanged])

  useRequestSubscription({
    requestId: request.id,
    onEvent: (event) => {
      if (REFETCH_EVENT_RE.test(event.eventType)) {
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
    solution: solution ?? ({ id: '', requestId: request.id, kind: 'unknown', status: 'unknown' } as Solution),
    approval,
    actions: {
      choose: solutionsApi.choose,
      generate: solutionsApi.generate,
      approve: approvalsApi.approve,
      reject: approvalsApi.reject
    },
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
    if (!failure) {return undefined}
    if (failure.errorClass === 'forbidden' || failure.errorClass === 'notApprover') {
      toast.error(requestErrorMessage('forbidden'))
    } else if (failure.errorClass === 'rateLimited') {
      toast.error(requestErrorMessage('rate_limited'))
      setCooldown(true)
      const t = setTimeout(() => setCooldown(false), RATE_LIMIT_COOLDOWN_MS)
      return () => clearTimeout(t)
    }
    return undefined
  }, [failure])

  const flow = request.type === 'unknown' ? undefined : REQUEST_FLOW_REGISTRY[request.type]
  const canGeneratePlan = !!flow && flow.plan !== 'none' && !request.planTaskId
  const generatePlan = (): void => {
    void requestActions.generatePlan(request.id).then((res) => {
      if (!res.ok) {toast.error(requestErrorMessage(res.error.kind))}
      else {refetchAll()}
    })
  }

  // ---- load / empty / error states ----
  if (solutionsApi.isLoading && solutionsApi.solutions.length === 0) {
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
            ? translate('auto.components.request.SolutionPanel.forbidden', 'You do not have permission to view the analysis.')
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
        {failure ? (
          <SolutionGenerationState
            variant="failed"
            errorKind={failure.error.kind}
            onRegenerate={() => void decision.regenerate()}
            regenerateDisabled={decision.submitting || cooldown}
          />
        ) : request.status === 'analyzing' ? (
          <SolutionGenerationState variant="analyzing" />
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

  return (
    <div data-testid="request-tab-analysis" className="flex h-full min-h-0 flex-col">
      <div className="flex-1 space-y-4 overflow-y-auto p-4 scrollbar-sleek">
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
            selectedId={selectedId ?? (solution.status === 'chosen' ? (solution.chosenOptionId ?? null) : null)}
            onSelect={setSelectedId}
            readOnly={readOnly}
            onRegenerate={() => void decision.regenerate()}
            regenerateDisabled={decision.submitting || cooldown}
          />
        )}
      </div>
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
