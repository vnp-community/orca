/**
 * GraphMini — FE-REQ-TASK-032-05
 *
 * Read-only thumbnail (max 12 nodes) drawn as plain SVG; always paired with a
 * text summary so the picture is never the only carrier of meaning.
 *
 * @module components/graph/GraphMini
 */

import React, { useMemo } from 'react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { computeWaveLayout } from './graph-layout-engine'
import { pickVisibleNodes } from './graph-grouping'
import { formatGraphSummary, summarizeGraph } from './graph-summary'
import { riskPresentation } from './risk-presentation'
import type { GraphPayload } from '../../../../shared/graph-types'

export const GRAPH_MINI_NODE_LIMIT = 12
const CELL_W = 44
const CELL_H = 26

type Props = {
  payload: GraphPayload | null
  /** True when a GraphListView of the same data is reachable; hides the drawing from AT. */
  hasListEquivalent?: boolean
  className?: string
}

export function GraphMini({ payload, hasListEquivalent = false, className }: Props): React.JSX.Element {
  const model = useMemo(() => {
    if (!payload) {return null}
    const { visible } = pickVisibleNodes(payload, { openGroups: new Set(), limit: GRAPH_MINI_NODE_LIMIT })
    const shown = visible.slice(0, GRAPH_MINI_NODE_LIMIT)
    const ids = new Set(shown.map((n) => n.id))
    const edges = payload.edges.filter((e) => ids.has(e.from) && ids.has(e.to))
    const pos = computeWaveLayout(shown, edges, { direction: 'LR', groupOf: () => null })
    return { shown, edges, pos, summary: formatGraphSummary(summarizeGraph(payload), translate) }
  }, [payload])

  if (!model) {
    return (
      <p className={cn('text-xs text-muted-foreground', className)}>
        {translate('auto.components.graph.Mini.empty', 'Not assessed')}
      </p>
    )
  }

  const xs = Object.values(model.pos)
  const width = Math.max(...xs.map((p) => p.x), 0) + CELL_W
  const height = Math.max(...xs.map((p) => p.y), 0) + CELL_H
  // Why: wave gaps are 220x90; scale down to thumbnail cells.
  const sx = (x: number): number => (x / 220) * CELL_W
  const sy = (y: number): number => (y / 90) * CELL_H

  return (
    <div className={cn('flex flex-col gap-1', className)}>
      <svg
        aria-hidden={hasListEquivalent ? true : undefined}
        role={hasListEquivalent ? undefined : 'img'}
        aria-label={hasListEquivalent ? undefined : model.summary}
        viewBox={`0 0 ${Math.max(width, CELL_W)} ${Math.max(height, CELL_H)}`}
        className="h-16 w-full"
      >
        {model.edges.map((e, i) => (
          <line
            key={i}
            x1={sx(model.pos[e.from].x) + 8}
            y1={sy(model.pos[e.from].y) + 8}
            x2={sx(model.pos[e.to].x) + 8}
            y2={sy(model.pos[e.to].y) + 8}
            className="stroke-border"
            strokeDasharray={e.change === 'removed' ? '3 2' : undefined}
          />
        ))}
        {model.shown.map((n) => {
          const pres = riskPresentation(n.risk)
          return (
            <rect
              key={n.id}
              x={sx(model.pos[n.id].x)}
              y={sy(model.pos[n.id].y)}
              width={16}
              height={16}
              rx={3}
              data-risk-level={pres.level}
              className={cn('fill-card', pres.dashed ? 'stroke-muted-foreground' : 'stroke-foreground')}
              strokeWidth={pres.borderWidthClass === 'border-2' ? 2 : 1}
              strokeDasharray={pres.dashed ? '2 2' : undefined}
            />
          )
        })}
      </svg>
      <p className="text-xs text-muted-foreground">{model.summary}</p>
    </div>
  )
}
