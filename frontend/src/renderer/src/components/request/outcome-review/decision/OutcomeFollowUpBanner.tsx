/**
 * OutcomeFollowUpBanner — FE-OR-T312
 *
 * A follow-up Request that this review sent findings to has finished. The review does not check it by itself (that would spend an
 * agent run nobody asked for); it says so and offers to regenerate.
 *
 * @module components/request/outcome-review/decision/OutcomeFollowUpBanner
 */

import React from 'react'
import { Info } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { followUpFinishedText } from './outcome-decision-copy'

export function OutcomeFollowUpBanner({
  busy,
  onRegenerate
}: {
  busy: boolean
  onRegenerate: () => void
}): React.JSX.Element {
  return (
    <div
      role="status"
      className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm"
      data-testid="outcome-follow-up-finished"
    >
      <Info className="size-4 shrink-0 text-muted-foreground" aria-hidden />
      <span className="min-w-0 flex-1">{followUpFinishedText()}</span>
      <Button size="xs" variant="outline" disabled={busy} onClick={onRegenerate}>
        {translate(
          'auto.components.request.outcomeReview.OutcomeReviewHeader.regenerate',
          'Regenerate…'
        )}
      </Button>
    </div>
  )
}
