/**
 * OutcomeReviewTab — FE-OR-T120, T122, T123, T201, T302, T322
 *
 * The high-level review of a Request: what it changes in the architecture and the flows, against the project's design. Opening
 * the tab only reads; an agent run starts when a person presses a button. Before the code exists it shows the design alone; once
 * Execution is over it adds what the code did and the decisions that follow.
 *
 * @module components/request/outcome-review/OutcomeReviewTab
 */

import React, { useCallback, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useChildRequestStatuses } from '../../../hooks/useChildRequestStatuses'
import { OutcomeAgentRunFacts } from './OutcomeAgentRunFacts'
import { OutcomeDesignControl } from './OutcomeDesignControl'
import { OutcomeNeedsDecision } from './OutcomeNeedsDecision'
import { OutcomeReleaseReadiness } from './OutcomeReleaseReadiness'
import { OutcomeReviewHeader } from './OutcomeReviewHeader'
import { OutcomeVerdictCards } from './OutcomeVerdictCards'
import {
  GeneratingLine,
  PendingBlock,
  ReviewBanners,
  ReviewError,
  ReviewInvite,
  ReviewLoading,
  ReviewUnavailable
} from './OutcomeReviewStates'
import { RegeneratePanel } from './artifact/RegeneratePanel'
import type { RegeneratePreset } from './artifact/ReviewArtifactFrame'
import { OutcomeCodeReviewButton } from './decision/OutcomeCodeReviewButton'
import { OutcomeDecisionBar } from './decision/OutcomeDecisionBar'
import { OutcomeDecisionCard } from './decision/OutcomeDecisionCard'
import { OutcomeFollowUpBanner } from './decision/OutcomeFollowUpBanner'
import { OutcomeSendBackDialog } from './decision/OutcomeSendBackDialog'
import { lensForItem, CONTAINER_LENS } from './decision/outcome-code-review-link'
import { useOutcomeDecisions } from './decision/use-outcome-decisions'
import { useOutcomeReviewLinks } from './decision/use-outcome-review-links'
import { OutcomeSystemDiff } from './diff/OutcomeSystemDiff'
import { errorText } from './outcome-review-state-copy'
import {
  bannersOf,
  needsDecisionItems,
  phaseOf,
  phaseOfRequest,
  screenOf
} from './outcome-review-view-model'
import type { UseOutcomeReviewResult } from '../../../hooks/useOutcomeReview'
import type { ReviewSection } from '../../../../../shared/outcome-review-types'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../../shared/request-types'

export type OutcomeReviewTabProps = {
  request: OrcaRequest
  review: UseOutcomeReviewResult
  summary: ExecutionSummary | null
}

