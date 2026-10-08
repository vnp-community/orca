import { ChartFrame } from './ChartFrame'
import type { ChartFrameIdentity } from './chart-frame-shared-props'
import { describeTreemap, formatChartNumber } from './chart-text-summary'
import { heatCellStyle } from './heat-cell-style'
import { bucketIntensity } from './heat-intensity-scale'
import { chartCopy } from './quality-chart-copy'
import { squarify } from './treemap-squarified-layout'
import { useChartKeyboardNavigation } from './useChartKeyboardNavigation'
import { useLazyChartMount } from './useLazyChartMount'

export type TreemapItem = {
  id: string
  label: string
  size: number
  intensity: number
  /** Caller-defined overlay tags, drawn as a border style (never a caller-supplied colour). */
  overlay?: string[]
}

type MetricTreemapProps = {
  frame: ChartFrameIdentity
  items: TreemapItem[]
  sizeLabel: string
  intensityLabel: string
  onSelect?: (id: string) => void
  maxTiles?: number
}

function TreemapGrid({
  tiles,
  hiddenCount,
  hiddenSize,
  domain,
  width,
  height,
  summary,
  sizeLabel,
  intensityLabel,
  onSelect
}: {
  tiles: TreemapItem[]
  hiddenCount: number
  hiddenSize: number
  domain: { min: number; max: number }
  width: number
  height: number
  summary: string
  sizeLabel: string
  intensityLabel: string
  onSelect?: (id: string) => void
}): React.JSX.Element {
  const rest = hiddenCount
  const layoutItems = tiles.map((t) => ({ id: t.id, size: t.size }))
  if (rest > 0) {
    layoutItems.push({ id: '__more__', size: hiddenSize })
  }
  const { rects } = squarify(layoutItems, { x: 0, y: 0, width, height })
  const byId = new Map(tiles.map((t) => [t.id, t]))
  const nav = useChartKeyboardNavigation({
    rowCount: 1,
    columnCount: tiles.length,
    onActivate: (_row, col) => {
      const tile = tiles[col]
      if (tile) {
        onSelect?.(tile.id)
      }
    }
  })
  const order = new Map(tiles.map((t, i) => [t.id, i]))
  return (
    <div
      {...nav.containerProps}
      aria-label={summary}
      aria-description={chartCopy('grid.hint')}
      className="relative outline-none"
      style={{ width, height }}
      data-treemap
    >
      <div {...nav.getRowProps(0)} style={{ display: 'contents' }}>
        {rects.map((rect) => {
          const style: React.CSSProperties = {
            left: rect.x,
            top: rect.y,
            width: rect.width,
            height: rect.height
          }
          if (rect.id === '__more__') {
            return (
              <div
                key={rect.id}
                role="gridcell"
                aria-disabled="true"
                aria-label={chartCopy('treemap.moreLabel', { count: rest })}
                data-more
                className="absolute flex items-center justify-center overflow-hidden border border-dashed border-border bg-muted text-xs text-foreground"
                style={style}
              >
                {chartCopy('treemap.more', { count: rest })}
              </div>
            )
          }
          const tile = byId.get(rect.id)!
          const col = order.get(rect.id)!
          const bucket = bucketIntensity(tile.intensity, domain)
          const heat = heatCellStyle(bucket)
          const overlay = tile.overlay && tile.overlay.length > 0 ? tile.overlay : null
          const label = chartCopy('treemap.tileLabel', {
            label: tile.label,
            sizeLabel,
            size: formatChartNumber(tile.size),
            intensityLabel,
            intensity: formatChartNumber(tile.intensity)
          })
          const roomForText = rect.width >= 48 && rect.height >= 18
          return (
            <button
              key={rect.id}
              type="button"
              {...nav.getCellProps(0, col)}
              aria-label={
                overlay
                  ? `${label}, ${chartCopy('treemap.overlay', { tags: overlay.join(', ') })}`
                  : label
              }
              data-intensity={bucket ?? ''}
              data-overlay={overlay ? overlay.join(' ') : undefined}
              className={`absolute overflow-hidden border p-0.5 text-left text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:z-10 ${heat.textClass} ${overlay ? 'border-2 border-dashed border-foreground' : 'border-border'}`}
              style={{ ...style, ...heat.style }}
            >
              {roomForText ? <span className="block truncate">{tile.label}</span> : null}
            </button>
          )
        })}
      </div>
    </div>
  )
}

export function MetricTreemap({
  frame,
  items,
  sizeLabel,
  intensityLabel,
  onSelect,
  maxTiles = 400
}: MetricTreemapProps): React.JSX.Element {
  const lazy = useLazyChartMount<HTMLDivElement>()
  const drawable = items.filter((i) => Number.isFinite(i.size) && i.size > 0)
  const sorted = [...drawable].sort((a, b) => b.size - a.size || (a.id < b.id ? -1 : 1))
  const tiles = sorted.slice(0, maxTiles)
  const hiddenCount = sorted.length - tiles.length
  const intensities = tiles.map((t) => t.intensity).filter(Number.isFinite)
  const domain = { min: Math.min(...intensities), max: Math.max(...intensities) }
  const summary = describeTreemap({ count: tiles.length, sizeLabel, intensityLabel })
  const empty = tiles.length === 0
  const table = {
    caption: frame.title,
    columns: [
      { key: 'label', label: chartCopy('table.label') },
      { key: 'size', label: `${chartCopy('table.size')} (${sizeLabel})`, align: 'end' as const },
      {
        key: 'intensity',
        label: `${chartCopy('table.intensity')} (${intensityLabel})`,
        align: 'end' as const
      }
    ],
    rows: [
      ...tiles.map((t) => ({
        label: t.label,
        size: t.size,
        intensity: Number.isFinite(t.intensity) ? t.intensity : null
      })),
      ...(hiddenCount > 0
        ? [
            {
              label: chartCopy('treemap.more', { count: hiddenCount }),
              size: null,
              intensity: null
            }
          ]
        : [])
    ]
  }
  return (
    <div ref={lazy.ref}>
      <ChartFrame
        {...frame}
        status={!lazy.mounted ? 'loading' : empty ? 'empty' : (frame.status ?? 'ready')}
        emptyReason={frame.emptyReason ?? chartCopy('treemap.empty')}
        minHeight={frame.minHeight ?? 280}
        summary={summary}
        surface="custom"
        table={table}
        legend={
          <>
            {hiddenCount > 0 ? (
              <p className="text-xs text-muted-foreground">
                {chartCopy('frame.showing', { shown: tiles.length, total: sorted.length })}
              </p>
            ) : null}
            {frame.legend}
          </>
        }
      >
        {({ width, height }) => (
          <TreemapGrid
            tiles={tiles}
            hiddenCount={hiddenCount}
            hiddenSize={sorted.slice(maxTiles).reduce((sum, t) => sum + t.size, 0)}
            domain={domain}
            width={width}
            height={Math.max(height, frame.minHeight ?? 280)}
            summary={summary}
            sizeLabel={sizeLabel}
            intensityLabel={intensityLabel}
            onSelect={onSelect}
          />
        )}
      </ChartFrame>
    </div>
  )
}
