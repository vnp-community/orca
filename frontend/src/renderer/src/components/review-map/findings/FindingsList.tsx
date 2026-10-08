/**
 * FindingsList.tsx — FE-CV-TASK-059-05
 *
 * Findings grouped by kind; above VIRTUALIZE_THRESHOLD rows the list is flat and windowed.
 * Shows "showing X of Y" and a Load more button while the server has more pages.
 *
 * @module components/review-map/findings/FindingsList
 */

import { useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { groupFindingsByKind, sortFindings } from './finding-sort'
import type { FindingRowModel } from './finding-view-model'
import { tf } from './findings-i18n'
import { kindLabel } from './FindingsToolbar'

export const FINDINGS_VIRTUALIZE_THRESHOLD = 50

export function FindingsList({
  rows,
  renderRow,
  hasNextPage,
  isFetchingNextPage,
  nextPageFailed,
  onLoadMore
}: {
  rows: readonly FindingRowModel[]
  renderRow: (row: FindingRowModel) => React.ReactNode
  hasNextPage: boolean
  isFetchingNextPage: boolean
  nextPageFailed: boolean
  onLoadMore: () => void
}): React.JSX.Element {
  const parentRef = useRef<HTMLDivElement>(null)
  const virtualize = rows.length > FINDINGS_VIRTUALIZE_THRESHOLD
  const flat = virtualize ? sortFindings(rows) : []
  const virtualizer = useVirtualizer({
    count: flat.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 76,
    overscan: 8
  })

  return (
    <div className="min-h-0 flex-1 overflow-y-auto" ref={parentRef}>
      {virtualize ? (
        <div role="list" style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
          {virtualizer.getVirtualItems().map((item) => (
            <div
              key={flat[item.index].findingKey}
              ref={virtualizer.measureElement}
              data-index={item.index}
              style={{ position: 'absolute', top: 0, left: 0, right: 0, transform: `translateY(${item.start}px)` }}
            >
              {renderRow(flat[item.index])}
            </div>
          ))}
        </div>
      ) : (
        groupFindingsByKind(rows).map((group) => (
          <section key={group.kind} aria-label={kindLabel(group.kind)}>
            <h3 className="bg-muted/40 px-2 py-1 text-[11px] font-medium text-muted-foreground">
              {kindLabel(group.kind)} ({group.rows.length})
            </h3>
            <div role="list">{group.rows.map((row) => <div key={row.findingKey}>{renderRow(row)}</div>)}</div>
          </section>
        ))
      )}
      {hasNextPage ? (
        <div className="flex items-center gap-2 p-2 text-xs text-muted-foreground">
          <span role="status">{tf('list.partial', 'Showing {{count}} loaded findings. Narrow the filters or load more.', { count: rows.length })}</span>
          <Button type="button" variant="outline" size="xs" disabled={isFetchingNextPage} onClick={onLoadMore}>
            {isFetchingNextPage ? <Loader2 className="animate-spin" aria-hidden /> : null}
            {nextPageFailed ? tf('retry', 'Retry') : tf('list.loadMore', 'Load more')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
