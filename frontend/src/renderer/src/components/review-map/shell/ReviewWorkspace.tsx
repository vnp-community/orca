import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useAppStore } from '@/store'
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { isEditableTarget } from '@/lib/editable-target'
import { translate } from '@/i18n/i18n'
import type { ReviewDataApi } from '../review-shell-data'
import { defaultReviewDataApi } from '../review-data-api-default'
import { getReviewDrawerRenderer, type ReviewLensProps } from '../review-lens-registry'
import {
  layoutModeForWidth,
  readReviewLayout,
  writeReviewLayout,
  type ReviewLayout
} from '../review-layout-storage'
import { useDiffCursorSymbolSync } from '../impact/use-diff-cursor-symbol-sync'
import '../impact/symbol-detail-drawer-registration'
import { ReadingOrderList } from '../reading-order/ReadingOrderList'
import {
  AmbiguousSymbolDialog,
  type AmbiguousSymbolCandidate,
  type AmbiguousSymbolChoice
} from './AmbiguousSymbolDialog'
import { QualityGateChip } from '../quality/QualityGateChip'
import { ReviewBottomDock } from './ReviewBottomDock'
import { ReviewCompanionStrip } from './ReviewCompanionStrip'
import { ReviewDetailDrawer } from './ReviewDetailDrawer'
import { ReviewHeaderBar } from './ReviewHeaderBar'
import { ReviewLensTabs } from './ReviewLensTabs'
import { ReviewStateBanners } from './ReviewStateBanners'
import { ReviewSummaryBar } from './ReviewSummaryBar'
import { ReviewViewStateScreen } from './ReviewViewStateScreen'
import { useReviewCompanions } from './use-review-companions'
import { useReviewWorkspaceModel } from './useReviewWorkspaceModel'

export type ReviewWorkspaceProps = {
  worktreeId: string
  api?: ReviewDataApi
  onOpenDiff?: (path: string, line?: number) => void
  onCloseTab?: () => void
  flags?: { quality: boolean }
}

const NO_QUALITY = { quality: false }
const noop = (): void => undefined

