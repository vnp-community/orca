import { isMobileGitUnavailable } from '../source-control/mobile-git-status'
import type { RpcClient } from '../transport/rpc-client'
import {
  readMobileReviewSummaryResult,
  type MobileReviewSummary,
  type MobileReviewSummaryReason
} from './mobile-review-summary-rpc'

export type MobileReviewSummaryState =
  | { kind: 'loading' }
  | { kind: 'ready'; summary: MobileReviewSummary }
  | { kind: 'unavailable'; message: string }
  | { kind: 'error'; message: string }

export const MOBILE_REVIEW_SUMMARY_HOST_UNSUPPORTED =
  'This desktop does not support review summaries yet. Update Orca desktop to use it.'

const UNAVAILABLE_MESSAGES: Record<MobileReviewSummaryReason | 'generic', string> = {
  flag_off:
    'Code intelligence is turned off for this workspace. Turn it on in Orca desktop settings.',
  no_binding: 'This repository is not linked to a code index. Link it from Orca desktop.',
  index_missing: 'No code index exists for this branch yet. Start indexing from Orca desktop.',
  tool_unavailable: 'The code analysis tools are unavailable on the desktop right now.',
  generic: 'Review summary is not available right now.'
}

export function mobileReviewSummaryUnavailableMessage(
  reason: MobileReviewSummaryReason | undefined
): string {
  return UNAVAILABLE_MESSAGES[reason ?? 'generic']
}

export async function loadMobileReviewSummary(
  client: RpcClient,
  worktreeId: string
): Promise<MobileReviewSummaryState> {
  let response
  try {
    response = await client.sendRequest('codeIntel.reviewSummary', {
      worktree: `id:${worktreeId}`
    })
  } catch (err) {
    return {
      kind: 'error',
      message: err instanceof Error ? err.message : 'Unable to load review summary'
    }
  }
  if (!response.ok) {
    if (isMobileGitUnavailable(response.error?.code, response.error?.message)) {
      return {
        kind: 'unavailable',
        message: MOBILE_REVIEW_SUMMARY_HOST_UNSUPPORTED
      }
    }
    return {
      kind: 'error',
      message: response.error?.message || 'Unable to load review summary'
    }
  }
  const summary = readMobileReviewSummaryResult(response.result)
  if (!summary) {
    return { kind: 'error', message: 'Review summary response was invalid' }
  }
  if (!summary.available) {
    return {
      kind: 'unavailable',
      message: mobileReviewSummaryUnavailableMessage(summary.reason)
    }
  }
  return { kind: 'ready', summary }
}
