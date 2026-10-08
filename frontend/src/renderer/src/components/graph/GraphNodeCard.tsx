/**
 * GraphNodeCard — FE-REQ-TASK-032-05
 *
 * Risk shows as text + icon + border style, never color alone.
 *
 * @module components/graph/GraphNodeCard
 */

import React, { memo } from 'react'
import { Box, Database, FileCode, Folder, ListChecks, Server } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { RiskBadge } from './RiskBadge'
import { riskPresentation } from './risk-presentation'
import { graphStatusPresentation } from './graph-status-presentation'
import type { GraphNode } from '../../../../shared/graph-types'

export type GraphNavigateDirection = 'up' | 'down' | 'left' | 'right'

type Props = {
  node: GraphNode
  /** Edge change touching this node, shown as a text sign. */
  change?: 'added' | 'removed' | 'unchanged'
  selected?: boolean
  dimmed?: boolean
  onOpen?: (id: string) => void
  onNavigate?: (id: string, direction: GraphNavigateDirection) => void
  onSelect?: (id: string) => void
  /** Semantic-zoom detail level; defaults to full. */
  detail?: 'full' | 'compact' | 'minimal'
}

const KIND_ICONS: Record<string, LucideIcon> = {
  service: Server, module: Folder, package: Folder, file: FileCode, rpc: FileCode, schema: FileCode,
  table: Database, column: Database, store: Database, task: ListChecks, phase: ListChecks
}

const ARROWS: Record<string, GraphNavigateDirection> = {
  ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right'
}

function GraphNodeCardBase({ node, change = 'unchanged', selected, dimmed, onOpen, onNavigate, onSelect, detail = 'full' }: Props): React.JSX.Element {
  const pres = riskPresentation(node.risk)
  const KindIcon = KIND_ICONS[node.kind] ?? Box
  const status = node.status ? graphStatusPresentation(node.status) : null
  const statusLabel = status?.labelKey ? translate(status.labelKey, status.fallback) : node.status
  const riskLabel = translate(pres.labelKey, pres.labelFallback)
  const ariaLabel = translate('auto.components.graph.Node.ariaLabel', '{{label}}, {{kind}}, risk {{risk}}{{status}}', {
    label: node.label,
    kind: node.kind,
    risk: riskLabel,
    status: statusLabel ? `, ${statusLabel}` : ''
  })

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={ariaLabel}
      aria-pressed={selected ? true : undefined}
      data-graph-node-id={node.id}
      onClick={() => onSelect?.(node.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          onOpen?.(node.id)
        } else if (ARROWS[e.key]) {
          e.preventDefault()
          onNavigate?.(node.id, ARROWS[e.key])
        }
      }}
      className={cn(
        detail === 'minimal' ? 'w-[120px]' : 'w-[200px]',
        'rounded-md bg-card px-2 py-1.5 text-xs text-card-foreground outline-none transition-opacity focus-visible:ring-2 focus-visible:ring-ring',
        pres.borderWidthClass,
        pres.borderClass,
        pres.dashed && 'border-dashed',
        pres.doubleRing && 'outline-2 outline-offset-2 outline-risk-critical-border',
        selected && 'ring-2 ring-ring',
        dimmed && 'opacity-30'
      )}
    >
      <div className="flex items-center gap-1.5">
        <KindIcon className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
        <span className="min-w-0 flex-1 truncate font-medium" title={node.label}>
          {node.label}
        </span>
        {change === 'added' ? <span aria-hidden className="font-mono text-graph-edge-added">+</span> : null}
        {change === 'removed' ? <span aria-hidden className="font-mono text-graph-edge-removed">−</span> : null}
      </div>
      <div className={cn('mt-1 flex-wrap items-center gap-1', detail === 'minimal' ? 'hidden' : 'flex')}>
        <RiskBadge level={node.risk} size="sm" />
        {node.status && detail === 'full' ? (
          <span className="inline-flex items-center gap-0.5 rounded border border-border px-1 text-[10px] text-muted-foreground">
            {status?.Icon ? <status.Icon className="size-2.5" aria-hidden /> : null}
            {statusLabel}
          </span>
        ) : null}
      </div>
    </div>
  )
}

export const GraphNodeCard = memo(
  GraphNodeCardBase,
  (a, b) =>
    a.node.id === b.node.id &&
    a.node.risk === b.node.risk &&
    a.node.status === b.node.status &&
    a.node.label === b.node.label &&
    a.change === b.change &&
    a.selected === b.selected &&
    a.dimmed === b.dimmed &&
    a.detail === b.detail &&
    a.onOpen === b.onOpen &&
    a.onNavigate === b.onNavigate &&
    a.onSelect === b.onSelect
)
