/**
 * ErdTableList.tsx — FE-CV-TASK-057-05
 *
 * Text equivalent of the diagram and the keyboard / screen-reader way through large
 * schemas. Virtualised past ERD_LIST_VIRTUAL_THRESHOLD rows.
 */

import { useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { ERD_CHANGE_TEXT_CLASS } from './erd-change-style'
import type { ErdChangeKind } from './erd-column-changes'
import type { ErdTableView } from './erd-view-model'

export const ERD_LIST_VIRTUAL_THRESHOLD = 60
const ROW_HEIGHT = 44

function Row({
  table,
  selected,
  dimmed,
  onSelect
}: {
  table: ErdTableView
  selected: boolean
  dimmed: boolean
  onSelect: (key: string) => void
}): React.JSX.Element {
  const kind = (['added', 'modified', 'removed'] as const).find((k) => k === table.tableChange) as
    | ErdChangeKind
    | undefined
  return (
    <button
      type="button"
      aria-current={selected ? 'true' : undefined}
      data-testid={`erd-list-${table.key}`}
      onClick={() => onSelect(table.key)}
      className={cn(
        'flex h-11 w-full items-center gap-2 border-b px-3 text-left text-xs hover:bg-accent',
        selected && 'bg-accent',
        dimmed && 'opacity-50'
      )}
    >
      <span
        className={cn('w-3 shrink-0 text-center font-mono', kind && ERD_CHANGE_TEXT_CLASS[kind])}
        aria-hidden="true"
      >
        {table.changeSymbol ?? ''}
      </span>
      <span className="truncate font-medium">{table.name}</span>
      <span className="truncate text-muted-foreground">{table.schema}</span>
      <span className="ml-auto shrink-0 text-muted-foreground">
        {translate(
          'auto.components.reviewMap.ErdTableList.summary',
          '{{columns}} columns · {{reads}} read · {{writes}} write',
          {
            columns: table.columns.length,
            reads: table.accessCount.read,
            writes: table.accessCount.write
          }
        )}
      </span>
    </button>
  )
}

export function ErdTableList({
  tables,
  selectedKey,
  dimmed,
  onSelect
}: {
  tables: readonly ErdTableView[]
  selectedKey: string | null
  dimmed: ReadonlySet<string>
  onSelect: (key: string) => void
}): React.JSX.Element {
  const parent = useRef<HTMLDivElement>(null)
  const virtual = tables.length > ERD_LIST_VIRTUAL_THRESHOLD
  const virtualizer = useVirtualizer({
    count: virtual ? tables.length : 0,
    getScrollElement: () => parent.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 8
  })
  const label = translate('auto.components.reviewMap.ErdTableList.label', 'Tables')
  return (
    <div ref={parent} className="min-h-0 flex-1 overflow-y-auto" role="region" aria-label={label}>
      {virtual ? (
        <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
          {virtualizer.getVirtualItems().map((v) => (
            <div
              key={tables[v.index].key}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                right: 0,
                transform: `translateY(${v.start}px)`
              }}
            >
              <Row
                table={tables[v.index]}
                selected={tables[v.index].key === selectedKey}
                dimmed={dimmed.has(tables[v.index].key)}
                onSelect={onSelect}
              />
            </div>
          ))}
        </div>
      ) : (
        tables.map((t) => (
          <Row
            key={t.key}
            table={t}
            selected={t.key === selectedKey}
            dimmed={dimmed.has(t.key)}
            onSelect={onSelect}
          />
        ))
      )}
    </div>
  )
}
