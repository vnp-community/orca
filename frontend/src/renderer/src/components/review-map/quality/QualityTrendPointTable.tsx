/**
 * QualityTrendPointTable.tsx — FE-CV-TASK-087-17
 *
 * Per-point table of the trend: source is spelled out with a glyph (● local, ◇ CI), so it never
 * depends on colour.
 *
 * @module components/review-map/quality/QualityTrendPointTable
 */

import { VERDICT_ENCODING, labelOf } from '../../quality-charts/severity-encoding'
import { formatChartNumber } from '../../quality-charts/chart-text-summary'
import { NO_VALUE_DASH } from '../../quality-charts/quality-chart-copy'
import type { QualityTrendModel, TrendSourceKind } from './quality-trend-series'
import { qv } from './quality-visualization-copy'

function sourceText(kind: TrendSourceKind): string {
  if (kind === 'local') {
    return `● ${qv('trend.sourceLocal')}`
  }
  if (kind === 'ci') {
    return `◇ ${qv('trend.sourceCi')}`
  }
  return qv('trend.sourceUnknown')
}

export function QualityTrendPointTable({ model }: { model: QualityTrendModel }): React.JSX.Element {
  return (
    <details className="text-xs" data-trend-details>
      <summary className="cursor-pointer text-foreground">{qv('trend.details')}</summary>
      <table className="mt-1 w-full text-left">
        <thead>
          <tr className="text-muted-foreground">
            <th scope="col" className="py-0.5 pr-2 font-medium">
              {qv('trend.colPoint')}
            </th>
            <th scope="col" className="py-0.5 pr-2 font-medium">
              {qv('trend.colSource')}
            </th>
            <th scope="col" className="py-0.5 pr-2 font-medium">
              {qv('trend.colVerdict')}
            </th>
            <th scope="col" className="py-0.5 text-right font-medium">
              {qv('trend.colCoverage')}
            </th>
          </tr>
        </thead>
        <tbody>
          {model.xLabels.map((label, i) => {
            const coverage = model.diffCoverageSpark[i]
            return (
              <tr key={`${model.points[i].turnKey}-${i}`} data-trend-row={i}>
                <th scope="row" className="py-0.5 pr-2 font-normal">
                  {label}
                </th>
                <td className="py-0.5 pr-2">{sourceText(model.sources[i])}</td>
                <td className="py-0.5 pr-2">{labelOf(VERDICT_ENCODING[model.verdicts[i]])}</td>
                <td className="py-0.5 text-right tabular-nums">
                  {coverage === null ? NO_VALUE_DASH : `${formatChartNumber(coverage)}%`}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </details>
  )
}
