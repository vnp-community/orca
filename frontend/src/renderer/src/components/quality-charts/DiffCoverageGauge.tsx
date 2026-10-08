import { Badge } from '../ui/badge'
import { ChartFrame } from './ChartFrame'
import type { ChartFrameIdentity } from './chart-frame-shared-props'
import { describeCoverage, formatChartNumber } from './chart-text-summary'
import { chartCopy } from './quality-chart-copy'

type DiffCoverageGaugeProps = {
  frame: ChartFrameIdentity
  covered: number
  total: number
  /** Percentages 0..100, already normalised by the caller. */
  thresholds?: { warnBelow?: number | null; failBelow?: number | null }
  source: 'measured' | 'estimated'
  partial?: boolean
}

function validThreshold(value: number | null | undefined): number | null {
  return typeof value === 'number' && Number.isFinite(value)
    ? Math.min(100, Math.max(0, value))
    : null
}

export function DiffCoverageGauge({
  frame,
  covered,
  total,
  thresholds,
  source,
  partial
}: DiffCoverageGaugeProps): React.JSX.Element {
  const hasData = Number.isFinite(total) && total > 0
  const clamped = hasData && covered > total
  const safeCovered = hasData
    ? Math.min(Math.max(0, Number.isFinite(covered) ? covered : 0), total)
    : 0
  const percent = hasData ? (safeCovered / total) * 100 : 0
  const warn = validThreshold(thresholds?.warnBelow)
  const fail = validThreshold(thresholds?.failBelow)
  const percentText = formatChartNumber(Math.round(percent * 10) / 10)
  const valueText = hasData
    ? chartCopy('gauge.value', { percent: percentText, covered: safeCovered, total })
    : chartCopy('gauge.none')
  const sourceText =
    source === 'estimated' ? chartCopy('gauge.estimated') : chartCopy('gauge.measured')
  const relation = !hasData
    ? null
    : fail !== null && percent < fail
      ? chartCopy('gauge.belowFail')
      : warn !== null && percent < warn
        ? chartCopy('gauge.belowWarn')
        : null
  const summary = describeCoverage({ covered, total, source })

  const rows: Record<string, string | number | null>[] = [
    { metric: chartCopy('gauge.percent'), value: hasData ? `${percentText}%` : null },
    { metric: chartCopy('gauge.covered'), value: hasData ? safeCovered : null },
    { metric: chartCopy('gauge.total'), value: hasData ? total : null },
    { metric: chartCopy('gauge.source'), value: sourceText },
    ...(partial ? [{ metric: chartCopy('gauge.scope'), value: chartCopy('gauge.partial') }] : []),
    ...(warn !== null
      ? [{ metric: chartCopy('gauge.warnBelow', { value: formatChartNumber(warn) }), value: warn }]
      : []),
    ...(fail !== null
      ? [{ metric: chartCopy('gauge.failBelow', { value: formatChartNumber(fail) }), value: fail }]
      : []),
    ...(clamped ? [{ metric: chartCopy('table.note'), value: chartCopy('gauge.clamped') }] : [])
  ]

  return (
    <ChartFrame
      {...frame}
      status={hasData ? (frame.status ?? 'ready') : 'empty'}
      emptyReason={frame.emptyReason ?? chartCopy('gauge.none')}
      minHeight={frame.minHeight ?? 72}
      summary={summary}
      surface="custom"
      table={{
        caption: frame.title,
        columns: [
          { key: 'metric', label: chartCopy('table.metric') },
          { key: 'value', label: chartCopy('table.value'), align: 'end' }
        ],
        rows
      }}
    >
      {() => (
        <div
          role="meter"
          aria-label={summary}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(percent * 10) / 10}
          aria-valuetext={`${valueText}. ${sourceText}`}
          data-gauge
          className="flex flex-col gap-2"
        >
          <div className="flex flex-wrap items-center gap-2 text-sm text-foreground">
            <span className="tabular-nums">{valueText}</span>
            <Badge
              variant="outline"
              data-source={source}
              className={source === 'estimated' ? 'border-dashed' : undefined}
            >
              {sourceText}
            </Badge>
            {partial ? <Badge variant="outline">{chartCopy('gauge.partial')}</Badge> : null}
          </div>
          <div className="relative h-3 w-full rounded-sm bg-muted">
            <div
              className="absolute inset-y-0 left-0 rounded-sm bg-foreground"
              style={{ width: `${percent}%` }}
            />
            {warn !== null ? (
              <div
                data-threshold="warn"
                className="absolute -inset-y-1 border-l-2 border-dashed border-quality-warning"
                style={{ left: `${warn}%` }}
              />
            ) : null}
            {fail !== null ? (
              <div
                data-threshold="fail"
                className="absolute -inset-y-1 border-l-2 border-solid border-quality-error"
                style={{ left: `${fail}%` }}
              />
            ) : null}
          </div>
          <div className="flex flex-wrap gap-x-4 text-xs text-foreground">
            {warn !== null ? (
              <span>{chartCopy('gauge.warnBelow', { value: formatChartNumber(warn) })}</span>
            ) : null}
            {fail !== null ? (
              <span>{chartCopy('gauge.failBelow', { value: formatChartNumber(fail) })}</span>
            ) : null}
            {relation ? <span className="font-medium">{relation}</span> : null}
            {clamped ? <span>{chartCopy('gauge.clamped')}</span> : null}
          </div>
        </div>
      )}
    </ChartFrame>
  )
}
