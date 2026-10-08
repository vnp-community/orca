/**
 * SolutionDecisionBar — CR-REQ-020-04
 *
 * Sticky bottom bar. No keyboard shortcut on Approve (prevents accidental approval).
 *
 * @module components/request/solution/SolutionDecisionBar
 */

import React, { useState } from 'react'
import { Check, Loader2, RefreshCw, X } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { useDecisions } from '../../../hooks/useDecisions'
import { useImpactAssessment } from '../../../hooks/useImpactAssessment'
import { splitErrorCode } from '../../../../../shared/request-errors'
import { ImpactFindingList } from '../impact/ImpactFindingList'
import { RiskAcceptanceChecklist } from '../impact/RiskAcceptanceChecklist'
import { RiskOverrideMenu } from '../impact/RiskOverrideMenu'
import { RiskSummaryCard } from '../impact/RiskSummaryCard'
import { useRiskApprovalGate } from '../impact/useRiskApprovalGate'
import { DecisionHistoryList } from '../decision/DecisionHistoryList'
import { DecisionRationaleField } from '../decision/DecisionRationaleField'
import { HighRiskDecisionConfirmDialog } from '../decision/HighRiskDecisionConfirmDialog'
import { getDecisionGate, isRationaleValid, requiresRationale } from '../decision/decision-rules'
import type { Decision } from '../../../../../shared/request-artifact-types'
import type { Approval, RequestType, Solution } from '../../../../../shared/request-types'
import { requestErrorMessage } from '../request-error-message'
import { RejectReasonDialog } from './RejectReasonDialog'
import { canApproveSolution } from './solution-view-model'
import type { SolutionPresentation } from './solution-view-model'
import type { SolutionDecision } from './useSolutionDecision'

export type SolutionDecisionBarProps = {
  solution: Solution
  requestType: RequestType
  presentation: SolutionPresentation
  /** Pending approval for this solution, if any. */
  approval: Approval | null
  /** An approval for this solution that expired (only regenerate is allowed). */
  expiredApproval?: boolean
  selectedId: string | null
  decision: SolutionDecision
  onGeneratePlan?: () => void
  /** Briefly true after REQUEST_RATE_LIMITED so regenerate cannot be hammered. */
  regenerateCooldown?: boolean
}

