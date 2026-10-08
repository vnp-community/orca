/**
 * use-review-turn-recorder-channel.ts — FE-CV-TASK-060-07
 *
 * Workspace side of the App-level turn recorder: publishes the overlay symbol keys, applies
 * freshly saved markers and exposes the inline save-failure state.
 *
 * @module components/review-map/turns/use-review-turn-recorder-channel
 */

import { useEffect, useRef, useSyncExternalStore } from 'react'
import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'
import {
  getTurnSaveFailure,
  registerTurnSymbolKeys,
  subscribeTurnSaveFailure,
  subscribeTurnSaved
} from './review-turn-recorder-bus'

export function useReviewTurnRecorderChannel(
  worktreeId: string,
  symbolKeys: readonly string[] | null,
  applySaved: (worktreeId: string, markers: ReviewTurnMarker[]) => void
): { failed: boolean; retry: () => void } {
  const keysRef = useRef(symbolKeys)
  keysRef.current = symbolKeys
  const applyRef = useRef(applySaved)
  applyRef.current = applySaved

  useEffect(() => registerTurnSymbolKeys(worktreeId, () => keysRef.current), [worktreeId])
  useEffect(() => subscribeTurnSaved((id, markers) => applyRef.current(id, markers)), [])

  const failure = useSyncExternalStore(subscribeTurnSaveFailure, getTurnSaveFailure)
  return {
    failed: failure?.worktreeId === worktreeId,
    retry: () => failure?.retry()
  }
}
