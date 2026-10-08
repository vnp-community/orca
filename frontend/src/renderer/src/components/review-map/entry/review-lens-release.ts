import { getReviewLenses } from '../review-lens-registry'

/** A lens counts as released only when it has a loader (placeholders stay hidden from Cmd+K). */
export function isReviewLensReleased(lensId: string, flags: { quality: boolean }): boolean {
  return getReviewLenses(flags).some((lens) => lens.id === lensId && lens.load !== undefined)
}