export function SolutionDecisionBar({
  solution,
  requestType,
  presentation,
  approval,
  expiredApproval = false,
  selectedId,
  decision,
  onGeneratePlan,
  regenerateCooldown = false
}: SolutionDecisionBarProps): React.JSX.Element | null {
  const [rejectOpen, setRejectOpen] = useState(false)
  const [rationale, setRationale] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [pendingDecision, setPendingDecision] = useState<Decision | null>(null)
  const decisions = useDecisions(solution.requestId, solution.id)
  const impactSubject = selectedId ?? solution.chosenOptionId ?? ''
  const impact = useImpactAssessment({
    requestId: solution.requestId,
    subjectType: 'solution_option',
    subjectId: impactSubject,
    solutionId: solution.id,
    enabled: solution.kind === 'solution' && impactSubject !== ''
  })
  const riskGate = useRiskApprovalGate(impact)
  const earlyFailure = decision.failure
  const earlyFailureCode = earlyFailure ? splitErrorCode(earlyFailure.error.message).code || earlyFailure.error.code : ''
  // Why: a missing/stale/pending acceptance means our local view of the assessment is outdated.
  const { refetch: refetchImpact } = impact
  React.useEffect(() => {
    if (['REQUEST_RISK_ACCEPTANCE_REQUIRED', 'REQUEST_RISK_ASSESSMENT_STALE', 'REQUEST_RISK_ASSESSMENT_PENDING'].includes(earlyFailureCode)) {
      refetchImpact()
    }
  }, [earlyFailureCode, refetchImpact])
  const { actions } = presentation
  if (presentation.readOnly || actions.length === 0) {return null}

  const denied = decision.isDenied(approval?.id)
  const gate = canApproveSolution({
    kind: solution.kind,
    requestType,
    options: solution.options,
    chosenOptionId: selectedId ?? solution.chosenOptionId
  })
  const busy = decision.submitting
  const approveLabel =
    solution.kind === 'answer'
      ? translate('auto.components.request.SolutionDecisionBar.accept', 'Accept')
      : solution.kind === 'solution'
        ? translate('auto.components.request.SolutionDecisionBar.approveOption', 'Approve this option')
        : translate('auto.components.request.SolutionDecisionBar.approve', 'Approve')
  const failure = decision.failure
  const showFailure =
    failure !== null && failure.errorClass !== 'notApprover' && failure.errorClass !== 'conflict'

  const chosenId = selectedId ?? solution.chosenOptionId ?? undefined
  const failureCode = failure ? splitErrorCode(failure.error.message).code || failure.error.code : ''
  const approverNotAllowed = failureCode === 'REQUEST_RISK_APPROVER_NOT_ALLOWED'
  const reqs = riskGate.requirements
  const riskBlocked = !reqs.canApprove
  const recommendedId =
    decisions.current?.recommendedOptionId ??
    solution.options?.find((o) => o.raw?.recommended === true)?.id
  const rationaleRequired = solution.kind === 'solution' && requiresRationale(chosenId, recommendedId)
  const rationaleOk = !rationaleRequired || isRationaleValid(rationale, true)
  const activeDecision = pendingDecision ?? decisions.current
  const decisionGate = getDecisionGate(
    solution.kind === 'solution' ? decisions.current : null,
    approval,
    approval?.subjectDigest,
    { selfChoiceForbidden: failureCode === 'REQUEST_DECISION_SELF_CHOICE_FORBIDDEN' }
  )
  const optionTitle = (id: string | undefined): string =>
    solution.options?.find((o) => o.id === id)?.title ?? id ?? ''
  const waitingConfirmation = decisionGate === 'needsConfirmation' || (pendingDecision !== null && pendingDecision.status !== 'effective')
  const approve = async (): Promise<void> => {
    const outcome = await decision.approveSelected(chosenId, undefined, { rationale, ...riskGate.approveExtras })
    if (outcome.needsConfirmation && outcome.decision) {
      setPendingDecision(outcome.decision)
      setConfirmOpen(true)
      decisions.refetch()
    }
  }

  return (
    <div
      data-testid="solution-decision-bar"
      className="sticky bottom-0 flex flex-col gap-2 border-t border-border bg-background p-3"
    >
      {decision.chosenButNotApproved && (
        <p role="status" className="text-xs text-muted-foreground">
          {translate(
            'auto.components.request.SolutionDecisionBar.partial',
            'The option was chosen but not yet approved. Try approving again.'
          )}
        </p>
      )}
      {waitingConfirmation && (
        <p role="status" className="text-xs text-muted-foreground" data-testid="decision-pending-confirmation">
          {translate('auto.components.request.decision.pendingConfirmation', 'Waiting for the second confirmation of this high-risk choice')}
        </p>
      )}
      {decisionGate === 'digestChanged' && (
        <p role="alert" className="text-xs text-destructive" data-testid="decision-digest-changed">
          {translate('auto.components.request.decision.digestChanged', 'The content just changed. Review it again before approving.')}
        </p>
      )}
      {decisionGate === 'selfChoiceForbidden' && (
        <p role="status" className="text-xs text-muted-foreground">
          {translate('auto.components.request.decision.selfChoiceForbidden', 'The reporter cannot choose their own request')}
        </p>
      )}
      {showFailure && (
        <p role="alert" className="text-xs text-destructive">
          {failure.errorClass === 'alreadyDecided'
            ? translate('auto.components.request.SolutionDecisionBar.alreadyDecided', 'This was already decided.')
            : failure.errorClass === 'expired'
              ? translate('auto.components.request.SolutionDecisionBar.expired', 'Expired')
              : requestErrorMessage(failure.error.kind)}
        </p>
      )}
      {!gate.ok && gate.reasonKey === 'chooseOne' && actions.includes('approve') && !denied && (
        <p className="text-xs text-muted-foreground">
          {translate('auto.components.request.SolutionDecisionBar.chooseOne', 'Choose an option to approve.')}
        </p>
      )}
      {solution.kind === 'solution' && approval && actions.includes('approve') && chosenId && reqs.requiresView && (
        <Collapsible onOpenChange={(open) => { if (open) {riskGate.markViewed()} }}>
          <CollapsibleTrigger className="text-xs underline-offset-2 hover:underline" data-testid="view-impact-trigger">
            {translate('auto.components.request.impact.gate.viewImpact', 'View impact')}
          </CollapsibleTrigger>
          <CollapsibleContent className="flex flex-col gap-2 pt-2">
            <RiskSummaryCard assessment={impact} subjectType="solution_option" subjectId={impactSubject} solutionId={solution.id} />
            <ImpactFindingList findings={impact.findings} />
          </CollapsibleContent>
        </Collapsible>
      )}
      {solution.kind === 'solution' && approval && actions.includes('approve') && chosenId && (
        <RiskAcceptanceChecklist
          requirements={reqs}
          accepted={riskGate.state.acceptedFindingIds}
          info={riskGate.info}
          invalidated={riskGate.invalidated}
          onAccept={riskGate.accept}
          disabled={busy}
        />
      )}
      {approverNotAllowed && (
        <p role="status" className="text-xs text-muted-foreground" data-testid="risk-approver-not-allowed">
          {translate('auto.components.request.impact.gate.approverNotAllowed', 'An approver from the designated team is required')}
        </p>
      )}
      {solution.kind === 'solution' && approval && actions.includes('approve') && chosenId && decisionGate !== 'selfChoiceForbidden' && (
        <DecisionRationaleField
          value={rationale}
          onChange={setRationale}
          required={rationaleRequired}
          disabled={busy}
          onSubmit={rationaleOk && gate.ok ? () => void approve() : undefined}
        />
      )}
      <div className="flex flex-wrap items-center gap-2">
        {denied ? (
          <p className="text-sm text-muted-foreground">
            {translate('auto.components.request.SolutionDecisionBar.noPermission', 'You do not have permission to approve')}
          </p>
        ) : expiredApproval ? (
          <span className="text-sm text-muted-foreground">
            {translate('auto.components.request.SolutionDecisionBar.expired', 'Expired')}
          </span>
        ) : (
          approval &&
          actions.includes('approve') && (
            <>
              <Button
                disabled={busy || !gate.ok || !rationaleOk || riskBlocked || approverNotAllowed || decisionGate === 'digestChanged' || decisionGate === 'selfChoiceForbidden'}
                title={reqs.blockedReasonKey ? translate(reqs.blockedReasonKey, 'Review the impact before approving') : undefined}
                onClick={() => (waitingConfirmation ? setConfirmOpen(true) : void approve())}
              >
                {busy ? <Loader2 className="size-4 animate-spin" aria-hidden /> : <Check className="size-4" aria-hidden />}
                {waitingConfirmation
                  ? translate('auto.components.request.decision.confirmSubmit', 'Confirm choice')
                  : approveLabel}
              </Button>
              {actions.includes('reject') && (
                <Button variant="outline" disabled={busy} onClick={() => setRejectOpen(true)}>
                  <X className="size-4" aria-hidden />
                  {translate('auto.components.request.SolutionDecisionBar.reject', 'Reject')}
                </Button>
              )}
            </>
          )
        )}
        {approval && (
          <RiskOverrideMenu
            gate="solution"
            allowed={impact.canOverride}
            onOverride={impact.override}
          />
        )}
        {actions.includes('regenerate') && (
          <Button variant="outline" disabled={busy || regenerateCooldown} onClick={() => void decision.regenerate()}>
            <RefreshCw className="size-4" aria-hidden />
            {translate('auto.components.request.SolutionGenerationState.regenerate', 'Regenerate')}
          </Button>
        )}
        {actions.includes('generatePlan') && onGeneratePlan && (
          <Button disabled={busy} onClick={onGeneratePlan}>
            {translate('auto.components.request.SolutionDecisionBar.generatePlan', 'Generate plan')}
          </Button>
        )}
      </div>
      {solution.kind === 'solution' && decisions.decisions.length > 0 && (
        <Collapsible>
          <CollapsibleTrigger className="text-xs text-muted-foreground underline-offset-2 hover:underline">
            {translate('auto.components.request.decision.history.title', 'Decision history')}
          </CollapsibleTrigger>
          <CollapsibleContent className="pt-2">
            <DecisionHistoryList decisions={decisions.decisions} optionTitle={optionTitle} />
          </CollapsibleContent>
        </Collapsible>
      )}
      {activeDecision && (
        <HighRiskDecisionConfirmDialog
          open={confirmOpen}
          decision={activeDecision}
          optionTitle={optionTitle(activeDecision.chosenOptionId ?? chosenId)}
          reasons={activeDecision.riskReasons}
          onCancel={() => setConfirmOpen(false)}
          onConfirm={async (text) => {
            const res = await decisions.confirm(activeDecision.id, text, activeDecision.version)
            if (res.ok) {
              setConfirmOpen(false)
              setPendingDecision(null)
              void decision.approveSelected(chosenId)
            }
            return res
          }}
        />
      )}
      <RejectReasonDialog
        open={rejectOpen}
        onOpenChange={setRejectOpen}
        submitting={busy}
        offerRegenerate
        onSubmit={(comment, opts) => decision.reject(comment, opts?.regenerate ?? false)}
      />
    </div>
  )
}
