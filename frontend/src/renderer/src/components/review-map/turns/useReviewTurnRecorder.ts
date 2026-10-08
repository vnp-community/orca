/**
 * useReviewTurnRecorder.ts — FE-CV-TASK-060-07
 *
 * Records one ReviewTurnMarker per finished agent turn (worktree-level ReviewState row, newest
 * five). Mounted at workspace level so a turn that ends while the Review tab is closed is still
 * recorded. Support off: no store subscription, no RPC. A failed save is shown inline by the
 * switcher and never blocks sending notes. The SOL-061 completion feed does not exist yet, so
 * this reuses the diff-based detector from the agent-turn recorder.
 *
 * @module components/review-map/turns/useReviewTurnRecorder
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { defaultReviewDataApi } from '../review-data-api-default'
import type { ReviewDataApi } from '../review-shell-data'
import { detectAgentTurnCompletions } from './agent-turn-completion-detector'
import { buildReviewTurnMarker, reviewTurnId } from './review-turn-marker-builder'
import { saveTurnMarkerRow } from './review-turn-marker-row'
import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'

// Let git status settle after the agent's last write before fingerprinting the tree.
export const TURN_MARKER_SETTLE_MS = 3000
const SEEN_LIMIT = 100

export type UseReviewTurnRecorderResult = {
  /** Worktree whose last marker save failed, if any. */
  failedWorktreeId: string | null
  retry: () => void
}

export function useReviewTurnRecorder(
  opts: {
    api?: ReviewDataApi
    /** Symbol keys of the loaded change overlay for a worktree, or null when not loaded. */
    getSymbolKeys?: (worktreeId: string) => readonly string[] | null
    /** Receives the stored list after each successful save (drives the switcher). */
    onSaved?: (worktreeId: string, markers: ReviewTurnMarker[]) => void
  } = {}
): UseReviewTurnRecorderResult {
  const supported = useAppStore((s) => s.codeIntelSupportState.state === 'enabled')
  const api = opts.api ?? defaultReviewDataApi
  const getSymbolKeysRef = useRef(opts.getSymbolKeys)
  getSymbolKeysRef.current = opts.getSymbolKeys
  const onSavedRef = useRef(opts.onSaved)
  onSavedRef.current = opts.onSaved
  const [failed, setFailed] = useState<{ worktreeId: string; marker: ReviewTurnMarker } | null>(null)
  const failedRef = useRef(failed)
  failedRef.current = failed

  const save = useCallback(
    async (marker: ReviewTurnMarker): Promise<void> => {
      const res = await saveTurnMarkerRow(api, marker.worktreeId, [marker])
      setFailed(res.ok ? null : { worktreeId: marker.worktreeId, marker })
      if (res.ok) {
        onSavedRef.current?.(marker.worktreeId, res.markers)
      }
    },
    [api]
  )

  useEffect(() => {
    if (!supported) {
      return
    }
    const seen: string[] = []
    const timers = new Set<ReturnType<typeof setTimeout>>()
    const unsubscribe = useAppStore.subscribe((state, prev) => {
      for (const { paneKey, entry } of detectAgentTurnCompletions(
        prev.agentStatusByPaneKey,
        state.agentStatusByPaneKey
      )) {
        const turnId = reviewTurnId(paneKey, entry.stateStartedAt)
        if (seen.includes(turnId)) {
          continue
        }
        seen.push(turnId)
        if (seen.length > SEEN_LIMIT) {
          seen.shift()
        }
        const worktreeId = entry.worktreeId ?? ''
        const timer = setTimeout(() => {
          timers.delete(timer)
          const current = useAppStore.getState()
          const summary = current.gitBranchCompareSummaryByWorktree[worktreeId]
          const marker = buildReviewTurnMarker({
            worktreeId,
            paneKey,
            entry,
            headOid: summary?.headOid ?? null,
            baseOid: summary?.baseOid ?? null,
            mergeBase: summary?.mergeBase ?? null,
            statusEntries: current.gitStatusByWorktree[worktreeId] ?? [],
            symbolKeys: getSymbolKeysRef.current?.(worktreeId) ?? null
          })
          void save(marker)
        }, TURN_MARKER_SETTLE_MS)
        timers.add(timer)
      }
    })
    return () => {
      unsubscribe()
      for (const timer of timers) {
        clearTimeout(timer)
      }
    }
  }, [supported, save])

  const retry = useCallback(() => {
    if (failedRef.current) {
      void save(failedRef.current.marker)
    }
  }, [save])

  return { failedWorktreeId: failed?.worktreeId ?? null, retry }
}
