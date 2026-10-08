import { useMemo } from 'react'
import { ChartFrame } from './ChartFrame'
import type { ChartFrameIdentity } from './chart-frame-shared-props'
import { describeMatrix } from './chart-text-summary'
import { orderByStrongComponents } from './dependency-matrix-ordering'
import { bucketIntensity } from './heat-intensity-scale'
import { chartCopy } from './quality-chart-copy'
import { useChartKeyboardNavigation } from './useChartKeyboardNavigation'
import { useLazyChartMount } from './useLazyChartMount'

export type MatrixNode = { id: string; label: string; group?: string }
export type MatrixEdge = { from: string; to: string; weight: number }

type DependencyMatrixProps = {
  frame: ChartFrameIdentity
  nodes: MatrixNode[]
  edges: MatrixEdge[]
  maxNodes?: number
  onSelectCell?: (from: string, to: string) => void
}

const OTHER_ID = '__other__'
const LABEL_GUTTER = 120
const HEADER_HEIGHT = 16

type PreparedMatrix = {
  order: MatrixNode[]
  blocks: string[][]
  cells: { row: number; col: number; from: string; to: string; weight: number; backward: boolean }[]
  hidden: number
}

function prepareMatrix(nodes: MatrixNode[], edges: MatrixEdge[], maxNodes: number): PreparedMatrix {
  const unique = [...new Map(nodes.map((n) => [n.id, n])).values()]
  let kept = unique
  let hidden = 0
  const collapsed = new Set<string>()
  if (unique.length > maxNodes) {
    // Why: keep the busiest nodes; the rest collapse into one trailing "Other" row/column (never silent).
    const load = new Map<string, number>(unique.map((n) => [n.id, 0]))
    for (const e of edges) {
      load.set(e.from, (load.get(e.from) ?? 0) + e.weight)
      load.set(e.to, (load.get(e.to) ?? 0) + e.weight)
    }
    const ranked = [...unique].sort((a, b) => (load.get(b.id) ?? 0) - (load.get(a.id) ?? 0))
    const keepIds = new Set(ranked.slice(0, maxNodes - 1).map((n) => n.id))
    kept = unique.filter((n) => keepIds.has(n.id))
    unique.forEach((n) => !keepIds.has(n.id) && collapsed.add(n.id))
    hidden = collapsed.size
  }
  const mapId = (id: string): string => (collapsed.has(id) ? OTHER_ID : id)
  const merged = new Map<string, MatrixEdge>()
  for (const e of edges) {
    if (!Number.isFinite(e.weight) || e.weight <= 0) {
      continue
    }
    const from = mapId(e.from)
    const to = mapId(e.to)
    const key = `${from}\u0000${to}`
    const prev = merged.get(key)
    merged.set(key, { from, to, weight: (prev?.weight ?? 0) + e.weight })
  }
  const ids = kept.map((n) => n.id)
  const ordering = orderByStrongComponents(
    ids,
    [...merged.values()].filter((e) => e.from !== OTHER_ID && e.to !== OTHER_ID)
  )
  const labelOf = new Map(kept.map((n) => [n.id, n]))
  const order = ordering.order.map((id) => labelOf.get(id)!)
  if (hidden > 0) {
    order.push({ id: OTHER_ID, label: chartCopy('matrix.other', { count: hidden }) })
  }
  const index = new Map(order.map((n, i) => [n.id, i]))
  const cells = [...merged.values()]
    .filter((e) => index.has(e.from) && index.has(e.to))
    .map((e) => ({
      row: index.get(e.from)!,
      col: index.get(e.to)!,
      from: e.from,
      to: e.to,
      weight: e.weight,
      backward: index.get(e.from)! > index.get(e.to)!
    }))
    .sort((a, b) => a.row - b.row || a.col - b.col)
  return { order, blocks: ordering.blocks, cells, hidden }
}

function shorten(text: string): string {
  return text.length > 18 ? `${text.slice(0, 17)}…` : text
}

