/**
 * ContractChangeTable.tsx — FE-CV-TASK-059-04
 *
 * Four-column table: contract, before, after, compatibility. Before/after come from the
 * `details.before|after` convention; without it the row shows `change` and the first details.
 * Windowed above VIRTUALIZE_THRESHOLD rows.
 *
 * @module components/review-map/contract/ContractChangeTable
 */

import { useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import type { ContractChange } from '../../../../../shared/code-intel-types'
import { cn } from '@/lib/utils'
import { contractDetailRows } from './contract-detail-rows'
import { normalizeChangeKind } from './contract-grouping'
import { diffSignatureTokens } from './contract-signature-diff'
import { tc } from './contract-i18n'
import { ContractCompatibilityBadge } from './ContractCompatibilityBadge'
import { ContractSignatureCell } from './ContractSignatureCell'

export const CONTRACT_VIRTUALIZE_THRESHOLD = 100
const ROW_HEIGHT = 44
const GRID = 'grid grid-cols-[minmax(0,2fr)_minmax(0,2fr)_minmax(0,2fr)_minmax(0,1.2fr)] gap-2'

function changeLabel(kind: ReturnType<typeof normalizeChangeKind>): string {
  switch (kind) {
    case 'added':
      return tc('change.added', 'Added')
    case 'removed':
      return tc('change.removed', 'Removed')
    case 'modified':
      return tc('change.modified', 'Modified')
    default:
      return tc('change.unknown', 'Changed (unclassified)')
  }
}

function Row({
  change,
  selected,
  onSelect,
  style
}: {
  change: ContractChange
  selected: boolean
  onSelect: (id: string) => void
  style?: React.CSSProperties
}): React.JSX.Element {
  const detail = contractDetailRows(change)
  const diff = detail.signature ? diffSignatureTokens(detail.signature.before, detail.signature.after) : null
  const kind = normalizeChangeKind(change.change)
  return (
    <div
      role="row"
      aria-selected={selected}
      style={style}
      className={cn(GRID, 'items-start border-b px-2 py-1.5 text-xs', selected && 'bg-accent')}
    >
      <div role="cell" className="min-w-0">
        <button
          type="button"
          onClick={() => onSelect(change.id)}
          className="max-w-full truncate text-left font-medium underline-offset-2 hover:underline"
          title={change.name}
        >
          {change.name}
        </button>
        <div className="text-[11px] text-muted-foreground">
          {change.kind} · {changeLabel(kind)}
        </div>
      </div>
      {diff ? (
        <>
          <div role="cell" className="min-w-0">
            <ContractSignatureCell tokens={diff.before} />
          </div>
          <div role="cell" className="min-w-0">
            <ContractSignatureCell tokens={diff.after} />
          </div>
        </>
      ) : (
        <div role="cell" className="col-span-2 min-w-0 text-[11px] text-muted-foreground">
          {detail.rows.slice(0, 2).map((r) => (
            <div key={r.key} className="truncate">
              <span className="font-medium">{r.key}</span>: {r.value}
            </div>
          ))}
          {detail.rows.length === 0 ? tc('table.noDetails', 'No details provided') : null}
        </div>
      )}
      <div role="cell" className="min-w-0">
        <ContractCompatibilityBadge compatibility={change.compatibility} ruleId={change.ruleId} />
        {change.ruleId ? (
          <div className="mt-0.5 truncate text-[11px] text-muted-foreground" title={change.ruleId}>
            {change.ruleId}
          </div>
        ) : null}
      </div>
    </div>
  )
}

export function ContractChangeTable({
  changes,
  selectedId,
  onSelect
}: {
  changes: readonly ContractChange[]
  selectedId: string | null
  onSelect: (id: string) => void
}): React.JSX.Element {
  const parentRef = useRef<HTMLDivElement>(null)
  const virtualize = changes.length > CONTRACT_VIRTUALIZE_THRESHOLD
  const virtualizer = useVirtualizer({
    count: virtualize ? changes.length : 0,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 8
  })
  return (
    <div role="table" aria-label={tc('table.label', 'Contract changes')} className="min-w-0">
      <div role="row" className={cn(GRID, 'border-b px-2 py-1 text-[11px] font-medium text-muted-foreground')}>
        <div role="columnheader">{tc('table.col.contract', 'Contract')}</div>
        <div role="columnheader">{tc('table.col.before', 'Before')}</div>
        <div role="columnheader">{tc('table.col.after', 'After')}</div>
        <div role="columnheader">{tc('table.col.compat', 'Compatibility')}</div>
      </div>
      {virtualize ? (
        <div ref={parentRef} role="rowgroup" className="max-h-96 overflow-y-auto">
          <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
            {virtualizer.getVirtualItems().map((item) => (
              <Row
                key={changes[item.index].id}
                change={changes[item.index]}
                selected={changes[item.index].id === selectedId}
                onSelect={onSelect}
                style={{ position: 'absolute', top: 0, left: 0, right: 0, transform: `translateY(${item.start}px)` }}
              />
            ))}
          </div>
        </div>
      ) : (
        <div role="rowgroup">
          {changes.map((change) => (
            <Row key={change.id} change={change} selected={change.id === selectedId} onSelect={onSelect} />
          ))}
        </div>
      )}
    </div>
  )
}
