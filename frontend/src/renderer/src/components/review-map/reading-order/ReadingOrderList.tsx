import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type { PerceivedLoadingStage } from '@/hooks/usePerceivedLoadingStage'
import type { ReviewProgressSaveStatus } from '@/store/slices/review-progress'
import {
  buildReadingOrderRows,
  countReadingOrderTotals,
  type ReadingOrderItem,
  type ReadingOrderRow as Row
} from '../reading-order-model'
import type { ChangeOverlayView, ReadingProgress } from '../review-wire-types'
import { computeReadingStepOverlayFlags } from '../review-overlay-model'
import { useRovingListKeys } from '../useRovingListKeys'
import { ReviewLoadingStage } from '../shell/ReviewLoadingStage'
import { ReadingOrderGroupHeader } from './ReadingOrderGroupHeader'
import { ReadingOrderRow } from './ReadingOrderRow'
import { ReadingProgressBar } from './ReadingProgressBar'

export const READING_ORDER_VIRTUALIZE_AFTER = 150
const GROUP_ROW_PX = 28
const STEP_ROW_PX = 40

export type ReadingOrderListProps = {
  items: readonly ReadingOrderItem[]
  /** Number of changed files, to tell "no changes" from "could not build an order". */
  changedFileCount: number
  progress: ReadingProgress
  saveStatus: ReviewProgressSaveStatus
  localOnly?: boolean
  oversize?: boolean
  /** Progress load state: a failed load must not block reading. */
  progressLoadFailed?: boolean
  loadingStage?: PerceivedLoadingStage
  filterFiles: ReadonlySet<string> | null
  filterLabel?: string
  onClearFilter: () => void
  onSetSeen: (stepKey: string, seen: boolean) => void
  onSetGroupSeen: (stepKeys: readonly string[], seen: boolean) => void
  onFocusChange: (stepKey: string | null) => void
  onOpenDiff: (path: string, line?: number) => void
  onRetrySave: () => void
  onRetryProgressLoad: () => void
  /** Focus lands on lastFocusedKey / first unseen step only on deliberate open. */
  initialFocusKey?: string | null
  /** Overlay data for per-row untested/violation marks; absent => no marks. */
  overlay?: ChangeOverlayView | null
  /** `shown`/`total` when the backend capped the step list. */
  overflow?: { shown: number; total: number } | null
}

function firstHunkLine(item: ReadingOrderItem): number | undefined {
  return item.hunks[0]?.startLine ?? item.symbols[0]?.startLine
}

