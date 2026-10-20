/**
 * PlanRunNotice — CR-REQ-044
 *
 * What the server's Plan run says above a Plan that already exists: a rewrite in progress, or the failure of the last one.
 * Both are about a change request, so the failure is worded and retried as one (CR-REQ-045).
 *
 * @module components/request/plan/PlanRunNotice
 */

import React from 'react'
import { PlanRewriteStrip } from './PlanRewriteStrip'
import { PlanRunFailedCard } from './PlanRunFailedCard'
import type { PlanAutoDisplay } from './plan-auto-display'

type Props = {
  display: PlanAutoDisplay
  onRetry: () => void
  retrying: boolean
  retryError: string | null
  /** The revision panel already says "Writing version n"; a second strip would only repeat it (CR-REQ-045). */
  hideStrip?: boolean
  /** A new round is under way, so the failure of the last one is no longer news. */
  hideFailure?: boolean
}

export function PlanRunNotice({
  display,
  onRetry,
  retrying,
  retryError,
  hideStrip = false,
  hideFailure = false
}: Props): React.JSX.Element | null {
  if (display.kind === 'writing' && display.replan) {
    return hideStrip ? null : <PlanRewriteStrip />
  }
  if (display.kind === 'failed' && !hideFailure) {
    return (
      <div className="border-b border-border px-4 py-3">
        <PlanRunFailedCard
          code={display.code}
          variant="revise"
          onRetry={onRetry}
          retrying={retrying}
          retryError={retryError}
        />
      </div>
    )
  }
  return null
}