export default function ReviewWorkspace({
  worktreeId,
  api = defaultReviewDataApi,
  onOpenDiff = noop,
  onCloseTab = noop,
  flags = NO_QUALITY
}: ReviewWorkspaceProps): React.JSX.Element {
  const m = useReviewWorkspaceModel(worktreeId, api, flags)
  const { ui, scope, data, viewState, lenses, activeLensId } = m
  const store = useAppStore.getState
  const changedSymbolRefs = useMemo(
    () => (data.overlay?.changedSymbols ?? []).map((c) => c.symbol),
    [data.overlay]
  )
  useDiffCursorSymbolSync(worktreeId, changedSymbolRefs)

  const c = useReviewCompanions(worktreeId, api, m)

  const rootRef = useRef<HTMLDivElement>(null)
  // Element focused when the drawer was opened; Esc/close returns focus there.
  const returnFocusRef = useRef<HTMLElement | null>(null)
  const [width, setWidth] = useState(1200)
  const [layout, setLayout] = useState<ReviewLayout>(() => readReviewLayout())
  const [narrowPane, setNarrowPane] = useState<'reading' | 'lens'>('lens')
  const [ambiguous, setAmbiguous] = useState<{
    candidates: readonly AmbiguousSymbolCandidate[]
    resolve: (c: AmbiguousSymbolChoice | null) => void
  } | null>(null)

  // Width of the tab itself (not the window): the tab can be a narrow split.
  useEffect(() => {
    const el = rootRef.current
    if (!el || typeof ResizeObserver === 'undefined') {
      return
    }
    const ro = new ResizeObserver((entries) => setWidth(entries[0]?.contentRect.width ?? 1200))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const mode = layoutModeForWidth(width)
  const updateLayout = useCallback((patch: Partial<ReviewLayout>) => {
    setLayout((prev) => {
      const next = { ...prev, ...patch }
      writeReviewLayout(next)
      return next
    })
  }, [])

  const requestSymbolChoice = useCallback(
    (candidates: readonly AmbiguousSymbolCandidate[]) =>
      new Promise<AmbiguousSymbolChoice | null>((resolve) => setAmbiguous({ candidates, resolve })),
    []
  )
  const settleAmbiguous = (choice: AmbiguousSymbolChoice | null): void => {
    ambiguous?.resolve(choice)
    setAmbiguous(null)
  }

  const closeDrawer = (): void => store().setReviewDrawerOpen(worktreeId, false)
  const onKeyDown = (e: KeyboardEvent): void => {
    if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || isEditableTarget(e.target)) {
      return
    }
    if (e.key === '[') {
      e.preventDefault()
      updateLayout({ leftOpen: !layout.leftOpen })
    } else if (e.key === ']') {
      e.preventDefault()
      if (ui.drawerOpen) {
        closeDrawer()
      } else if (ui.selectedSymbolKey) {
        store().setReviewDrawerOpen(worktreeId, true)
      }
    } else if (e.key === 'Escape' && ui.drawerOpen) {
      e.preventDefault()
      closeDrawer()
    }
  }

  const progress = m.progressEntry
  const actions = {
    onReindex: () => void api.reindex(worktreeId, 'incremental').then(() => data.refreshStatus()),
    onRebind: () => void api.bindRepo(worktreeId).then(() => data.refreshStatus({ refresh: true })),
    onRetry: () => {
      data.refreshStatus()
      data.refetchOverlay()
    },
    onCloseTab,
    onOpenScopePicker: () => store().setReviewScopePickerOpen(worktreeId, true)
  }

  const reindexProps = {
    worktreeId,
    api,
    onStarted: () => data.refreshStatus(),
    remote: m.selector.state === 'ready' && m.selector.environmentId !== null
  }
  const header = (
    <ReviewHeaderBar
      branchName={m.worktree?.branch ?? null}
      overlay={data.overlay}
      status={data.status}
      reindex={reindexProps}
      trailing={<QualityGateChip worktreeId={worktreeId} currentHead={m.summary?.headOid ?? null} />}
      scopePicker={{
        scope,
        open: ui.scopePickerOpen,
        onOpenChange: (open) => store().setReviewScopePickerOpen(worktreeId, open),
        onChange: (s) => store().setReviewScope(worktreeId, s),
        defaultBaseRef: m.summary?.baseRef ?? m.worktree?.baseRef ?? null,
        hostedReview: null
      }}
    />
  )

  if (viewState.kind === 'screen') {
    return (
      <div
        ref={rootRef}
        className="flex h-full min-h-0 flex-col"
        data-testid="review-workspace"
        onKeyDown={onKeyDown}
      >
        {header}
        <ReviewViewStateScreen screen={viewState.screen} actions={actions} />
      </div>
    )
  }

  const overlay = data.overlay!
  const lensProps: ReviewLensProps | null = scope
    ? {
        worktreeId,
        environmentId: m.selector.state === 'ready' ? m.selector.environmentId : null,
        scope,
        overlay,
        selectedSymbolKey: ui.selectedSymbolKey,
        chipFilter: ui.chipFilter,
        onSelectSymbol: (key) => {
          if (key && document.activeElement instanceof HTMLElement) {
            returnFocusRef.current = document.activeElement
          }
          store().selectReviewSymbol(worktreeId, key)
        },
        onOpenDiff,
        requestSymbolChoice
      }
    : null

  const readingList = (
    <ReadingOrderList
      items={m.items}
      overlay={overlay}
      changedFileCount={overlay.changedFiles.length}
      progress={progress?.progress ?? { version: 1, entries: {}, lastFocusedKey: null }}
      saveStatus={progress?.saveStatus ?? 'saved'}
      localOnly={progress?.localOnly}
      oversize={progress?.oversize}
      progressLoadFailed={progress?.loadStatus === 'error'}
      loadingStage={progress && progress.loadStatus === 'loading' ? m.loadingStage : 'idle'}
      filterFiles={c.readingFilter}
      filterLabel={
        ui.chipFilter ??
        (c.turnFilterActive
          ? translate('auto.components.reviewMap.shell.turnFilter', 'Since previous turn')
          : undefined)
      }
      initialFocusKey={
        progress?.loadStatus === 'ready' ? progress.progress.lastFocusedKey : undefined
      }
      onClearFilter={() => {
        store().clearReviewChipFilter(worktreeId)
        c.setTurnMode('all')
      }}
      onSetSeen={(k, seen) => store().setReadingItemSeen(worktreeId, k, seen)}
      onSetGroupSeen={(ks, seen) => store().setReadingGroupSeen(worktreeId, ks, seen)}
      onFocusChange={(k) => store().setReadingLastFocused(worktreeId, k)}
      onOpenDiff={onOpenDiff}
      onRetrySave={() => void store().retryReviewProgressSave(worktreeId)}
      onRetryProgressLoad={() => data.applyNewData()}
      overflow={
        overlay.limits.truncated.steps
          ? {
              shown: m.items.length,
              total: overlay.limits.totalCounts.readingOrder ?? m.items.length
            }
          : null
      }
    />
  )

  const center = (
    <div className="flex h-full min-h-0 flex-col">
      {lensProps ? (
        <ReviewLensTabs
          lenses={lenses}
          activeLensId={activeLensId}
          onSelectLens={(id) => store().setReviewLens(worktreeId, id)}
          lensProps={lensProps}
        />
      ) : null}
    </div>
  )

  const drawerOpen = ui.drawerOpen && ui.selectedSymbolKey !== null && scope !== null
  const renderDrawer = getReviewDrawerRenderer()
  const drawer = drawerOpen ? (
    <ReviewDetailDrawer
      mode={mode === 'three-column' ? 'panel' : 'sheet'}
      onClose={closeDrawer}
      returnFocusTo={returnFocusRef.current}
    >
      {renderDrawer ? (
        renderDrawer({
          worktreeId,
          environmentId: m.selector.state === 'ready' ? m.selector.environmentId : null,
          scope: scope!,
          selectedSymbolKey: ui.selectedSymbolKey!,
          indexStatus: data.status,
          onClose: closeDrawer,
          onOpenDiff
        })
      ) : (
        <p className="text-sm text-muted-foreground">
          {translate(
            'auto.components.reviewMap.shell.drawer.unavailable',
            'Details are not available yet.'
          )}
        </p>
      )}
    </ReviewDetailDrawer>
  ) : null

  let body: React.JSX.Element
  if (mode === 'one-column') {
    body = (
      <div className="flex min-h-0 flex-1 flex-col">
        <ToggleGroup
          type="single"
          value={narrowPane}
          onValueChange={(v) => v && setNarrowPane(v as 'reading' | 'lens')}
          variant="outline"
          size="sm"
          className="mx-3 mb-2 w-fit"
        >
          <ToggleGroupItem value="reading">
            {translate('auto.components.reviewMap.shell.pane.reading', 'Reading order')}
          </ToggleGroupItem>
          <ToggleGroupItem value="lens">
            {translate('auto.components.reviewMap.shell.pane.lens', 'Lens')}
          </ToggleGroupItem>
        </ToggleGroup>
        <div className="min-h-0 flex-1">{narrowPane === 'reading' ? readingList : center}</div>
      </div>
    )
  } else {
    const showRight = mode === 'three-column' && drawerOpen
    const leftSize = String(layout.leftSize)
    const rightSize = String(layout.rightSize)
    // Panels are mounted conditionally (not collapsed) so closed columns cost nothing.
    body = (
      <ResizablePanelGroup
        orientation="horizontal"
        className="min-h-0 flex-1"
        onLayoutChanged={(l: Record<string, number>) =>
          updateLayout({
            leftSize: l.left ?? layout.leftSize,
            rightSize: l.right ?? layout.rightSize
          })
        }
      >
        {layout.leftOpen ? (
          <>
            <ResizablePanel id="left" defaultSize={leftSize} minSize="16" maxSize="35">
              {readingList}
            </ResizablePanel>
            <ResizableHandle />
          </>
        ) : null}
        <ResizablePanel id="center" minSize="30">
          {center}
        </ResizablePanel>
        {showRight ? (
          <>
            <ResizableHandle />
            <ResizablePanel id="right" defaultSize={rightSize} minSize="20" maxSize="40">
              {drawer}
            </ResizablePanel>
          </>
        ) : null}
      </ResizablePanelGroup>
    )
  }

  return (
    <div
      ref={rootRef}
      className="flex h-full min-h-0 flex-col"
      data-testid="review-workspace"
      data-layout={mode}
      onKeyDown={onKeyDown}
    >
      {header}
      <div className="space-y-1.5 px-3 pb-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <ReviewSummaryBar overlay={overlay} active={ui.chipFilter} onChipClick={m.onChipClick} />
          <ReviewCompanionStrip worktreeId={worktreeId} m={m} c={c} onOpenDiff={onOpenDiff} />
        </div>
        <ReviewStateBanners
          banners={viewState.banners}
          onRetry={actions.onRetry}
          onApplyNewData={data.applyNewData}
          onRefreshIndex={() => data.refreshStatus({ refresh: true })}
        />
      </div>
      {body}
      {scope ? (
        <ReviewBottomDock
          flags={flags}
          panelProps={{
            worktreeId,
            environmentId: m.selector.state === 'ready' ? m.selector.environmentId : null,
            scope,
            overlay,
            availableLensIds: new Set(lenses.map((l) => l.id)),
            onOpenDiff
          }}
        />
      ) : null}
      {mode !== 'three-column' ? drawer : null}
      {ambiguous ? (
        <AmbiguousSymbolDialog
          candidates={ambiguous.candidates}
          onChoose={settleAmbiguous}
          onCancel={() => settleAmbiguous(null)}
        />
      ) : null}
    </div>
  )
}
