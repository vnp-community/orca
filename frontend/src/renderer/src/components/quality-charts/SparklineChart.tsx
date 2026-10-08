import { ChartFrame } from './ChartFrame'
import { ChartTextAlternative } from './ChartTextAlternative'
import type { ChartFrameIdentity } from './chart-frame-shared-props'
import { createLinearScale, niceDomain } from './chart-linear-scale'
import { splitAtGaps, toPolylinePoints } from './chart-line-segments'
import { describeSeriesRange, formatChartNumber } from './chart-text-summary'
import { chartCopy } from './quality-chart-copy'

type SparklineChartProps = {
  label: string
  points: readonly (number | null)[]
  /** When set the sparkline is wrapped in a full ChartFrame; otherwise it renders compactly inline. */
  frame?: ChartFrameIdentity
  width?: number
  height?: number
}

function Drawing({
  points,
  width,
  height
}: {
  points: readonly (number | null)[]
  width: number
  height: number
}): React.JSX.Element {
  const numeric = points.filter((p): p is number => typeof p === 'number' && Number.isFinite(p))
  const pad = 4
  if (numeric.length < 2) {
    return (
      <span className="text-xs text-muted-foreground">
        {numeric.length === 1 ? (
          <span className="tabular-nums">{formatChartNumber(numeric[0])} </span>
        ) : null}
        {chartCopy('sparkline.needTwo')}
      </span>
    )
  }
  const [lo, hi] = niceDomain(Math.min(...numeric), Math.max(...numeric))
  const y = createLinearScale([lo, hi], [height - pad, pad])
  const step = points.length > 1 ? (width - pad * 2 - 28) / (points.length - 1) : 0
  const xOf = (i: number): number => pad + i * step
  const segments = splitAtGaps(points, xOf, y.scale)
  const last = segments.at(-1)?.at(-1)
  return (
    <span className="inline-flex items-center gap-1 text-foreground">
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        aria-hidden="true"
        focusable="false"
      >
        {segments.map((seg, i) =>
          seg.length > 1 ? (
            <polyline
              key={i}
              points={toPolylinePoints(seg)}
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
            />
          ) : (
            <circle key={i} cx={seg[0].x} cy={seg[0].y} r="1.5" fill="currentColor" />
          )
        )}
        {last ? <circle cx={last.x} cy={last.y} r="2.5" fill="currentColor" /> : null}
      </svg>
      <span className="text-xs tabular-nums">{formatChartNumber(numeric.at(-1) ?? 0)}</span>
    </span>
  )
}

export function SparklineChart({
  label,
  points,
  frame,
  width = 96,
  height = 24
}: SparklineChartProps): React.JSX.Element {
  const summary = describeSeriesRange({ label, points })
  const table = {
    caption: label,
    columns: [
      { key: 'point', label: chartCopy('trend.point') },
      { key: 'value', label: chartCopy('table.value'), align: 'end' as const }
    ],
    rows: points.map((value, i) => ({ point: i + 1, value }))
  }
  if (frame) {
    return (
      <ChartFrame
        {...frame}
        status={frame.status ?? 'ready'}
        minHeight={frame.minHeight ?? height + 8}
        summary={summary}
        table={table}
      >
        {({ width: w }) => <Drawing points={points} width={Math.min(w, 320)} height={height} />}
      </ChartFrame>
    )
  }
  return (
    <span data-sparkline>
      <span role="img" aria-label={summary}>
        <Drawing points={points} width={width} height={height} />
      </span>
      <ChartTextAlternative data={table} visible={false} />
    </span>
  )
}
