/**
 * QualityTrendPanel.tsx — FE-CV-TASK-087-17
 *
 * Findings per turn (or commit) as lines, a ◆ where the gate verdict changed, a diff coverage
 * sparkline and a per-point table that names the source (local/CI) in words and glyphs.
 * States numbers only; it never calls a trend "improved" or "worse".
 *
 * @module components/review-map/quality/QualityTrendPanel
 */

import { useMemo, useState } from 'react'
import { useQualityTrend } from '@/hooks/useQualityTrend'
import { Button } from '../../ui/button'
import { SparklineChart } from '../../quality-charts/SparklineChart'
import { TrendLineChart } from '../../quality-charts/TrendLineChart'
import { chartCopy } from '../../quality-charts/quality-chart-copy'
import type { QualityTrendGroup } from '../../../store/slices/code-intel-quality-state-types'
import { describeQualityBlockError } from './quality-block-error-message'
import { QualityBlockStateFrame } from './QualityBlockStateFrame'
import type { QualityBlockProps } from './quality-lens-block-types'
import { QualityTrendPointTable } from './QualityTrendPointTable'
import { buildQualityTrendModel, shortCommit, sourceKind } from './quality-trend-series'
import { qv } from './quality-visualization-copy'

export type QualityTrendPanelProps = QualityBlockProps & {
  /** turnKey -> label of a known review turn marker; only these points can open a comparison. */
  turnLabels?: ReadonlyMap<string, string>
  /** The review turn comparison of FE-CV-SOL-060; when absent, points are not selectable. */
  onCompareTurn?: (turnKey: string) => void
}

export function QualityTrendPanel({
  worktreeId,
  turnLabels,
  onCompareTurn
}: QualityTrendPanelProps): React.JSX.Element | null {
  const [groupBy, setGroupBy] = useState<QualityTrendGroup>('turn')
  const trend = useQualityTrend(worktreeId, groupBy)
  const model = useMemo(
    () => buildQualityTrendModel(trend.points, { turnLabels }),
    [trend.points, turnLabels]
  )
  if (trend.support === 'disabled' || trend.support === 'unsupported') {
    return null
  }
  const title = groupBy === 'turn' ? qv('trend.title') : qv('trend.titleCommit')
  const frameBase = { id: 'quality-trend', title, description: qv('trend.description') }
  const { status } = trend

  const toggle = (
    <div role="group" aria-label={qv('trend.groupBy')} className="flex items-center gap-1 text-xs">
      <span className="text-muted-foreground">{qv('trend.groupBy')}</span>
      {(['turn', 'commit'] as const).map((g) => (
        <Button
          key={g}
          type="button"
          size="sm"
          variant={groupBy === g ? 'secondary' : 'outline'}
          aria-pressed={groupBy === g}
          onClick={() => setGroupBy(g)}
        >
          {g === 'turn' ? qv('trend.groupTurn') : qv('trend.groupCommit')}
        </Button>
      ))}
    </div>
  )

  let body: React.ReactNode
  if (status === 'idle' || status === 'loading') {
    body = <QualityBlockStateFrame {...frameBase} status="loading" />
  } else if (status === 'error') {
    body = (
      <QualityBlockStateFrame
        {...frameBase}
        status="error"
        errorMessage={describeQualityBlockError(trend.error)}
        onRetry={trend.refetch}
      />
    )
  } else if (status === 'unavailable') {
    body = (
      <QualityBlockStateFrame {...frameBase} status="empty" emptyReason={qv('block.unavailable')} />
    )
  } else if (model.points.length === 0) {
    body = <QualityBlockStateFrame {...frameBase} status="empty" emptyReason={qv('trend.empty')} />
  } else if (model.points.length < 2) {
    const only = model.points[0]
    body = (
      <QualityBlockStateFrame
        {...frameBase}
        status="empty"
        emptyReason={
          <span className="flex flex-col gap-1" data-trend-need-two>
            <span>{qv('trend.needTwo')}</span>
            <span className="tabular-nums">
              {qv('trend.latest', {
                error: only.counts.error,
                warning: only.counts.warning,
                info: only.counts.info
              })}
            </span>
          </span>
        }
      />
    )
  } else {
    const total = Math.max(trend.totalCount, model.points.length)
    const interactive = Boolean(onCompareTurn)
    const missing = model.diffCoverageSpark.filter((v) => v === null).length
    const last = model.points.at(-1)!
    const lastSource = sourceKind(last.source)
    const sourceWord =
      lastSource === 'local'
        ? qv('trend.sourceLocal')
        : lastSource === 'ci'
          ? qv('trend.sourceCi')
          : qv('trend.sourceUnknown')
    body = (
      <div className="flex flex-col gap-3" data-status={status}>
        {status === 'stale' ? (
          <p className="text-xs text-muted-foreground" data-chart-stale>
            {qv('block.stale')}
          </p>
        ) : null}
        <TrendLineChart
          frame={{
            id: 'quality-trend-chart',
            title,
            description: interactive ? qv('trend.clickHint') : qv('trend.description')
          }}
          series={model.series.map((s) => ({
            ...s,
            label: chartCopy(`severity.${s.id as 'error' | 'warning' | 'info'}`)
          }))}
          xLabels={model.xLabels}
          maxPoints={model.xLabels.length}
          onSelectPoint={
            onCompareTurn
              ? (index) => {
                  const key = model.points[index]?.turnKey
                  // Why: only turns the review knows can be compared; other points stay inert.
                  if (key && turnLabels?.has(key)) {
                    onCompareTurn(key)
                  }
                }
              : undefined
          }
        />
        {trend.truncated || total > model.points.length ? (
          <p className="text-xs text-muted-foreground">
            {chartCopy('frame.showing', { shown: model.points.length, total })}
          </p>
        ) : null}
        <div className="flex flex-col gap-1">
          <SparklineChart label={qv('trend.diffCoverage')} points={model.diffCoverageSpark} />
          {missing > 0 ? (
            <p className="text-xs text-muted-foreground">
              {qv('trend.diffCoverageMissing', { count: missing })}
            </p>
          ) : null}
        </div>
        <p className="text-xs text-foreground">
          {qv('trend.headNote', { commit: shortCommit(last.headCommit), source: sourceWord })}
        </p>
        <QualityTrendPointTable model={model} />
      </div>
    )
  }

  return (
    <section aria-label={title} className="flex flex-col gap-2" data-quality-trend>
      {toggle}
      {body}
    </section>
  )
}

export default QualityTrendPanel
