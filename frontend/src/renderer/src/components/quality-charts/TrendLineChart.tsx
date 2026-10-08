import { ChartFrame } from './ChartFrame'
import { ChartLegend } from './ChartLegend'
import type { ChartFrameIdentity } from './chart-frame-shared-props'
import { createLinearScale, niceDomain } from './chart-linear-scale'
import { splitAtGaps, toPolylinePoints } from './chart-line-segments'
import { describeSeriesRange } from './chart-text-summary'
import { chartCopy } from './quality-chart-copy'
import {
  SEVERITY_ENCODING,
  VERDICT_ENCODING,
  labelOf,
  toSeverityLevel,
  type EncodingShape,
  type QualitySeverityLevel,
  type QualityVerdictLevel
} from './severity-encoding'
import { useChartKeyboardNavigation } from './useChartKeyboardNavigation'

export type LinearSeriesPoint = {
  label: string
  value: number | null
  marker?: QualityVerdictLevel
}
export type TrendSeries = {
  id: QualitySeverityLevel | string
  label: string
  points: LinearSeriesPoint[]
}

type TrendLineChartProps = {
  frame: ChartFrameIdentity
  series: TrendSeries[]
  xLabels: string[]
  maxPoints?: number
  onSelectPoint?: (index: number) => void
}

// Why: non-severity series still need a non-colour cue, so cycle dash patterns.
const FALLBACK_DASHES = [null, '6 3', '2 2', '8 2 2 2']
const MARGIN = { left: 36, right: 12, top: 18, bottom: 22 }

function lastN<T>(values: readonly T[], n: number): T[] {
  return values.length > n ? values.slice(values.length - n) : [...values]
}

function TrendPlot({
  series,
  xLabels,
  width,
  height,
  summary,
  onSelectPoint
}: {
  series: TrendSeries[]
  xLabels: string[]
  width: number
  height: number
  summary: string
  onSelectPoint?: (index: number) => void
}): React.JSX.Element {
  const count = xLabels.length
  const nav = useChartKeyboardNavigation({
    rowCount: 1,
    columnCount: Math.max(1, count),
    onActivate: (_row, col) => onSelectPoint?.(col)
  })
  const numbers = series
    .flatMap((s) => s.points.map((p) => p.value))
    .filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
  const [lo, hi] = niceDomain(
    Math.min(0, ...(numbers.length ? numbers : [0])),
    Math.max(...(numbers.length ? numbers : [0]))
  )
  const plotW = Math.max(1, width - MARGIN.left - MARGIN.right)
  const plotH = Math.max(1, height - MARGIN.top - MARGIN.bottom)
  const yScale = createLinearScale([lo, hi], [MARGIN.top + plotH, MARGIN.top])
  const xOf = (i: number): number =>
    MARGIN.left + (count > 1 ? (i / (count - 1)) * plotW : plotW / 2)
  const labelEvery = Math.max(1, Math.ceil(count / Math.max(1, Math.floor(plotW / 56))))
  const interactive = Boolean(onSelectPoint)
  const markers = series
    .flatMap((s) => s.points.map((p, i) => ({ i, marker: p.marker })))
    .filter((m) => m.marker)
  const markerIndexes = [...new Map(markers.map((m) => [m.i, m.marker!])).entries()]

  const svgProps = interactive
    ? { ...nav.containerProps, 'aria-label': summary, 'aria-description': chartCopy('grid.hint') }
    : { 'aria-hidden': true as const }

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      focusable="false"
      {...svgProps}
    >
      {yScale.ticks(4).map((t) => (
        <g key={t}>
          <line
            x1={MARGIN.left}
            x2={MARGIN.left + plotW}
            y1={yScale.scale(t)}
            y2={yScale.scale(t)}
            className="stroke-border"
            strokeWidth="1"
          />
          <text
            x={MARGIN.left - 6}
            y={yScale.scale(t)}
            textAnchor="end"
            dominantBaseline="middle"
            className="fill-muted-foreground text-[10px]"
          >
            {t}
          </text>
        </g>
      ))}
      {xLabels.map((label, i) =>
        i % labelEvery === 0 ? (
          <text
            key={i}
            x={xOf(i)}
            y={height - 6}
            textAnchor="middle"
            className="fill-muted-foreground text-[10px]"
          >
            {label}
          </text>
        ) : null
      )}
      {series.map((s, si) => {
        const level = toSeverityLevel(s.id)
        const isSeverity = s.id === 'error' || s.id === 'warning' || s.id === 'info'
        const entry = isSeverity ? SEVERITY_ENCODING[level] : null
        const dash = entry ? entry.strokeDash : FALLBACK_DASHES[si % FALLBACK_DASHES.length]
        const colorClass = entry ? entry.textClass : 'text-foreground'
        const values = s.points.map((p) => p.value)
        const segments = splitAtGaps(values, xOf, yScale.scale)
        return (
          <g key={s.id} data-series={s.id} className={colorClass}>
            {segments.map((seg, k) =>
              seg.length > 1 ? (
                <polyline
                  key={k}
                  points={toPolylinePoints(seg)}
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.75"
                  strokeDasharray={dash ?? undefined}
                />
              ) : null
            )}
            {segments.flat().map((p) =>
              entry ? (
                <svg
                  key={p.index}
                  x={p.x - 5}
                  y={p.y - 5}
                  width="10"
                  height="10"
                  viewBox="0 0 16 16"
                  aria-hidden="true"
                >
                  <SeverityGlyphInline shape={entry.shape} />
                </svg>
              ) : (
                <circle key={p.index} cx={p.x} cy={p.y} r="2.5" fill="currentColor" />
              )
            )}
          </g>
        )
      })}
      {markerIndexes.map(([i, marker]) => (
        <g key={`m${i}`} data-marker={marker} className="text-foreground">
          <polygon
            points={`${xOf(i)},${MARGIN.top - 14} ${xOf(i) + 4},${MARGIN.top - 9} ${xOf(i)},${MARGIN.top - 4} ${xOf(i) - 4},${MARGIN.top - 9}`}
            fill="currentColor"
          />
          <text x={xOf(i) + 7} y={MARGIN.top - 5} className="fill-foreground text-[10px]">
            {labelOf(VERDICT_ENCODING[marker])}
          </text>
        </g>
      ))}
      {interactive ? (
        <g {...nav.getRowProps(0)}>
          {xLabels.map((label, i) => (
            <rect
              key={i}
              {...nav.getCellProps(0, i)}
              aria-label={label}
              x={xOf(i) - Math.max(6, plotW / Math.max(1, count) / 2)}
              y={MARGIN.top}
              width={Math.max(12, plotW / Math.max(1, count))}
              height={plotH}
              className="fill-transparent stroke-transparent focus-visible:stroke-ring focus-visible:stroke-2 outline-none cursor-pointer"
            />
          ))}
        </g>
      ) : null}
    </svg>
  )
}

