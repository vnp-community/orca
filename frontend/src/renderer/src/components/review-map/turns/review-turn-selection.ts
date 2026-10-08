/**
 * review-turn-selection.ts — FE-CV-TASK-060-08
 *
 * Which two markers a comparison uses: the chosen (default newest) turn and the marker before it.
 *
 * @module components/review-map/turns/review-turn-selection
 */

import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'

export type ReviewTurnViewMode = 'all' | 'since-previous' | 'previous'

export type TurnPair = { current: ReviewTurnMarker; previous: ReviewTurnMarker }

/** Markers are stored oldest first; null when there is no earlier marker to compare against. */
export function selectTurnPair(
  markers: readonly ReviewTurnMarker[],
  selectedTurnId: string | null
): TurnPair | null {
  const index = selectedTurnId ? markers.findIndex((m) => m.turnId === selectedTurnId) : markers.length - 1
  const at = index >= 0 ? index : markers.length - 1
  if (at < 1) {
    return null
  }
  return { current: markers[at], previous: markers[at - 1] }
}
