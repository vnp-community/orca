import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import {
  buildMobileReviewSummary,
  mapCodeIntelErrorToSummary,
  REVIEW_SUMMARY_MAX_ITEMS
} from './code-intel-review-summary-mapping'
import { getCodeIntelSummaryPort } from './code-intel-summary-port'

const ReviewSummaryParams = z.object({
  worktree: z.string().min(1).max(512),
  scope: z.literal('branch').optional()
})

// Why: mobile gets one compact read-only method instead of the gateway channels;
// the worktree selector is `id:<worktreeId>` like the other mobile git methods.
function worktreeIdFromSelector(selector: string): string {
  return selector.startsWith('id:') ? selector.slice(3) : selector
}

export const CODE_INTEL_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'codeIntel.reviewSummary',
    params: ReviewSummaryParams,
    handler: async (params) => {
      const worktreeId = worktreeIdFromSelector(params.worktree)
      const port = getCodeIntelSummaryPort()
      try {
        const [overlay, findings, status] = await Promise.all([
          port.getOverlay(worktreeId),
          port.getFindings(worktreeId, {
            limit: REVIEW_SUMMARY_MAX_ITEMS,
            scope: 'changed'
          }),
          port.getStatus(worktreeId)
        ])
        return buildMobileReviewSummary({ overlay, findings, status })
      } catch (error) {
        return mapCodeIntelErrorToSummary(error)
      }
    }
  })
]
