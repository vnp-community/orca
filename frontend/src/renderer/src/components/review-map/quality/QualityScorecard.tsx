/**
 * QualityScorecard.tsx — FE-CV-TASK-087-05
 *
 * The gate scorecard: verdict, per-check reasons, severity totals, steps that ran, provenance
 * and local-versus-CI. There is no single score and no "safe" wording; an unknown verdict
 * says what is missing instead of looking like a pass.
 *
 * @module components/review-map/quality/QualityScorecard
 */

import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import type {
  QualityGateResponse,
  QualityRun
} from '../../../../../shared/code-intel-quality-types'
import { StackedSeverityBar } from '../../quality-charts/StackedSeverityBar'
import { QualityCiComparisonRow } from './QualityCiComparisonRow'
import { QualityGateReasonRow } from './QualityGateReasonRow'
import type { ReasonFilter } from './QualityGateReasonRow'
import { QualityGateVerdictHeader } from './QualityGateVerdictHeader'
import { QualityProvenanceLine } from './QualityProvenanceLine'
import { QualityStepList } from './QualityStepList'
import { indexDiffersFromHead, isGateStale } from './quality-stale-model'
import { summarizeRuns } from './quality-scorecard-model'
import { scorecardCopy } from './quality-scorecard-copy'

export type QualityScorecardProps = {
  response: QualityGateResponse
  /** Runs known for the worktree; the ones behind the gate are picked by id. */
  runs: readonly QualityRun[] | null
  cacheStale: boolean
  currentHead?: string | null
  runLocked?: boolean
  onRerun?: () => void
  onShowFindings?: (filter: ReasonFilter | null) => void
}

export function QualityScorecard({
  response,
  runs,
  cacheStale,
  currentHead,
  runLocked,
  onRerun,
  onShowFindings
}: QualityScorecardProps): React.JSX.Element {
  const { gate, comparison } = response
  const behind = useMemo(() => {
    const ids = new Set(gate.basedOn.runIds)
    return (runs ?? []).filter((r) => ids.has(r.id))
  }, [gate.basedOn.runIds, runs])
  const summary = useMemo(() => summarizeRuns(behind), [behind])
  const staleness = isGateStale({ gate, cacheStale, runs, currentHead })
  const indexBehind = indexDiffersFromHead(gate, currentHead)

  return (
    <section
      className="flex flex-col gap-3 rounded-lg border border-border bg-card p-3"
      data-testid="quality-scorecard"
    >
      <QualityGateVerdictHeader gate={gate} stale={staleness.stale} />
      <QualityProvenanceLine
        summary={summary}
        staleness={staleness}
        currentHead={currentHead}
        indexBehindHead={indexBehind}
        onRerun={onRerun}
        runLocked={runLocked}
      />
      {summary.ran ? (
        <StackedSeverityBar
          frame={{
            id: 'quality-scorecard-counts',
            title: scorecardCopy('counts.title'),
            description: scorecardCopy('counts.description')
          }}
          counts={summary.counts}
          ranCheck={summary.steps.every((s) => !s.incomplete)}
        />
      ) : null}
      <div>
        <h3 className="text-xs font-medium text-foreground">{scorecardCopy('reasons.title')}</h3>
        {gate.reasons.length === 0 ? (
          <p className="py-1 text-xs text-muted-foreground">{scorecardCopy('reasons.none')}</p>
        ) : (
          <ul className="divide-y divide-border">
            {gate.reasons.map((reason, i) => (
              <QualityGateReasonRow
                key={`${reason.check}:${reason.code ?? ''}:${i}`}
                reason={reason}
                onShowFindings={onShowFindings}
              />
            ))}
          </ul>
        )}
      </div>
      {summary.ran ? <QualityStepList steps={summary.steps} /> : null}
      {comparison.length > 0 ? (
        <div>
          <h3 className="text-xs font-medium text-foreground">{scorecardCopy('ci.title')}</h3>
          <ul className="divide-y divide-border">
            {comparison.map((c, i) => (
              <QualityCiComparisonRow key={`${c.profile}:${c.ci.runId ?? i}`} comparison={c} />
            ))}
          </ul>
        </div>
      ) : null}
      {onShowFindings ? (
        <div>
          <Button type="button" variant="outline" size="xs" onClick={() => onShowFindings(null)}>
            {scorecardCopy('lens.findingsLinkNone')}
          </Button>
        </div>
      ) : null}
    </section>
  )
}