export function ReadingOrderList(props: ReadingOrderListProps): React.JSX.Element {
  const {
    items,
    changedFileCount,
    progress,
    saveStatus,
    filterFiles,
    loadingStage = 'idle',
    onFocusChange,
    onOpenDiff,
    onSetSeen,
    onSetGroupSeen,
    initialFocusKey
  } = props
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set())
  const [activeIndex, setActiveIndex] = useState(0)
  const scrollRef = useRef<HTMLDivElement>(null)

  const rows = useMemo(
    () =>
      buildReadingOrderRows(items, { collapsedGroupIds: collapsed, filter: filterFiles, progress }),
    [items, collapsed, filterFiles, progress]
  )
  const totals = useMemo(() => countReadingOrderTotals(items, progress), [items, progress])
  const overlayFlagsByStep = useMemo(() => {
    const map = new Map<string, ReturnType<typeof computeReadingStepOverlayFlags>>()
    if (props.overlay) {
      for (const item of items) {
        map.set(item.stepKey, computeReadingStepOverlayFlags(item, props.overlay))
      }
    }
    return map
  }, [items, props.overlay])
  const visibleSteps = rows.filter((r) => r.type === 'step').length

  // Deliberate-open focus: last focused step, else first unseen.
  const focusApplied = useRef(false)
  useEffect(() => {
    if (focusApplied.current || initialFocusKey === undefined || rows.length === 0) {
      return
    }
    focusApplied.current = true
    const byKey = initialFocusKey
      ? rows.findIndex((r) => r.type === 'step' && r.id === initialFocusKey)
      : -1
    const firstUnseen = rows.findIndex(
      (r) => r.type === 'step' && progress.entries[r.id]?.state !== 'seen'
    )
    const idx = byKey >= 0 ? byKey : firstUnseen
    if (idx >= 0) {
      setActiveIndex(idx)
    }
  }, [initialFocusKey, rows, progress])

  const virtualized = rows.length > READING_ORDER_VIRTUALIZE_AFTER
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: (i) => (rows[i]?.type === 'group' ? GROUP_ROW_PX : STEP_ROW_PX),
    overscan: 8,
    initialRect: { width: 320, height: 600 },
    enabled: virtualized
  })

  const activeRow: Row | undefined = rows[Math.min(activeIndex, rows.length - 1)]

  const setActive = useCallback(
    (i: number) => {
      setActiveIndex(i)
      const r = rows[i]
      if (r?.type === 'step') {
        onFocusChange(r.id)
      }
      if (virtualized) {
        virtualizer.scrollToIndex(i)
      }
    },
    [rows, onFocusChange, virtualized, virtualizer]
  )

  const toggleGroupCollapsed = (id: string, collapse: boolean): void =>
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (collapse) {
        next.add(id)
      } else {
        next.delete(id)
      }
      return next
    })

  const toggleStep = (r: Extract<Row, { type: 'step' }>): void =>
    onSetSeen(r.id, progress.entries[r.id]?.state !== 'seen')

  const toggleGroupSeen = (r: Extract<Row, { type: 'group' }>): void =>
    onSetGroupSeen(r.stepKeys, r.seen !== r.total)

  const { onKeyDown } = useRovingListKeys({
    count: rows.length,
    activeIndex,
    onActiveChange: setActive,
    // Enter opens the diff only; it never marks the step as read.
    onActivate: (i) => {
      const r = rows[i]
      if (r?.type === 'step') {
        onOpenDiff(r.item.file, firstHunkLine(r.item))
      }
    },
    onToggle: (i) => {
      const r = rows[i]
      if (!r) {
        return
      }
      if (r.type === 'step') {
        toggleStep(r)
      } else {
        toggleGroupSeen(r)
      }
    },
    isGroupRow: (i) => rows[i]?.type === 'group',
    onCollapse: (i, collapse) => {
      const r = rows[i]
      if (r?.type === 'group') {
        toggleGroupCollapsed(r.id, collapse)
      }
    }
  })

  if (loadingStage !== 'idle' && items.length === 0) {
    return <ReviewLoadingStage stage={loadingStage} rows={8} />
  }

  if (items.length === 0) {
    return (
      <div className="p-4 text-sm text-muted-foreground" data-state="empty">
        {changedFileCount > 0
          ? translate(
              'auto.components.reviewMap.readingOrder.noOrder',
              'No reading order could be built from the index.'
            )
          : translate('auto.components.reviewMap.readingOrder.empty', 'No changes in this scope.')}
      </div>
    )
  }

  const domId = (r: Row): string => `ro-${r.type}-${r.id}`
  const renderRow = (r: Row, i: number): React.JSX.Element =>
    r.type === 'group' ? (
      <ReadingOrderGroupHeader
        key={domId(r)}
        row={r}
        domId={domId(r)}
        active={i === activeIndex}
        onSelect={() => setActive(i)}
        onToggleCollapsed={() => toggleGroupCollapsed(r.id, !r.collapsed)}
        onToggleSeen={() => toggleGroupSeen(r)}
      />
    ) : (
      <ReadingOrderRow
        key={domId(r)}
        item={r.item}
        domId={domId(r)}
        overlayFlags={overlayFlagsByStep.get(r.id)}
        seen={progress.entries[r.id]?.state === 'seen'}
        active={i === activeIndex}
        onSelect={() => setActive(i)}
        onToggleSeen={() => toggleStep(r)}
        onOpenDiff={() => onOpenDiff(r.item.file, firstHunkLine(r.item))}
      />
    )

  return (
    <div className="flex h-full min-h-0 flex-col" data-testid="reading-order">
      <ReadingProgressBar
        seen={totals.seen}
        total={totals.total}
        saveStatus={saveStatus}
        localOnly={props.localOnly}
        oversize={props.oversize}
        onRetrySave={props.onRetrySave}
      />
      {props.progressLoadFailed ? (
        <p
          className="flex items-center gap-1 px-3 pb-1 text-xs text-muted-foreground"
          role="status"
        >
          {translate(
            'auto.components.reviewMap.readingOrder.progressLoadFailed',
            'Saved progress could not be loaded.'
          )}
          <Button type="button" size="xs" variant="ghost" onClick={props.onRetryProgressLoad}>
            {translate('auto.components.reviewMap.readingOrder.retry', 'Retry')}
          </Button>
        </p>
      ) : null}
      {filterFiles ? (
        <div
          className="flex items-center gap-1 px-3 pb-1 text-xs"
          data-testid="reading-filter-notice"
        >
          <span className="min-w-0 flex-1 truncate">
            {translate(
              'auto.components.reviewMap.readingOrder.filtering',
              'Filtering {{label}}: {{shown}} of {{total}}',
              {
                label: props.filterLabel ?? '',
                shown: visibleSteps,
                total: totals.total
              }
            )}
          </span>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            className="size-5"
            onClick={props.onClearFilter}
            aria-label={translate(
              'auto.components.reviewMap.readingOrder.clearFilter',
              'Clear filter'
            )}
          >
            <X aria-hidden />
          </Button>
        </div>
      ) : null}
      <div
        ref={scrollRef}
        role="listbox"
        tabIndex={0}
        aria-label={translate('auto.components.reviewMap.readingOrder.aria', 'Reading order')}
        aria-activedescendant={activeRow ? domId(activeRow) : undefined}
        onKeyDown={onKeyDown}
        className="min-h-0 flex-1 overflow-auto outline-none focus-visible:ring-1 focus-visible:ring-ring"
      >
        {virtualized ? (
          <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
            {virtualizer.getVirtualItems().map((v) => (
              <div
                key={v.key}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  height: v.size,
                  transform: `translateY(${v.start}px)`
                }}
              >
                {renderRow(rows[v.index], v.index)}
              </div>
            ))}
          </div>
        ) : (
          rows.map(renderRow)
        )}
        {props.overflow ? (
          <p className="px-3 py-2 text-xs text-muted-foreground">
            {translate(
              'auto.components.reviewMap.readingOrder.overflow',
              '{{shown}} of {{total}} files listed',
              props.overflow
            )}
          </p>
        ) : null}
      </div>
      <p className="px-3 py-1 text-[11px] text-muted-foreground">
        {translate(
          'auto.components.reviewMap.readingOrder.keys',
          'j/k move · Space mark read · Enter open diff'
        )}
      </p>
    </div>
  )
}