function MatrixGrid({
  model,
  width,
  summary,
  onSelectCell
}: {
  model: PreparedMatrix
  width: number
  summary: string
  onSelectCell?: (from: string, to: string) => void
}): React.JSX.Element {
  const n = model.order.length
  // Why: cell size depends on width only; deriving it from height would feed back into the measured size.
  const cell = Math.max(6, Math.min(20, Math.floor((width - LABEL_GUTTER) / Math.max(1, n))))
  const side = n * cell
  const nav = useChartKeyboardNavigation({
    rowCount: n,
    columnCount: n,
    cells: model.cells.map((c) => ({ row: c.row, col: c.col })),
    onActivate: (row, col) => {
      const hit = model.cells.find((c) => c.row === row && c.col === col)
      if (hit) {
        onSelectCell?.(hit.from, hit.to)
      }
    }
  })
  const weights = model.cells.map((c) => c.weight)
  const domain = { min: Math.min(...weights), max: Math.max(...weights) }
  const grid = useMemo(() => {
    let d = ''
    for (let i = 0; i <= n; i++) {
      d += `M${LABEL_GUTTER},${HEADER_HEIGHT + i * cell}h${side}M${LABEL_GUTTER + i * cell},${HEADER_HEIGHT}v${side}`
    }
    return d
  }, [n, cell, side])
  const index = new Map(model.order.map((node, i) => [node.id, i]))
  return (
    <svg
      {...nav.containerProps}
      width={LABEL_GUTTER + side + 2}
      height={HEADER_HEIGHT + side + 2}
      aria-label={summary}
      aria-description={`${chartCopy('matrix.readHint')} ${chartCopy('grid.hint')}`}
      className="outline-none"
      data-matrix
    >
      <path d={grid} className="stroke-border" strokeWidth="0.5" fill="none" />
      {model.order.map((node, i) => (
        <g key={node.id}>
          <text
            x={LABEL_GUTTER - 4}
            y={HEADER_HEIGHT + i * cell + cell / 2}
            textAnchor="end"
            dominantBaseline="middle"
            className="fill-foreground text-[10px]"
          >
            {`${i + 1} ${shorten(node.label)}`}
          </text>
          {cell >= 12 ? (
            <text
              x={LABEL_GUTTER + i * cell + cell / 2}
              y={HEADER_HEIGHT - 4}
              textAnchor="middle"
              className="fill-muted-foreground text-[9px]"
            >
              {i + 1}
            </text>
          ) : null}
        </g>
      ))}
      {model.blocks.map((block) => {
        const positions = block.map((id) => index.get(id)!).sort((a, b) => a - b)
        const start = positions[0]
        const span = positions.length
        return (
          <rect
            key={block.join('|')}
            data-cycle-block={block.join(' ')}
            x={LABEL_GUTTER + start * cell}
            y={HEADER_HEIGHT + start * cell}
            width={span * cell}
            height={span * cell}
            fill="none"
            className="stroke-foreground"
            strokeWidth="2"
          />
        )
      })}
      {model.order.map((_, r) => (
        <g key={r} {...nav.getRowProps(r)}>
          {model.cells
            .filter((c) => c.row === r)
            .map((c) => {
              const bucket = bucketIntensity(c.weight, domain)
              const x = LABEL_GUTTER + c.col * cell
              const y = HEADER_HEIGHT + r * cell
              const label = chartCopy('matrix.cellLabel', {
                from: model.order[c.row].label,
                to: model.order[c.col].label,
                weight: c.weight
              })
              return (
                <g key={c.col}>
                  <rect
                    {...nav.getCellProps(c.row, c.col)}
                    aria-label={c.backward ? `${label}, ${chartCopy('matrix.backEdge')}` : label}
                    x={x + 0.5}
                    y={y + 0.5}
                    width={cell - 1}
                    height={cell - 1}
                    style={{ fill: `var(--quality-heat-${bucket ?? 3})` }}
                    className="cursor-pointer stroke-border outline-none focus-visible:stroke-ring focus-visible:stroke-2"
                  />
                  {c.backward ? (
                    <polygon
                      data-backward
                      points={`${x + cell / 2},${y + 2} ${x + cell - 2},${y + cell - 2} ${x + 2},${y + cell - 2}`}
                      className="pointer-events-none"
                      style={{
                        fill: (bucket ?? 3) <= 3 ? 'var(--foreground)' : 'var(--background)'
                      }}
                    />
                  ) : null}
                </g>
              )
            })}
        </g>
      ))}
    </svg>
  )
}

export function DependencyMatrix({
  frame,
  nodes,
  edges,
  maxNodes = 60,
  onSelectCell
}: DependencyMatrixProps): React.JSX.Element {
  const lazy = useLazyChartMount<HTMLDivElement>()
  const model = useMemo(() => prepareMatrix(nodes, edges, maxNodes), [nodes, edges, maxNodes])
  const total = new Set(nodes.map((n) => n.id)).size
  const summary = describeMatrix({
    nodes: model.order.length,
    cells: model.cells.length,
    blocks: model.blocks.length
  })
  const blockOf = new Map(model.blocks.flatMap((b, i) => b.map((id) => [id, i + 1] as const)))
  const table = {
    caption: frame.title,
    columns: [
      { key: 'from', label: chartCopy('table.from') },
      { key: 'to', label: chartCopy('table.to') },
      { key: 'weight', label: chartCopy('table.weight'), align: 'end' as const },
      { key: 'note', label: chartCopy('table.note') }
    ],
    rows: model.cells.map((c) => ({
      from: model.order[c.row].label,
      to: model.order[c.col].label,
      weight: c.weight,
      note:
        c.backward || (blockOf.has(c.from) && blockOf.get(c.from) === blockOf.get(c.to))
          ? `${chartCopy('matrix.cycleGroup')}${c.backward ? `, ${chartCopy('matrix.backEdge')}` : ''}`
          : null
    }))
  }
  return (
    <div ref={lazy.ref}>
      <ChartFrame
        {...frame}
        status={
          !lazy.mounted ? 'loading' : model.cells.length === 0 ? 'empty' : (frame.status ?? 'ready')
        }
        emptyReason={frame.emptyReason ?? chartCopy('matrix.empty')}
        minHeight={frame.minHeight ?? 240}
        summary={summary}
        surface="custom"
        description={frame.description ?? chartCopy('matrix.readHint')}
        table={table}
        legend={
          <>
            {model.hidden > 0 ? (
              <p className="text-xs text-muted-foreground">
                {chartCopy('frame.showing', { shown: model.order.length - 1, total })}
              </p>
            ) : null}
            {model.blocks.length > 0 ? (
              <p className="text-xs text-foreground">{`▲ ${chartCopy('matrix.backEdge')}`}</p>
            ) : null}
            {frame.legend}
          </>
        }
      >
        {({ width }) => (
          <MatrixGrid model={model} width={width} summary={summary} onSelectCell={onSelectCell} />
        )}
      </ChartFrame>
    </div>
  )
}
