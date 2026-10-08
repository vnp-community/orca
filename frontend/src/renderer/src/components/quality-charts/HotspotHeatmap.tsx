import { ChartFrame } from './ChartFrame'
import type { ChartFrameIdentity } from './chart-frame-shared-props'
import { describeGrid, formatChartNumber } from './chart-text-summary'
import { heatCellStyle } from './heat-cell-style'
import { bucketIntensity } from './heat-intensity-scale'
import { NO_VALUE_DASH, chartCopy } from './quality-chart-copy'
import { useChartKeyboardNavigation } from './useChartKeyboardNavigation'
import { useLazyChartMount } from './useLazyChartMount'

export type HeatmapRow = { id: string; label: string; values: (number | null)[] }
export type HeatmapColumn = { key: string; label: string; unit?: string }

type HotspotHeatmapProps = {
  frame: ChartFrameIdentity
  rows: HeatmapRow[]
  columns: HeatmapColumn[]
  onSelectRow?: (id: string) => void
  maxRows?: number
}

function formatCell(value: number | null | undefined, unit?: string): string {
  return typeof value === 'number' && Number.isFinite(value)
    ? `${formatChartNumber(value)}${unit ?? ''}`
    : NO_VALUE_DASH
}

function HeatGrid({
  rows,
  columns,
  summary,
  onSelectRow
}: {
  rows: HeatmapRow[]
  columns: HeatmapColumn[]
  summary: string
  onSelectRow?: (id: string) => void
}): React.JSX.Element {
  const nav = useChartKeyboardNavigation({
    rowCount: rows.length,
    columnCount: columns.length,
    onActivate: (row) => {
      const target = rows[row]
      if (target) {
        onSelectRow?.(target.id)
      }
    }
  })
  // Why: per-column domains, since churn, findings and uncovered % are not comparable units.
  const domains = columns.map((_, c) => {
    const nums = rows
      .map((r) => r.values[c])
      .filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
    return { min: Math.min(...nums), max: Math.max(...nums) }
  })
  const template = `minmax(8rem, 2fr) repeat(${columns.length}, minmax(4.5rem, 1fr))`
  return (
    <div
      {...nav.containerProps}
      aria-label={summary}
      aria-description={chartCopy('grid.hint')}
      className="flex flex-col text-xs outline-none"
      data-heatmap
    >
      <div role="row" className="grid gap-px" style={{ gridTemplateColumns: template }}>
        <div role="columnheader" className="px-2 py-1 font-medium text-muted-foreground">
          {chartCopy('table.label')}
        </div>
        {columns.map((c) => (
          <div
            key={c.key}
            role="columnheader"
            className="px-2 py-1 text-right font-medium text-muted-foreground"
          >
            {c.label}
            {c.unit ? ` (${c.unit})` : ''}
          </div>
        ))}
      </div>
      {rows.map((row, r) => (
        <div
          key={row.id}
          {...nav.getRowProps(r)}
          className="grid gap-px"
          style={{ gridTemplateColumns: template }}
        >
          <div role="rowheader" className="truncate px-2 py-1 text-foreground" title={row.label}>
            {row.label}
          </div>
          {columns.map((col, c) => {
            const value = row.values[c]
            const bucket = bucketIntensity(value, domains[c])
            const heat = heatCellStyle(bucket)
            return (
              <div
                key={col.key}
                {...nav.getCellProps(r, c)}
                aria-label={`${row.label}, ${col.label}: ${formatCell(value, col.unit)}`}
                data-intensity={bucket ?? ''}
                className={`cursor-pointer border border-border px-2 py-1 text-right tabular-nums outline-none focus-visible:ring-2 focus-visible:ring-ring ${heat.textClass}`}
                style={heat.style}
              >
                {formatCell(value, col.unit)}
              </div>
            )
          })}
        </div>
      ))}
    </div>
  )
}

export function HotspotHeatmap({
  frame,
  rows,
  columns,
  onSelectRow,
  maxRows = 40
}: HotspotHeatmapProps): React.JSX.Element {
  const lazy = useLazyChartMount<HTMLDivElement>()
  const shown = rows.slice(0, maxRows)
  const summary = describeGrid({
    rows: shown.length,
    columns: columns.length,
    shown: shown.length,
    total: rows.length
  })
  const table = {
    caption: frame.title,
    columns: [
      { key: 'label', label: chartCopy('table.label') },
      ...columns.map((c) => ({
        key: c.key,
        label: c.unit ? `${c.label} (${c.unit})` : c.label,
        align: 'end' as const
      }))
    ],
    rows: shown.map((r) => ({
      label: r.label,
      ...Object.fromEntries(columns.map((c, i) => [c.key, r.values[i] ?? null]))
    }))
  }
  return (
    <div ref={lazy.ref}>
      <ChartFrame
        {...frame}
        status={
          !lazy.mounted ? 'loading' : shown.length === 0 ? 'empty' : (frame.status ?? 'ready')
        }
        emptyReason={frame.emptyReason ?? chartCopy('heatmap.empty')}
        minHeight={frame.minHeight ?? 160}
        summary={summary}
        surface="custom"
        table={table}
        legend={
          <>
            {rows.length > shown.length ? (
              <p className="text-xs text-muted-foreground">
                {chartCopy('frame.showing', { shown: shown.length, total: rows.length })}
              </p>
            ) : null}
            {frame.legend}
          </>
        }
      >
        {() => (
          <HeatGrid rows={shown} columns={columns} summary={summary} onSelectRow={onSelectRow} />
        )}
      </ChartFrame>
    </div>
  )
}
