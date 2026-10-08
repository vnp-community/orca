/**
 * GraphListView — FE-REQ-TASK-032-06
 *
 * Peer of the canvas (not an appendix): same payload, accessible grid,
 * virtualized above 100 rows.
 *
 * @module components/graph/GraphListView
 */

import React, { useMemo, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { RiskBadge } from './RiskBadge'
import { GRAPH_RISK_ORDER } from '../../../../shared/graph-types'
import type { GraphNode, GraphPayload } from '../../../../shared/graph-types'

export const LIST_VIRTUALIZE_THRESHOLD = 100
export const LIST_ROW_HEIGHT = 40
const OVERSCAN = 8
const T = 'auto.components.graph.List.'

type Props = {
  payload: GraphPayload
  selectedId: string | null
  onSelect: (id: string) => void
  onOpen: (id: string) => void
}

type Row = { node: GraphNode; change: 'added' | 'removed' | 'unchanged'; relations: number }

export function buildGraphListRows(payload: GraphPayload): Row[] {
  const relations = new Map<string, number>()
  const change = new Map<string, 'added' | 'removed'>()
  for (const e of payload.edges) {
    relations.set(e.from, (relations.get(e.from) ?? 0) + 1)
    relations.set(e.to, (relations.get(e.to) ?? 0) + 1)
    if (e.change !== 'unchanged') {
      change.set(e.from, change.get(e.from) ?? e.change)
      change.set(e.to, change.get(e.to) ?? e.change)
    }
  }
  return payload.nodes
    .map((node) => ({ node, change: change.get(node.id) ?? ('unchanged' as const), relations: relations.get(node.id) ?? 0 }))
    .sort((a, b) => {
      const r = GRAPH_RISK_ORDER[b.node.risk] - GRAPH_RISK_ORDER[a.node.risk]
      return r !== 0 ? r : a.node.label.localeCompare(b.node.label)
    })
}

const CHANGE_LABEL = {
  added: ['Change.added', 'Added'],
  removed: ['Change.removed', 'Removed'],
  unchanged: ['Change.unchanged', 'Unchanged']
} as const

const COLS = 'grid grid-cols-[80px_minmax(0,2fr)_minmax(0,1fr)_120px_100px_90px_60px] items-center gap-2 px-3'

export function GraphListView({ payload, selectedId, onSelect, onOpen }: Props): React.JSX.Element {
  const rows = useMemo(() => buildGraphListRows(payload), [payload])
  const scrollRef = useRef<HTMLDivElement>(null)
  const virtualize = rows.length > LIST_VIRTUALIZE_THRESHOLD
  const virtualizer = useVirtualizer({
    count: virtualize ? rows.length : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => LIST_ROW_HEIGHT,
    overscan: OVERSCAN
  })

  const renderRow = (row: Row, index: number, style?: React.CSSProperties): React.JSX.Element => {
    const [changeKey, changeFallback] = CHANGE_LABEL[row.change]
    const selected = row.node.id === selectedId
    return (
      <div
        key={row.node.id}
        role="row"
        aria-rowindex={index + 2}
        aria-selected={selected}
        tabIndex={selected || (selectedId === null && index === 0) ? 0 : -1}
        data-graph-row-id={row.node.id}
        style={style}
        onClick={() => onSelect(row.node.id)}
        onKeyDown={(e) => {
          const move = (delta: number): void => {
            const next = rows[index + delta]
            if (!next) {return}
            e.preventDefault()
            onSelect(next.node.id)
            requestAnimationFrame(() => {
              document.querySelector<HTMLElement>(`[data-graph-row-id="${CSS.escape(next.node.id)}"]`)?.focus()
            })
          }
          if (e.key === 'ArrowDown') {move(1)}
          else if (e.key === 'ArrowUp') {move(-1)}
          else if (e.key === 'Enter') {
            e.preventDefault()
            onOpen(row.node.id)
          }
        }}
        className={cn(COLS, 'h-10 cursor-pointer border-b border-border text-xs outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring', selected && 'bg-accent')}
      >
        <span role="gridcell" className="truncate text-muted-foreground">{row.node.kind}</span>
        <span role="gridcell" className="truncate font-medium" title={row.node.label}>{row.node.label}</span>
        <span role="gridcell" className="truncate text-muted-foreground">{row.node.group ?? ''}</span>
        <span role="gridcell"><RiskBadge level={row.node.risk} size="sm" /></span>
        <span role="gridcell" className="truncate">{row.node.status ?? ''}</span>
        <span role="gridcell">{translate(`${T}${changeKey}`, changeFallback)}</span>
        <span role="gridcell" className="text-right tabular-nums">{row.relations}</span>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col" data-testid="graph-list">
      {payload.truncated ? (
        <p className="px-3 py-1 text-xs text-muted-foreground">
          {translate(`${T}shown`, 'Showing {{shown}} of {{total}}', { shown: rows.length, total: payload.totalNodes })}
        </p>
      ) : null}
      <div role="grid" aria-rowcount={rows.length + 1} className="flex min-h-0 flex-1 flex-col">
        <div role="row" aria-rowindex={1} className={cn(COLS, 'h-8 border-b border-border text-[11px] font-medium text-muted-foreground')}>
          {(['kind', 'label', 'group', 'risk', 'status', 'change', 'relations'] as const).map((c) => (
            <span key={c} role="columnheader">{translate(`${T}col.${c}`, c)}</span>
          ))}
        </div>
        <div ref={scrollRef} className="min-h-0 flex-1 overflow-auto">
          {virtualize ? (
            <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
              {virtualizer.getVirtualItems().map((v) =>
                renderRow(rows[v.index], v.index, { position: 'absolute', top: 0, left: 0, width: '100%', height: LIST_ROW_HEIGHT, transform: `translateY(${v.start}px)` })
              )}
            </div>
          ) : (
            rows.map((r, i) => renderRow(r, i))
          )}
        </div>
      </div>
    </div>
  )
}
