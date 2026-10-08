/**
 * ErdTableNode.tsx — FE-CV-TASK-057-05
 *
 * xyflow node for one table: header, columns (collapsed to a bounded set), "+N" toggle.
 * Enter selects the table; the "+N" button toggles expansion.
 */

import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Table2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import {
  ERD_CHANGE_BORDER_CLASS,
  ERD_CHANGE_LABEL,
  ERD_CHANGE_TEXT_CLASS
} from './erd-change-style'
import { ErdColumnRow } from './ErdColumnRow'
import { ERD_COLLAPSED_COLUMN_LIMIT, ERD_HEADER_HEIGHT, ERD_NODE_WIDTH } from './erd-layout'
import { selectCollapsedColumns } from './erd-table-filter'
import type { ErdChangeKind } from './erd-column-changes'
import type { ErdTableView } from './erd-view-model'

export type ErdTableNodeData = {
  table: ErdTableView
  expanded: boolean
  query: string
  dimmed: boolean
  onSelect: (key: string) => void
  onToggleExpand: (key: string) => void
}

export function ErdTableNode({ data, selected }: NodeProps): React.JSX.Element {
  const { table, expanded, query, dimmed, onSelect, onToggleExpand } = data as ErdTableNodeData
  const { shown, hidden } = selectCollapsedColumns(table.columns, query, expanded)
  const change = table.tableChange
  const changeKind = (
    change === 'added' || change === 'modified' || change === 'removed' ? change : null
  ) as ErdChangeKind | null
  const stateLabel = changeKind
    ? translate(ERD_CHANGE_LABEL[changeKind].key, ERD_CHANGE_LABEL[changeKind].fallback)
    : change === 'adjacent'
      ? translate('auto.components.reviewMap.ErdTableNode.adjacent', 'Related')
      : null
  const aria = translate(
    'auto.components.reviewMap.ErdTableNode.ariaLabel',
    'Table {{name}}, {{count}} columns{{state}}',
    {
      name: table.name,
      count: table.columns.length,
      state: stateLabel ? `, ${stateLabel}` : ''
    }
  )
  return (
    <div
      role="group"
      tabIndex={0}
      aria-label={aria}
      data-testid={`erd-table-${table.key}`}
      onKeyDown={(e) => {
        if (e.key === 'Enter' && e.target === e.currentTarget) {
          onSelect(table.key)
        }
      }}
      className={cn(
        'rounded-md border bg-card text-card-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring',
        changeKind
          ? ERD_CHANGE_BORDER_CLASS[changeKind]
          : change === 'adjacent'
            ? 'border-muted-foreground/40'
            : 'border-border',
        dimmed && 'opacity-40',
        selected && 'ring-2 ring-ring'
      )}
      style={{ width: ERD_NODE_WIDTH }}
    >
      <Handle type="target" position={Position.Left} isConnectable={false} className="opacity-0" />
      <Handle type="source" position={Position.Right} isConnectable={false} className="opacity-0" />
      <div className="flex items-center gap-1 border-b px-2" style={{ height: ERD_HEADER_HEIGHT }}>
        <Table2 className="size-3.5 shrink-0" aria-hidden="true" />
        <span className="truncate text-xs font-semibold">{table.name}</span>
        <span className="truncate text-[10px] text-muted-foreground">{table.schema}</span>
        {stateLabel ? (
          <Badge
            variant="outline"
            className={cn(
              'ml-auto shrink-0 text-[10px]',
              changeKind && ERD_CHANGE_TEXT_CLASS[changeKind]
            )}
          >
            {table.changeSymbol ? <span aria-hidden="true">{table.changeSymbol}</span> : null}
            {stateLabel}
          </Badge>
        ) : null}
      </div>
      {table.dropped ? (
        <p className="px-2 py-1 text-[11px] text-muted-foreground">
          {translate(
            'auto.components.reviewMap.ErdTableNode.dropped',
            'Table dropped by this change'
          )}
        </p>
      ) : table.fromTouchedOnly ? (
        <p className="px-2 py-1 text-[11px] text-muted-foreground">
          {translate(
            'auto.components.reviewMap.ErdTableNode.noColumnDetail',
            'No column detail available'
          )}
        </p>
      ) : (
        <ul>
          {shown.map((c) => (
            <ErdColumnRow key={`${c.name}:${c.ghost ? 'g' : ''}`} column={c} />
          ))}
        </ul>
      )}
      {table.columns.length > ERD_COLLAPSED_COLUMN_LIMIT ? (
        <button
          type="button"
          aria-expanded={expanded}
          onClick={() => onToggleExpand(table.key)}
          className="w-full px-2 py-0.5 text-left text-[11px] text-muted-foreground hover:text-foreground"
        >
          {expanded
            ? translate('auto.components.reviewMap.ErdTableNode.fewerColumns', 'Show fewer columns')
            : translate(
                'auto.components.reviewMap.ErdTableNode.moreColumns',
                '+{{count}} more columns',
                { count: hidden }
              )}
        </button>
      ) : null}
    </div>
  )
}
