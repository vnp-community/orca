/**
 * GraphGroupNode — FE-REQ-TASK-032-05
 *
 * Folded "+N unaffected" summary. Opening a group moves its members into the
 * visible set (no nested xyflow nodes in the first version).
 *
 * @module components/graph/GraphGroupNode
 */

import React from 'react'
import { Layers } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { RiskBadge } from './RiskBadge'
import type { GraphGroup } from './graph-grouping'

type Props = {
  group: GraphGroup
  onToggleGroup: (groupId: string) => void
}

export function GraphGroupNode({ group, onToggleGroup }: Props): React.JSX.Element {
  const kinds = Object.entries(group.byKind)
    .map(([kind, n]) => `${n} ${kind}`)
    .join(', ')
  const summary = translate('auto.components.graph.GroupSummary', '+{{count}} not affected ({{kinds}})', {
    count: group.count,
    kinds
  })
  return (
    <button
      type="button"
      data-graph-group-id={group.id}
      aria-label={`${group.label}: ${summary}`}
      onClick={() => onToggleGroup(group.id)}
      className="w-[200px] rounded-md border border-dashed border-border bg-muted px-2 py-1.5 text-left text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <span className="flex items-center gap-1.5">
        <Layers className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
        <span className="min-w-0 flex-1 truncate font-medium" title={group.label}>
          {group.label}
        </span>
      </span>
      <span className="mt-1 flex flex-wrap items-center gap-1 text-muted-foreground">
        <span>{summary}</span>
        <RiskBadge level={group.maxRisk} size="sm" />
      </span>
    </button>
  )
}
