import { useCallback, useEffect, useMemo } from 'react'
import { useAppStore } from '@/store'
import { resolveCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import { DEFAULT_REVIEW_UI_STATE } from '@/store/slices/review-ui'
import { usePerceivedLoadingStage } from '@/hooks/usePerceivedLoadingStage'
import {
  reviewChipPredicate,
  REVIEW_CHIP_TARGET_LENS,
  type ReviewChipId
} from '../review-chip-filter'
import { buildReadingOrderItems } from '../reading-order-model'
import {
  resolveDefaultReviewScope,
  reviewStateKeyFromOverlayScope,
  type DefaultScopeResult,
  type ReviewScope
} from '../review-scope-model'
import type { ReviewDataApi } from '../review-shell-data'
import { computeReviewViewState } from '../review-view-state'
import { getReviewLenses, resolveActiveLensId } from '../review-lens-registry'
import { useReviewShellData } from './useReviewShellData'
import { useUnavailableReviewLenses } from './review-lens-availability'

/** Everything the workspace renders, derived once so the component stays layout-only. */
export function useReviewWorkspaceModel(
  worktreeId: string,
  api: ReviewDataApi,
  flags: { quality: boolean }
) {
  const ui = useAppStore((s) => s.reviewUiByWorktree[worktreeId]) ?? DEFAULT_REVIEW_UI_STATE
  const summary = useAppStore((s) => s.gitBranchCompareSummaryByWorktree[worktreeId] ?? null)
  const worktree = useAppStore((s) =>
    Object.values(s.worktreesByRepo)
      .flat()
      .find((w) => w.id === worktreeId)
  )
  const support = useAppStore((s) => s.codeIntelSupportState?.state ?? 'unknown')
  // Primitive snapshot: the shared hook returns a fresh object per call when ready, which
  // zustand v5 treats as an ever-changing snapshot.
  const selectorSnapshot = useAppStore((s) => {
    const r = resolveCodeIntelSelector(s, worktreeId)
    return r.state === 'ready' ? JSON.stringify({ environmentId: r.environmentId }) : null
  })
  const selector = useMemo(
    () =>
      selectorSnapshot === null
        ? ({ state: 'unsupported' } as const)
        : ({
            state: 'ready',
            environmentId: (JSON.parse(selectorSnapshot) as { environmentId: string | null })
              .environmentId
          } as const),
    [selectorSnapshot]
  )
  const store = useAppStore.getState

  const defaultScope: DefaultScopeResult = useMemo(() => {
    const fromSummary = resolveDefaultReviewScope(summary)
    // Summary not loaded yet but the worktree pins a base: usable right away.
    if (!summary && worktree?.baseRef) {
      return {
        status: 'ready',
        scope: { kind: 'branch', baseRef: worktree.baseRef, includeUncommitted: true }
      }
    }
    return fromSummary
  }, [summary, worktree?.baseRef])

  const scope: ReviewScope | null =
    ui.scope ?? (defaultScope.status === 'ready' ? defaultScope.scope : null)
  const scopeResult: DefaultScopeResult = ui.scope
    ? { status: 'ready', scope: ui.scope }
    : defaultScope

  const selectorReady = selector.state === 'ready'
  const enabled =
    support !== 'disabled' && support !== 'unsupported' && selectorReady && scope !== null
  const data = useReviewShellData({ worktreeId, scope, api, enabled })

  const viewState = computeReviewViewState({
    support,
    selectorUnsupported: !selectorReady,
    scope: scopeResult,
    status: { data: data.status, error: data.statusError },
    overlay: { data: data.overlay, error: data.overlayError, pending: data.overlayPending },
    hasNewDataSignal: data.hasNewDataSignal
  })

  const unavailableLenses = useUnavailableReviewLenses(worktreeId)
  const lenses = useMemo(
    () => getReviewLenses(flags).filter((l) => !unavailableLenses.has(l.id)),
    [flags, unavailableLenses]
  )
  const activeLensId = resolveActiveLensId(ui.lens, lenses)

  // Reading progress is keyed by the backend-resolved (base, head).
  const stateKey = reviewStateKeyFromOverlayScope(data.overlay?.scope)
  const baseCommit = stateKey?.baseCommit
  const headCommit = stateKey?.headCommit
  useEffect(() => {
    if (!baseCommit || !headCommit) {
      return
    }
    void store().loadReviewProgress(worktreeId, baseCommit, headCommit)
  }, [worktreeId, baseCommit, headCommit, store])

  useEffect(() => {
    const flush = (): void => void store().flushReviewProgress(worktreeId)
    const onVisibility = (): void => {
      if (document.visibilityState === 'hidden') {
        flush()
      }
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      flush()
    }
  }, [worktreeId, store])

  // A connection coming back retries an unsaved/failed state instead of waiting for the timer.
  const anyEstablished = useAppStore((s) =>
    Object.values(s.connections ?? {}).some((c) => c.status === 'established')
  )
  const { refetchOverlay, overlayError } = data
  useEffect(() => {
    if (!anyEstablished) {
      return
    }
    void store().retryReviewProgressSave(worktreeId)
    if (overlayError?.kind === 'offline') {
      refetchOverlay()
    }
  }, [anyEstablished, worktreeId, store, refetchOverlay, overlayError?.kind])

  const progressEntry = useAppStore((s) => s.reviewProgressByWorktree[worktreeId])
  const items = useMemo(
    () =>
      data.overlay
        ? buildReadingOrderItems(
            data.overlay.readingOrder,
            data.overlay.components,
            data.overlay.changedFiles
          )
        : [],
    [data.overlay]
  )
  const filter = useMemo(() => {
    if (!ui.chipFilter || !data.overlay) {
      return null
    }
    return reviewChipPredicate(data.overlay, ui.chipFilter)?.files ?? null
  }, [ui.chipFilter, data.overlay])

  const loadingStage = usePerceivedLoadingStage(
    data.overlayPending ||
      (viewState.kind === 'screen' && viewState.screen.id === 'loading-initial'),
    {
      remote: selector.state === 'ready' && selector.environmentId !== null
    }
  )

  const onChipClick = useCallback(
    (chip: ReviewChipId) => {
      const target = REVIEW_CHIP_TARGET_LENS[chip]
      // Navigation chips never set a filter.
      if (target) {
        if (lenses.some((l) => l.id === target)) {
          store().setReviewLens(worktreeId, target)
        }
        return
      }
      store().toggleReviewChipFilter(worktreeId, chip)
    },
    [lenses, store, worktreeId]
  )

  return {
    ui,
    scope,
    defaultScope,
    summary,
    worktree,
    selector,
    data,
    viewState,
    lenses,
    activeLensId,
    progressEntry,
    items,
    filter,
    loadingStage,
    onChipClick
  }
}
