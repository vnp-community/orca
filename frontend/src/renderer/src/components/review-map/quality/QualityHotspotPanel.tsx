/**
 * QualityHotspotPanel.tsx — FE-CV-TASK-087-18
 *
 * Heatmap of hotspot findings. Columns are the metric keys the backend actually sent (the
 * contract lists none), so there is no complexity column and no total score; a missing number
 * is "—". Owner names are rendered as plain text.
 *
 * @module components/review-map/quality/QualityHotspotPanel
 */

import { useMemo, useState } from 'react'
import { useQualityHotspots } from '@/hooks/useQualityHotspots'
import { translate } from '@/i18n/i18n'
import { Button } from '../../ui/button'
import { HotspotHeatmap } from '../../quality-charts/HotspotHeatmap'
import { NO_VALUE_DASH, chartCopy } from '../../quality-charts/quality-chart-copy'
import { formatChartNumber } from '../../quality-charts/chart-text-summary'
import { describeQualityBlockError } from './quality-block-error-message'
import { QualityBlockStateFrame } from './QualityBlockStateFrame'
import { buildHotspotModel, HOTSPOT_MAX_COLUMNS } from './quality-hotspot-rows'
import type { QualityBlockProps } from './quality-lens-block-types'
import { qv } from './quality-visualization-copy'

// Why: unknown metric keys fall back to the raw key; known ones may be translated by catalog key.
function metricLabel(key: string): string {
  return translate(
    `auto.components.reviewQuality.hotspotMetric.${key.replace(/[^A-Za-z0-9]+/g, '_')}`,
    key
  )
}

export function QualityHotspotPanel({
  worktreeId,
  onOpenDiff
}: QualityBlockProps): React.JSX.Element | null {
  const hotspots = useQualityHotspots(worktreeId)
  const [selected, setSelected] = useState<string | null>(null)
  const model = useMemo(
    () => buildHotspotModel(hotspots.findings, metricLabel),
    [hotspots.findings]
  )
  if (hotspots.support === 'disabled' || hotspots.support === 'unsupported') {
    return null
  }
  const frameBase = {
    id: 'quality-hotspot',
    title: qv('hotspot.title'),
    description: qv('hotspot.description')
  }
  const { status } = hotspots
  if (status === 'idle' || status === 'loading') {
    return <QualityBlockStateFrame {...frameBase} status="loading" />
  }
  if (status === 'error') {
    return (
      <QualityBlockStateFrame
        {...frameBase}
        status="error"
        errorMessage={describeQualityBlockError(hotspots.error)}
        onRetry={hotspots.refetch}
      />
    )
  }
  if (status === 'unavailable') {
    return (
      <QualityBlockStateFrame {...frameBase} status="empty" emptyReason={qv('block.unavailable')} />
    )
  }
  if (model.rows.length === 0) {
    return (
      <QualityBlockStateFrame
        {...frameBase}
        status="empty"
        emptyReason={qv('hotspot.emptyReason')}
      />
    )
  }
  const unloaded = Math.max(0, hotspots.totalCount - hotspots.findings.length)
  const owner = selected ? model.owners[selected] : undefined
  return (
    <section
      aria-label={frameBase.title}
      className="flex flex-col gap-2"
      data-quality-hotspot
      data-status={status}
    >
      {status === 'stale' ? (
        <p className="text-xs text-muted-foreground" data-chart-stale>
          {qv('block.stale')}
        </p>
      ) : null}
      <HotspotHeatmap
        frame={{ ...frameBase, emptyReason: qv('hotspot.emptyReason') }}
        rows={model.rows}
        columns={model.columns}
        onSelectRow={setSelected}
      />
      {unloaded > 0 ? (
        <p className="text-xs text-muted-foreground">
          {qv('hotspot.moreRows', { count: unloaded })}
        </p>
      ) : null}
      {selected ? (
        <div
          className="flex flex-wrap items-center gap-2 text-xs text-foreground"
          data-hotspot-selected
        >
          <span className="font-mono">{qv('hotspot.selected', { path: selected })}</span>
          {owner ? <span>{qv('hotspot.owner', { owner })}</span> : null}
          <Button type="button" size="sm" variant="outline" onClick={() => onOpenDiff(selected)}>
            {qv('hotspot.openDiff')}
          </Button>
        </div>
      ) : null}
      {model.allKeys.length > HOTSPOT_MAX_COLUMNS ? (
        <details className="text-xs" data-hotspot-all-metrics>
          <summary className="cursor-pointer text-foreground">
            {qv('hotspot.allMetrics', { count: model.allKeys.length })}
          </summary>
          <table className="mt-1 w-full text-left">
            <thead>
              <tr className="text-muted-foreground">
                <th scope="col" className="pr-2 font-medium">
                  {qv('hotspot.colFile')}
                </th>
                {model.allKeys.map((key) => (
                  <th key={key} scope="col" className="px-1 text-right font-medium">
                    {metricLabel(key)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {model.rows.slice(0, 40).map((row) => (
                <tr key={row.id}>
                  <th scope="row" className="pr-2 font-mono font-normal">
                    {row.label}
                  </th>
                  {model.allKeys.map((key) => {
                    const value = model.allMetrics[row.id]?.[key]
                    return (
                      <td key={key} className="px-1 text-right tabular-nums">
                        {typeof value === 'number' ? formatChartNumber(value) : NO_VALUE_DASH}
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
          {model.rows.length > 40 ? (
            <p className="mt-1 text-muted-foreground">
              {chartCopy('frame.showing', { shown: 40, total: model.rows.length })}
            </p>
          ) : null}
        </details>
      ) : null}
    </section>
  )
}

export default QualityHotspotPanel
