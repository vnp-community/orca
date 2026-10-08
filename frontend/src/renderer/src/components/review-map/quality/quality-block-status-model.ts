/**
 * quality-block-status-model.ts — FE-CV-TASK-087-15
 *
 * Maps a data hook's raw facts to the ChartFrame status of a lens block. Previous data is kept
 * on a failed refresh and shown as `stale`; it never silently looks current.
 *
 * @module components/review-map/quality/quality-block-status-model
 */

import type { ChartStatus } from '../../quality-charts/chart-frame-types'

/** `idle`: quality is not enabled (render nothing); `unavailable`: no addressable worktree here. */
export type QualityBlockStatus = ChartStatus | 'idle' | 'unavailable'

export function resolveQualityBlockStatus(input: {
  enabled: boolean
  hasData: boolean
  isEmpty: boolean
  loading: boolean
  failed: boolean
  stale: boolean
}): QualityBlockStatus {
  if (!input.enabled) {
    return 'idle'
  }
  if (!input.hasData) {
    return input.failed && !input.loading ? 'error' : 'loading'
  }
  if (input.isEmpty) {
    return 'empty'
  }
  return input.stale || input.failed ? 'stale' : 'ready'
}
