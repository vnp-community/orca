/**
 * DataFlowStepList.tsx — FE-CV-TASK-056-06
 *
 * Full-fidelity, keyboard-navigable list of the flow's steps (the accessible equivalent of the
 * diagram). Virtualized above 150 rows.
 */

import { useRef, useState } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { useRovingListKeys } from '../useRovingListKeys'
import type { DataFlowRow } from './data-flow-overlay'

export const DATA_FLOW_VIRTUALIZE_ABOVE = 150
const ROW_HEIGHT = 44

function RowBody({ row }: { row: DataFlowRow }): React.JSX.Element {
  if (row.type === 'gap') {
    return (
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <TriangleAlert className="size-3.5 shrink-0" aria-hidden="true" />
        <span>{translate('auto.components.reviewMap.dataflow.gapAfter', 'Data is missing after step {{n}}', { n: row.afterStep })}</span>
        <span className="truncate">{row.message}</span>
      </div>
    )
  }
  const { step, flags, stores } = row
  return (
    <div className="grid grid-cols-[2rem_minmax(0,1.4fr)_5rem_minmax(0,1fr)_minmax(0,1fr)] items-center gap-2 text-xs">
      <span className="tabular-nums text-muted-foreground">{step.n}</span>
      <span className="truncate">{step.from.name} → {step.to.name}</span>
      <span>{step.kind}{step.sync ? '' : ` · ${translate('auto.components.reviewMap.dataflow.async', 'async')}`}</span>
      <span className="truncate">{step.method ?? step.symbol?.name ?? ''}</span>
      <span className="flex flex-wrap items-center gap-x-2 text-[11px] text-muted-foreground">
        {stores.length > 0 ? <span>{stores.map((s) => `${s.op} ${s.table ?? s.store.name}`).join(', ')}</span> : null}
        {step.origin !== 'declared' ? <span>{translate('auto.components.reviewMap.dataflow.inferred', 'Inferred')}</span> : null}
        {flags.changed ? <span className="text-foreground">● {translate('auto.components.reviewMap.dataflow.changed', 'changed')}</span> : null}
        {flags.untested ? <span>{translate('auto.components.reviewMap.dataflow.untested', 'untested')}</span> : null}
        {flags.unimplemented ? <span>{translate('auto.components.reviewMap.dataflow.unimplemented', 'not implemented')}</span> : null}
      </span>
    </div>
  )
}

export function DataFlowStepList({
  rows,
  onActivate
}: {
  rows: readonly DataFlowRow[]
  onActivate: (step: Extract<DataFlowRow, { type: 'step' }>) => void
}): React.JSX.Element {
  const [active, setActive] = useState(0)
  const parentRef = useRef<HTMLDivElement | null>(null)
  const virtual = rows.length > DATA_FLOW_VIRTUALIZE_ABOVE
  const { onKeyDown } = useRovingListKeys({
    count: rows.length,
    activeIndex: active,
    onActiveChange: setActive,
    onActivate: (i) => {
      const row = rows[i]
      if (row?.type === 'step') {
        onActivate(row)
      }
    }
  })
  const virtualizer = useVirtualizer({
    count: virtual ? rows.length : 0,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 8
  })

  const renderRow = (row: DataFlowRow, i: number, style?: React.CSSProperties): React.JSX.Element => (
    <div
      key={row.type === 'step' ? `s${row.step.n}` : `g${row.afterStep}-${i}`}
      id={`dataflow-row-${i}`}
      role="option"
      aria-selected={i === active}
      aria-posinset={i + 1}
      aria-setsize={rows.length}
      data-row-type={row.type}
      data-changed={row.type === 'step' && row.flags.changed ? 'true' : undefined}
      style={style}
      className={cn(
        'cursor-pointer border-l-2 border-transparent px-2 py-1.5',
        i === active && 'bg-accent',
        row.type === 'step' && row.flags.changed && 'border-review-changed',
        row.type === 'step' && !row.flags.inDiagram && 'opacity-60'
      )}
      onClick={() => {
        setActive(i)
        if (row.type === 'step') {
          onActivate(row)
        }
      }}
    >
      <RowBody row={row} />
    </div>
  )

  return (
    <div
      ref={parentRef}
      role="listbox"
      tabIndex={0}
      aria-label={translate('auto.components.reviewMap.dataflow.steps', 'Flow steps')}
      aria-activedescendant={rows.length > 0 ? `dataflow-row-${active}` : undefined}
      onKeyDown={onKeyDown}
      className="max-h-[60vh] min-h-0 overflow-auto rounded-md border focus-visible:ring-2 focus-visible:ring-ring"
    >
      {virtual ? (
        <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
          {virtualizer.getVirtualItems().map((v) =>
            renderRow(rows[v.index], v.index, { position: 'absolute', top: 0, left: 0, right: 0, transform: `translateY(${v.start}px)`, height: ROW_HEIGHT })
          )}
        </div>
      ) : (
        rows.map((row, i) => renderRow(row, i))
      )}
    </div>
  )
}
