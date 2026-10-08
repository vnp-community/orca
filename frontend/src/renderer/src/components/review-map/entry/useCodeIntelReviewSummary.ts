/**
 * useCodeIntelReviewSummary.ts — FE-CV-TASK-061-06
 *
 * Summary data for the right-sidebar Review tab. Fetches only while the tab is visible and the
 * flag is on; refreshes on resync/new agent completion, never by polling.
 */

import { useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { resolveDefaultReviewScope, toChangeOverlayParams } from '../review-scope-model'
import { normalizeChangeOverlay, classifyReviewError } from '../review-shell-data'
import { defaultReviewDataApi } from '../review-data-api-default'
import type { ChangeOverlayView, ReadingProgress, ReviewError } from '../review-wire-types'
import { buildReviewSummaryModel, type ReviewSummaryModel } from './review-summary-model'

export type CodeIntelReviewSummaryState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'blocked'; message?: string }
  | { status: 'error'; error: ReviewError }
  | { status: 'ready'; model: ReviewSummaryModel }

export type ReviewSummaryFetchers = {
  fetchOverlaySummary: (
    worktreeId: string,
    params: ReturnType<typeof toChangeOverlayParams>,
    signal: AbortSignal
  ) => Promise<{ ok: true; value: ChangeOverlayView } | { ok: false; error: ReviewError }>
  fetchProgress: (worktreeId: string, overlay: ChangeOverlayView) => Promise<ReadingProgress | null>
}

async function fetchOverlaySummaryDefault(
  worktreeId: string,
  params: ReturnType<typeof toChangeOverlayParams>,
  signal: AbortSignal
): ReturnType<ReviewSummaryFetchers['fetchOverlaySummary']> {
  const [{ getCodeIntelClient }, { resolveCodeIntelSelector }] = await Promise.all([
    import('../../../runtime/code-intel-client'),
    import('../../../lib/code-intel-worktree-selector')
  ])
  const sel = resolveCodeIntelSelector(useAppStore.getState(), worktreeId)
  if (sel.state !== 'ready') {
    return { ok: false, error: { kind: 'no-binding', message: sel.reason } }
  }
  try {
    // Why: detail 'summary' is a single cheap call; the full overlay belongs to the Review tab.
    const res = await getCodeIntelClient().call(
      sel.worktreeId,
      'codeIntel.changeOverlay',
      { projectId: sel.projectId, worktreeId: sel.worktreeId, ...params, detail: 'summary' },
      { environmentId: sel.environmentId, signal }
    )
    return res.ok
      ? { ok: true, value: normalizeChangeOverlay(res.result) }
      : { ok: false, error: classifyReviewError(res.error) }
  } catch (err) {
    return { ok: false, error: classifyReviewError(err) }
  }
}

async function fetchProgressDefault(
  worktreeId: string,
  overlay: ChangeOverlayView
): Promise<ReadingProgress | null> {
  const baseCommit = overlay.scope?.mergeBase ?? overlay.scope?.baseOid
  const headCommit = overlay.scope?.headOid
  if (!baseCommit || !headCommit) {
    return null
  }
  const res = await defaultReviewDataApi.getReviewState(worktreeId, { baseCommit, headCommit })
  return res.ok ? res.value.readingProgress : null
}

export const DEFAULT_REVIEW_SUMMARY_FETCHERS: ReviewSummaryFetchers = {
  fetchOverlaySummary: fetchOverlaySummaryDefault,
  fetchProgress: fetchProgressDefault
}

export function useCodeIntelReviewSummary({
  worktreeId,
  enabled,
  latestCompletionId,
  fetchers = DEFAULT_REVIEW_SUMMARY_FETCHERS
}: {
  worktreeId: string | null
  /** rightSidebarOpen && effective tab is 'review' && flag on. */
  enabled: boolean
  latestCompletionId: string | null
  fetchers?: ReviewSummaryFetchers
}): { state: CodeIntelReviewSummaryState; refresh: () => void } {
  const branchSummary = useAppStore((s) =>
    worktreeId ? (s.gitBranchCompareSummaryByWorktree?.[worktreeId] ?? null) : null
  )
  const resync = useAppStore((s) => s.codeIntelResyncCounter)
  const [state, setState] = useState<CodeIntelReviewSummaryState>({ status: 'idle' })
  const [manualTick, setManualTick] = useState(0)
  const seq = useRef(0)

  const scopeResult = resolveDefaultReviewScope(branchSummary)
  const scopeBase = scopeResult.status === 'ready' ? JSON.stringify(scopeResult.scope) : null
  const blockedMessage = scopeResult.status === 'blocked' ? (scopeResult.message ?? '') : null

  useEffect(() => {
    if (!enabled || !worktreeId) {
      return
    }
    if (blockedMessage !== null) {
      setState({ status: 'blocked', message: blockedMessage || undefined })
      return
    }
    if (scopeBase === null) {
      setState({ status: 'loading' })
      return
    }
    const mySeq = ++seq.current
    const ctrl = new AbortController()
    const scope = JSON.parse(scopeBase) as Parameters<typeof toChangeOverlayParams>[0]
    setState((prev) => (prev.status === 'ready' ? prev : { status: 'loading' }))
    void (async () => {
      const res = await fetchers.fetchOverlaySummary(
        worktreeId,
        toChangeOverlayParams(scope),
        ctrl.signal
      )
      if (mySeq !== seq.current) {
        return
      }
      if (!res.ok) {
        setState({ status: 'error', error: res.error })
        return
      }
      const progress = await fetchers.fetchProgress(worktreeId, res.value).catch(() => null)
      if (mySeq === seq.current) {
        setState({ status: 'ready', model: buildReviewSummaryModel(res.value, progress) })
      }
    })()
    return () => {
      ctrl.abort()
    }
  }, [
    enabled,
    worktreeId,
    scopeBase,
    blockedMessage,
    resync,
    latestCompletionId,
    manualTick,
    fetchers
  ])

  return { state, refresh: () => setManualTick((n) => n + 1) }
}
