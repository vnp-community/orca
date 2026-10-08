/**
 * review-layout-storage.ts — FE-CV-TASK-051-05
 *
 * Panel geometry only (cosmetic, sole copy). Every access is guarded: storage can be
 * blocked (private window) and the workspace must still work.
 */

export const REVIEW_LAYOUT_STORAGE_KEY = 'orca.review.layout.v1'

export type ReviewLayout = {
  /** Panel sizes in percent. */
  leftSize: number
  rightSize: number
  leftOpen: boolean
}

export const DEFAULT_REVIEW_LAYOUT: ReviewLayout = { leftSize: 22, rightSize: 26, leftOpen: true }

const clamp = (n: number, min: number, max: number): number => Math.min(max, Math.max(min, n))

export function readReviewLayout(): ReviewLayout {
  try {
    const raw = window.localStorage.getItem(REVIEW_LAYOUT_STORAGE_KEY)
    if (!raw) {
      return DEFAULT_REVIEW_LAYOUT
    }
    const p = JSON.parse(raw) as Partial<ReviewLayout>
    return {
      leftSize:
        typeof p.leftSize === 'number' ? clamp(p.leftSize, 16, 35) : DEFAULT_REVIEW_LAYOUT.leftSize,
      rightSize:
        typeof p.rightSize === 'number'
          ? clamp(p.rightSize, 20, 40)
          : DEFAULT_REVIEW_LAYOUT.rightSize,
      leftOpen: typeof p.leftOpen === 'boolean' ? p.leftOpen : DEFAULT_REVIEW_LAYOUT.leftOpen
    }
  } catch {
    return DEFAULT_REVIEW_LAYOUT
  }
}

export function writeReviewLayout(layout: ReviewLayout): void {
  try {
    window.localStorage.setItem(REVIEW_LAYOUT_STORAGE_KEY, JSON.stringify(layout))
  } catch {
    // Blocked storage: geometry simply is not remembered.
  }
}

export type ReviewLayoutMode = 'three-column' | 'two-column' | 'one-column'

/** Thresholds are estimates (SOL-051 §9); one place to tune. */
export function layoutModeForWidth(width: number): ReviewLayoutMode {
  if (width >= 1100) {
    return 'three-column'
  }
  if (width >= 720) {
    return 'two-column'
  }
  return 'one-column'
}
