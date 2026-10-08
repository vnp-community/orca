/**
 * use-review-turn-markers.ts — FE-CV-TASK-060-08
 *
 * Reads the worktree-level turn markers for the switcher. Writes stay in useReviewTurnRecorder;
 * `applySaved` lets it push the freshly saved list here without a second read.
 *
 * @module components/review-map/turns/use-review-turn-markers
 */

import { useCallback, useEffect, useState } from 'react'
import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'
import type { ReviewDataApi } from '../review-shell-data'
import { loadTurnMarkers } from './review-turn-marker-row'

export function useReviewTurnMarkers(
  worktreeId: string,
  api: ReviewDataApi,
  enabled: boolean
): { markers: ReviewTurnMarker[]; applySaved: (worktreeId: string, next: ReviewTurnMarker[]) => void } {
  const [markers, setMarkers] = useState<ReviewTurnMarker[]>([])

  useEffect(() => {
    setMarkers([])
    if (!enabled) {
      return
    }
    let live = true
    void loadTurnMarkers(api, worktreeId)
      .then((res) => {
        if (live && res.ok) {
          setMarkers(res.markers)
        }
      })
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [api, worktreeId, enabled])

  const applySaved = useCallback(
    (savedWorktreeId: string, next: ReviewTurnMarker[]) => {
      if (savedWorktreeId === worktreeId) {
        setMarkers(next)
      }
    },
    [worktreeId]
  )
  return { markers, applySaved }
}