// Why: nested SVG data-point markers reuse the SeverityGlyph shapes at point size.
function SeverityGlyphInline({ shape }: { shape: EncodingShape }): React.JSX.Element {
  switch (shape) {
    case 'octagon':
      return (
        <polygon
          points="5,1.5 11,1.5 14.5,5 14.5,11 11,14.5 5,14.5 1.5,11 1.5,5"
          fill="currentColor"
        />
      )
    case 'triangle':
      return <polygon points="8,2 14.5,13.5 1.5,13.5" fill="currentColor" />
    default:
      return <circle cx="8" cy="8" r="5.5" fill="none" stroke="currentColor" strokeWidth="2.5" />
  }
}

export function TrendLineChart({
  frame,
  series,
  xLabels,
  maxPoints = 50,
  onSelectPoint
}: TrendLineChartProps): React.JSX.Element {
  const total = xLabels.length
  const shownLabels = lastN(xLabels, maxPoints)
  const shownSeries = series.map((s) => ({ ...s, points: lastN(s.points, maxPoints) }))
  const truncated = total > shownLabels.length
  const summary = [
    shownSeries
      .map((s) => describeSeriesRange({ label: s.label, points: s.points.map((p) => p.value) }))
      .join('. '),
    truncated ? chartCopy('frame.showing', { shown: shownLabels.length, total }) : ''
  ]
    .filter(Boolean)
    .join('. ')
  const severityLevels = shownSeries
    .filter((s) => s.id === 'error' || s.id === 'warning' || s.id === 'info')
    .map((s) => s.id as QualitySeverityLevel)
  const hasMarkers = shownSeries.some((s) => s.points.some((p) => p.marker))
  const table = {
    caption: frame.title,
    columns: [
      { key: 'turn', label: chartCopy('trend.turn') },
      ...shownSeries.map((s) => ({ key: s.id, label: s.label, align: 'end' as const })),
      ...(hasMarkers ? [{ key: 'verdict', label: chartCopy('trend.verdict') }] : [])
    ],
    rows: shownLabels.map((label, i) => {
      const marker = shownSeries.map((s) => s.points[i]?.marker).find(Boolean)
      return {
        turn: label,
        ...Object.fromEntries(shownSeries.map((s) => [s.id, s.points[i]?.value ?? null])),
        ...(hasMarkers ? { verdict: marker ? labelOf(VERDICT_ENCODING[marker]) : null } : {})
      }
    })
  }
  return (
    <ChartFrame
      {...frame}
      status={frame.status ?? 'ready'}
      minHeight={frame.minHeight ?? 200}
      summary={summary}
      surface={onSelectPoint ? 'custom' : 'img'}
      table={table}
      description={frame.description ?? chartCopy('trend.axis')}
      legend={
        <div className="flex flex-col gap-1">
          {severityLevels.length > 0 ? (
            <ChartLegend kind="severity" levels={severityLevels} />
          ) : null}
          {hasMarkers ? (
            <p className="text-xs text-foreground">{`◆ ${chartCopy('trend.verdictChange')}`}</p>
          ) : null}
          {truncated ? (
            <p className="text-xs text-muted-foreground">
              {chartCopy('frame.showing', { shown: shownLabels.length, total })}
            </p>
          ) : null}
          {frame.legend}
        </div>
      }
    >
      {({ width, height }) => (
        <TrendPlot
          series={shownSeries}
          xLabels={shownLabels}
          width={width}
          height={Math.max(height, 120)}
          summary={summary}
          onSelectPoint={onSelectPoint}
        />
      )}
    </ChartFrame>
  )
}
