/**
 * open-quality-lens.ts — FE-CV-TASK-087-05
 *
 * Deep link into the `quality` lens of a worktree's Review tab. Entry points outside Review
 * (Source Control notice, Create PR) call this instead of reaching into the lens registry.
 *
 * @module components/review-map/quality/open-quality-lens
 */

import { openReviewFromEntryPoint } from '../entry/open-review-entry'
import type { ReviewEntrySource } from '../entry/open-review-entry'
import { QUALITY_LENS_ID } from './quality-lens-registration'

export function openQualityLens(
  worktreeId: string,
  source: ReviewEntrySource = 'source-control'
): boolean {
  return openReviewFromEntryPoint(worktreeId, source, { lens: QUALITY_LENS_ID })
}