export function OutcomeReviewTab({
  request,
  review: hook,
  summary
}: OutcomeReviewTabProps): React.JSX.Element {
  const { status, review, estimate, runs, activeRun, error, generating } = hook
  const screen = screenOf({
    status,
    review,
    estimate,
    requestStatus: request.status,
    activeRun,
    runs
  })
  const delivered = !!review && phaseOf(review) === 'delivered'
  const currentUser = useAppStore((st) => st.currentUser)
  // Why: the server decides again; this only hides the buttons from people it would refuse (the reporter and administrators act).
  const canAct =
    currentUser?.role === 'admin' || (!!currentUser?.id && currentUser.id === request.reporterId)
  const [section, setSection] = useState<ReviewSection>('architecture')
  const [panel, setPanel] = useState<RegeneratePreset | null>(null)
  const [sendingBack, setSendingBack] = useState(false)
  const diffRef = useRef<HTMLDivElement>(null)
  const itemRefs = useRef(new Map<string, HTMLElement>())

  const decisions = useOutcomeDecisions({
    requestId: request.id,
    review,
    enabled: delivered && screen === 'review',
    onChanged: hook.refresh
  })
  const { link, openFile } = useOutcomeReviewLinks(
    request,
    review,
    delivered && screen === 'review'
  )
  const sentBack = useMemo(
    () => [...decisions.inForce.values()].filter((d) => d.decision === 'send_back'),
    [decisions.inForce]
  )
  const childIds = useMemo(
    () => [...new Set(sentBack.map((d) => d.childRequestId).filter((id): id is string => !!id))],
    [sentBack]
  )
  const childStatus = useChildRequestStatuses(childIds)
  const followUpDone = childIds.some((id) => childStatus[id] === 'completed')

  const openSection = useCallback((next: ReviewSection) => {
    setSection(next)
    diffRef.current?.scrollIntoView?.({ block: 'start' })
  }, [])
  const generate = useCallback(
    async (runContext: boolean) => {
      const res = await hook.generate({ runContext })
      if (!res.ok) {
        toast.error(errorText(res.error.code))
      }
    },
    [hook]
  )
  const openChild = useCallback(
    (id: string) => useAppStore.getState().setRequestPageRequest(id),
    []
  )
  const showItem = useCallback(
    (key: string) => itemRefs.current.get(key)?.scrollIntoView?.({ block: 'center' }),
    []
  )

  if (screen === 'loading') {
    return <ReviewLoading />
  }
  if (screen === 'unavailable') {
    return <ReviewUnavailable />
  }
  if (screen === 'error') {
    return <ReviewError code={error?.code ?? ''} onRetry={hook.refresh} />
  }
  if (screen === 'invite_design' || screen === 'invite_delivered') {
    return (
      <div className="flex flex-col gap-3 px-4 py-3" data-testid="outcome-review-tab">
        <GeneratingLine phase={phaseOfRequest(request.status)} active={generating} />
        <ReviewInvite
          kind={screen === 'invite_design' ? 'design' : 'delivered'}
          estimate={estimate}
          generating={generating}
          onGenerate={(runContext) => void generate(runContext)}
        />
      </div>
    )
  }
  if (!review) {
    return <ReviewLoading />
  }

  const phase = phaseOf(review)
  const waitingForContext = activeRun?.kind === 'context'
  const banners = bannersOf({
    status,
    review,
    estimate,
    requestStatus: request.status,
    activeRun,
    runs
  })
  const candidates = review.model.items.filter(
    (i) =>
      (i.needsDecision && !i.advisory) || decisions.inForce.get(i.key)?.decision === 'send_back'
  )
  return (
    <div
      className="flex flex-col gap-3 px-4 py-3"
      data-testid="outcome-review-tab"
      data-phase={phase}
    >
      <OutcomeReviewHeader
        review={review}
        onRegenerate={canAct ? () => setPanel({ sections: [] }) : undefined}
      />
      <GeneratingLine
        phase={phaseOfRequest(request.status)}
        active={generating || activeRun !== null}
      />
      <ReviewBanners
        ids={banners}
        review={review}
        runs={runs}
        activeRun={activeRun}
        busy={generating}
        onGenerate={canAct ? () => void generate(true) : undefined}
        onRegenerate={canAct ? () => void generate(true) : undefined}
      />
      {followUpDone && canAct && (
        <OutcomeFollowUpBanner busy={generating} onRegenerate={() => setPanel({ sections: [] })} />
      )}
      {waitingForContext ? (
        <PendingBlock
          label={translate(
            'auto.components.request.outcomeReview.OutcomeReviewTab.pending',
            'The architecture and flow are being read'
          )}
        />
      ) : (
        <OutcomeDesignControl review={review} onOpenSection={openSection} />
      )}
      {delivered && <OutcomeVerdictCards review={review} />}
      <OutcomeNeedsDecision
        items={needsDecisionItems(review)}
        later={!delivered}
        renderActions={(item) => (
          <div
            ref={(el) => {
              if (el) {
                itemRefs.current.set(item.key, el)
              }
            }}
          >
            {canAct ? (
              <OutcomeDecisionCard
                item={item}
                decision={decisions.inForce.get(item.key) ?? null}
                currentUserId={currentUser?.id ?? null}
                busy={decisions.busy}
                onDecide={(value, note) => decisions.decide(item.key, value, note)}
                onOpenChild={openChild}
              />
            ) : null}
          </div>
        )}
      />
      <div ref={diffRef}>
        <OutcomeSystemDiff
          requestId={request.id}
          review={review}
          requestStatus={request.status}
          section={section}
          onSectionChange={setSection}
          canRegenerate={canAct}
          onRegenerate={canAct ? setPanel : undefined}
          onChanged={hook.refresh}
          onOpenFile={openFile}
          renderRowAction={
            delivered
              ? (item) => <OutcomeCodeReviewButton lens={lensForItem(item)} link={link} />
              : undefined
          }
          renderContainerAction={
            delivered
              ? () => <OutcomeCodeReviewButton lens={CONTAINER_LENS} link={link} />
              : undefined
          }
        />
      </div>
      {delivered && <OutcomeReleaseReadiness review={review} />}
      {delivered && <OutcomeAgentRunFacts runs={runs} summary={summary} />}
      {delivered && canAct && (
        <OutcomeDecisionBar
          review={review}
          openKeys={decisions.openKeys}
          sentBackCount={sentBack.length}
          currentUserId={currentUser?.id ?? null}
          busy={decisions.busy}
          onApprove={decisions.approve}
          onRequestChanges={() => setSendingBack(true)}
          onShowItem={showItem}
        />
      )}
      {panel && (
        <RegeneratePanel
          requestId={request.id}
          reviewVersion={review.version}
          preset={panel}
          onClose={() => setPanel(null)}
          onStarted={hook.refresh}
        />
      )}
      {sendingBack && (
        <OutcomeSendBackDialog
          request={request}
          review={review}
          candidates={candidates}
          sentBack={new Set(sentBack.map((d) => d.itemKey))}
          decide={(key, note) => decisions.decide(key, 'send_back', note)}
          linkChild={decisions.linkChild}
          onClose={() => setSendingBack(false)}
          onChanged={hook.refresh}
        />
      )}
    </div>
  )
}
