/**
 * OutcomeDecisionBar — FE-OR-T302, T303
 *
 * Approve, and Request changes. Approve is the one primary button and is never disabled: with decisions still open it says so
 * next to itself and takes the viewer to the first one (a button that cannot be pressed would not say why). The server refuses
 * an approval with open decisions too, whatever this screen does. Approving does not change the Request and is not a sign-off to
 * release: the line after it says so.
 *
 * @module components/request/outcome-review/decision/OutcomeDecisionBar
 */

import React, { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { errorText } from '../outcome-review-state-copy'
import {
  approvedText,
  barText,
  openDecisionsErrorText,
  requestChangesCountText,
  whoText
} from './outcome-decision-copy'
import type { DecisionResult } from './use-outcome-decisions'
import type { OutcomeReview } from '../../../../../../shared/outcome-review-types'

type Props = {
  review: OutcomeReview
  openKeys: string[]
  sentBackCount: number
  currentUserId: string | null
  busy: boolean
  onApprove: (note: string) => Promise<DecisionResult>
  onRequestChanges: () => void
  /** Takes the viewer to the first item still open. */
  onShowItem: (itemKey: string) => void
}

export function OutcomeDecisionBar({
  review,
  openKeys,
  sentBackCount,
  currentUserId,
  busy,
  onApprove,
  onRequestChanges,
  onShowItem
}: Props): React.JSX.Element {
  const [note, setNote] = useState('')
  const [error, setError] = useState<string | null>(null)
  const approved = review.state === 'approved'

  const approve = async (): Promise<void> => {
    if (openKeys.length > 0) {
      setError(openDecisionsErrorText(openKeys.length))
      onShowItem(openKeys[0])
      return
    }
    setError(null)
    const res = await onApprove(note)
    if (res.ok) {
      setNote('')
    } else {
      setError(errorText(res.error.code))
    }
  }

  return (
    <section
      className="flex flex-col gap-2 rounded-md border border-border p-3"
      aria-label={barText('approve')}
      data-testid="outcome-decision-bar"
    >
      {approved ? (
        <div className="flex flex-col gap-1" data-testid="outcome-approved">
          <p className="text-sm font-medium">
            {approvedText(whoText(!!currentUserId && review.approvedBy === currentUserId))}
          </p>
          {review.approvalNote && (
            <p className="text-xs text-muted-foreground">{review.approvalNote}</p>
          )}
        </div>
      ) : (
        <Textarea
          rows={2}
          value={note}
          placeholder={barText('note')}
          aria-label={barText('note')}
          onChange={(e) => setNote(e.target.value)}
        />
      )}
      <div className="flex flex-wrap items-center gap-2">
        {!approved && (
          <Button
            size="sm"
            disabled={busy}
            onClick={() => void approve()}
            data-testid="outcome-approve"
          >
            {barText('approve')}
          </Button>
        )}
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={onRequestChanges}
          data-testid="outcome-request-changes"
        >
          {requestChangesCountText(sentBackCount)}
        </Button>
        {error && (
          <span
            role="alert"
            className="text-sm text-destructive"
            data-testid="outcome-approve-error"
          >
            {error}
          </span>
        )}
      </div>
    </section>
  )
}
