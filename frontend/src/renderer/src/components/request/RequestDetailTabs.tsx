/**
 * RequestDetailTabs — FE-OR-T101
 *
 * The tab strip and the tab bodies of the request detail: Overview, Analysis, Plan, Execution, Outcome review, History, Related.
 * Split from RequestDetailPane so the pane stays under the line limit; nothing about what the tabs show changed.
 *
 * @module components/request/RequestDetailTabs
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { UseExecutionSummaryResult } from '../../hooks/useExecutionSummary'
import type { UseOutcomeReviewResult } from '../../hooks/useOutcomeReview'
import type { RequestFlowView } from '../../../../shared/request-execution-types'
import type {
  OrcaRequest,
  RequestLink,
  RequestTypeHistoryEntry
} from '../../../../shared/request-types'
import { RequestActivityTab } from './activity/RequestActivityTab'
import { ExecutionTab } from './execution/ExecutionTab'
import { OutcomeReviewTab } from './outcome-review/OutcomeReviewTab'
import { RequestPlanTab } from './plan/RequestPlanTab'
import { RequestOverviewTab } from './RequestOverviewTab'
import { RequestRelatedTab } from './RequestRelatedTab'
import { RequestAnalysisTab } from './solution/RequestAnalysisTab'

const T = 'auto.components.request.RequestDetailPane.'

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

export type RequestDetailTabsProps = {
  tab: string
  onTabChange: (tab: string) => void
  request: OrcaRequest
  flow: RequestFlowView | null
  /** Which of the optional tabs the flow has. */
  tabs: { analysis: boolean; plan: boolean }
  showExecution: boolean
  /** The Outcome Review tab is offered: the project has it on and the request has a plan. */
  showReview: boolean
  review: UseOutcomeReviewResult
  execution: UseExecutionSummaryResult
  busy: boolean
  planNeedsAttention: boolean
  executionNeedsAttention: boolean
  reviewNeedsAttention: boolean
  history: RequestTypeHistoryEntry[]
  links: RequestLink[]
  linksSupported: boolean
  isLoading: boolean
  error: string | null
  onChanged: () => void
  onReopen: () => void
  onStop: () => void
  onAnswerClarification: () => void
}

export function RequestDetailTabs({
  tab,
  onTabChange,
  request,
  flow,
  tabs,
  showExecution,
  showReview,
  review,
  execution,
  busy,
  planNeedsAttention,
  executionNeedsAttention,
  reviewNeedsAttention,
  history,
  links,
  linksSupported,
  isLoading,
  error,
  onChanged,
  onReopen,
  onStop,
  onAnswerClarification
}: RequestDetailTabsProps): React.JSX.Element {
  const attention = translate(`${T}tab.needsAttention`, 'Needs attention')
  return (
    <Tabs value={tab} onValueChange={onTabChange} className="mt-3">
      <TabsList className="mx-4">
        <TabsTrigger value="overview">{translate(`${T}tab.overview`, 'Overview')}</TabsTrigger>
        {tabs.analysis && (
          <TabsTrigger value="analysis">{translate(`${T}tab.analysis`, 'Analysis')}</TabsTrigger>
        )}
        {tabs.plan && (
          <TabsTrigger value="plan">
            {translate(`${T}tab.plan`, 'Plan')}
            {planNeedsAttention && <TabDot label={attention} />}
          </TabsTrigger>
        )}
        {showExecution && (
          <TabsTrigger value="execution">
            {translate(`${T}tab.execution`, 'Execution')}
            {executionNeedsAttention && <TabDot label={attention} />}
          </TabsTrigger>
        )}
        {showReview && (
          <TabsTrigger value="outcome-review">
            {translate(`${T}tab.outcomeReview`, 'Outcome review')}
            {reviewNeedsAttention && <TabDot label={attention} />}
          </TabsTrigger>
        )}
        <TabsTrigger value="history">{translate(`${T}tab.history`, 'History')}</TabsTrigger>
        <TabsTrigger value="related">{translate(`${T}tab.related`, 'Related')}</TabsTrigger>
      </TabsList>
      <TabsContent value="overview">
        <RequestOverviewTab request={request} onChanged={onChanged} />
      </TabsContent>
      {tabs.analysis && (
        <TabsContent value="analysis">
          <RequestAnalysisTab request={request} onChanged={onChanged} />
        </TabsContent>
      )}
      {tabs.plan && (
        <TabsContent value="plan">
          <RequestPlanTab
            request={request}
            onChanged={onChanged}
            flow={flow}
            onOpenExecution={showExecution ? () => onTabChange('execution') : undefined}
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
            onChanged={onChanged}
            onOpenPlan={tabs.plan ? () => onTabChange('plan') : undefined}
            onReopen={onReopen}
            onStop={onStop}
            onAnswerClarification={onAnswerClarification}
          />
        </TabsContent>
      )}
      {showReview && (
        <TabsContent value="outcome-review">
          <OutcomeReviewTab request={request} review={review} summary={execution.summary} />
        </TabsContent>
      )}
      <TabsContent value="history">
        <RequestActivityTab
          requestId={request.id}
          onOpenTab={onTabChange}
          active={tab === 'history'}
          typeHistory={history}
          typeHistoryLoading={isLoading}
          typeHistoryError={error}
          onRetryTypeHistory={onChanged}
        />
      </TabsContent>
      <TabsContent value="related">
        <RequestRelatedTab requestId={request.id} links={links} linksSupported={linksSupported} />
      </TabsContent>
    </Tabs>
  )
}
