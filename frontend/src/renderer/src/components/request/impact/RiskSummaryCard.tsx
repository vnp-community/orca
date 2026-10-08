/**
 * RiskSummaryCard — FE-REQ-TASK-036-05
 *
 * Never renders "Low" without an assessment; shadow mode is labelled advisory.
 *
 * @module components/request/impact/RiskSummaryCard
 */

import React, { useEffect, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Skeleton } from '@/components/ui/skeleton'
import { RiskBadge } from '../../graph/RiskBadge'
import { useImpactAssessment, type ImpactAssessmentApi } from '../../../hooks/useImpactAssessment'
import { summarizeCard } from './impact-dimension-model'

const T = 'auto.components.request.impact.'
const SKELETON_DELAY_MS = 200

type Props = {
  requestId?: string | null
  subjectType: string
  subjectId: string
  solutionId?: string
  compact?: boolean
  onConnectDevServer?: () => void
  /** Share a parent's assessment instead of fetching again. */
  assessment?: ImpactAssessmentApi
}

export function RiskSummaryCard({ requestId, subjectType, subjectId, solutionId, compact, onConnectDevServer, assessment }: Props): React.JSX.Element {
  const own = useImpactAssessment({ requestId, subjectType, subjectId, solutionId, enabled: !assessment })
  const impact = assessment ?? own
  const card = summarizeCard(impact.summary)
  const loading = impact.status === 'loading' || impact.status === 'collecting'
  const [showSkeleton, setShowSkeleton] = useState(false)

  // Why: delay the skeleton so fast (local) responses do not flash it over SSH-slow ones.
  useEffect(() => {
    if (!loading) {
      setShowSkeleton(false)
      return
    }
    const t = setTimeout(() => setShowSkeleton(true), SKELETON_DELAY_MS)
    return () => clearTimeout(t)
  }, [loading])

  let body: React.ReactNode
  if (impact.status === 'forbidden') {
    body = <p className="text-xs text-muted-foreground">{translate(`${T}forbidden`, 'You do not have permission to view the assessment')}</p>
  } else if (impact.status === 'noDevServer') {
    body = (
      <p className="flex items-center gap-2 text-xs text-muted-foreground">
        {translate(`${T}noDevServer`, 'No dev server connected')}
        {onConnectDevServer ? <Button size="xs" variant="outline" onClick={onConnectDevServer}>{translate(`${T}connectDevServer`, 'Connect')}</Button> : null}
      </p>
    )
  } else if (loading) {
    body = showSkeleton ? (
      <div className="flex flex-col gap-1" role="status" aria-label={translate(`${T}collecting`, 'Querying the code graph')}>
        <Skeleton className="h-5 w-24" />
        <span className="text-xs text-muted-foreground">{translate(`${T}collecting`, 'Querying the code graph')}</span>
      </div>
    ) : null
  } else if (!card || impact.status === 'unsupported' || impact.status === 'idle') {
    body = (
      <div className="flex items-center gap-2">
        <RiskBadge level="unknown" size="sm" />
        {impact.status === 'ready' && !card && subjectId ? (
          <Button size="xs" variant="outline" onClick={() => void impact.request()}>{translate(`${T}run`, 'Run assessment')}</Button>
        ) : null}
      </div>
    )
  } else {
    body = (
      <div className="flex flex-col gap-1.5">
        <div className="flex flex-wrap items-center gap-2">
          <RiskBadge level={card.level} assessedAt={card.caption.assessedAt} tool={card.caption.tool} />
          {card.scoreText ? <span className="text-xs tabular-nums">{translate(`${T}score`, 'Score {{score}}', { score: card.scoreText })}</span> : null}
          {card.advisory ? <span className="rounded border border-border px-1.5 text-[10px] text-muted-foreground">{translate(`${T}advisory`, 'Advisory')}</span> : null}
        </div>
        {!compact && card.reasons.length > 0 ? (
          <ul className="list-disc pl-5 text-xs">{card.reasons.map((r, i) => <li key={i}>{r}</li>)}</ul>
        ) : null}
        {card.hardRules.length > 0 ? <p className="text-xs">{translate(`${T}raisedBy`, 'Raised because: {{rules}}', { rules: card.hardRules.join(', ') })}</p> : null}
        <p className="text-[11px] text-muted-foreground">
          {card.caption.assessedAt
            ? translate(`${T}caption`, 'Assessed {{at}}, based on {{tool}}', { at: card.caption.assessedAt, tool: card.caption.tool ?? '-' })
            : null}
          {card.caption.confidence ? ` · ${translate(`${T}confidence.${card.caption.confidence}`, card.caption.confidence)}` : ''}
        </p>
        {card.stale ? <p className="text-[11px] text-muted-foreground">{translate(`${T}stale`, 'Index is out of date; not fully assessed')}</p> : null}
        {!compact && impact.summary?.narrative ? (
          <Collapsible>
            <CollapsibleTrigger className="text-xs underline-offset-2 hover:underline">{translate(`${T}narrative`, 'AI-written explanation')}</CollapsibleTrigger>
            <CollapsibleContent className="pt-1 text-xs whitespace-pre-wrap text-muted-foreground">{impact.summary.narrative}</CollapsibleContent>
          </Collapsible>
        ) : null}
      </div>
    )
  }

  return (
    <section aria-label={translate(`${T}title`, 'Impact and risk')} data-testid="risk-summary-card" className="rounded-md border border-border px-3 py-2">
      {body}
    </section>
  )
}
