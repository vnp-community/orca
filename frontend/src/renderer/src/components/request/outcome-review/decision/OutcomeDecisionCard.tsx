/**
 * OutcomeDecisionCard — FE-OR-T301
 *
 * The controls of one item that asks for a decision: accept it as an exception (a note is required, and the error appears next to
 * the field, not in a toast), send it back, or, for something an agent only claimed, dismiss the claim. A decision already made
 * is shown with who made it and can be changed; the earlier ones stay as history on the server.
 *
 * @module components/request/outcome-review/decision/OutcomeDecisionCard
 */

import React, { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { errorText } from '../outcome-review-state-copy'
import {
  cardActionText,
  decidedText,
  noteErrorText,
  noteLabelText,
  openChildText,
  sentBackInText,
  whoText
} from './outcome-decision-copy'
import type { DecisionResult } from './use-outcome-decisions'
import type {
  ReviewDecision,
  ReviewDecisionValue,
  ReviewItem
} from '../../../../../../shared/outcome-review-types'

/** The server's limit for a note. */
export const MAX_DECISION_NOTE = 500

type Mode = 'idle' | 'accept_exception' | 'dismiss'

type Props = {
  item: ReviewItem
  decision: ReviewDecision | null
  /** The viewer's id, to say "you". */
  currentUserId: string | null
  busy: boolean
  onDecide: (decision: ReviewDecisionValue, note: string) => Promise<DecisionResult>
  onOpenChild?: (childRequestId: string) => void
}

export function OutcomeDecisionCard({
  item,
  decision,
  currentUserId,
  busy,
  onDecide,
  onOpenChild
}: Props): React.JSX.Element {
  const [mode, setMode] = useState<Mode>('idle')
  const [changing, setChanging] = useState(false)
  const [note, setNote] = useState('')
  const [error, setError] = useState<string | null>(null)
  const id = `decision-note-${item.key}`

  const submit = async (value: ReviewDecisionValue): Promise<void> => {
    const needsNote = value !== 'send_back'
    if (needsNote && note.trim() === '') {
      setError(noteErrorText('required', MAX_DECISION_NOTE))
      return
    }
    if (note.trim().length > MAX_DECISION_NOTE) {
      setError(noteErrorText('tooLong', MAX_DECISION_NOTE))
      return
    }
    setError(null)
    const res = await onDecide(value, note)
    if (res.ok) {
      setMode('idle')
      setChanging(false)
      setNote('')
    } else {
      setError(errorText(res.error.code))
    }
  }

  const decided = decision !== null && !changing
  return (
    <div
      className="flex flex-col gap-2"
      data-testid="outcome-decision-card"
      data-item-key={item.key}
    >
      {decided ? (
        <div
          className="flex flex-col gap-1 text-sm"
          data-testid="outcome-decision-made"
          data-decision={decision.decision}
        >
          <p>
            {decidedText(
              decision.decision,
              whoText(!!currentUserId && decision.decidedBy === currentUserId)
            )}
          </p>
          {decision.note && <p className="text-xs text-muted-foreground">{decision.note}</p>}
          {decision.childRequestId && (
            <p className="flex items-center gap-2 text-xs text-muted-foreground">
              {sentBackInText()}
              {onOpenChild && (
                <Button
                  size="xs"
                  variant="link"
                  onClick={() => onOpenChild(decision.childRequestId!)}
                >
                  {openChildText()}
                </Button>
              )}
            </p>
          )}
          <Button
            size="xs"
            variant="ghost"
            className="self-start"
            disabled={busy}
            onClick={() => setChanging(true)}
          >
            {cardActionText('change')}
          </Button>
        </div>
      ) : mode === 'idle' ? (
        <div className="flex flex-wrap gap-2">
          <Button
            size="xs"
            variant="outline"
            disabled={busy}
            onClick={() => setMode('accept_exception')}
          >
            {cardActionText('accept')}
          </Button>
          <Button
            size="xs"
            variant="outline"
            disabled={busy}
            onClick={() => void submit('send_back')}
          >
            {cardActionText('sendBack')}
          </Button>
          {item.origin === 'agent' && (
            <Button size="xs" variant="ghost" disabled={busy} onClick={() => setMode('dismiss')}>
              {cardActionText('dismiss')}
            </Button>
          )}
          {changing && (
            <Button size="xs" variant="ghost" onClick={() => setChanging(false)}>
              {cardActionText('cancel')}
            </Button>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-1">
          <Label htmlFor={id}>{noteLabelText(true)}</Label>
          <Textarea
            id={id}
            rows={2}
            value={note}
            aria-invalid={error !== null}
            aria-describedby={error ? `${id}-error` : undefined}
            onChange={(e) => {
              setNote(e.target.value)
              setError(null)
            }}
          />
          {error && (
            <p
              id={`${id}-error`}
              role="alert"
              className="text-xs text-destructive"
              data-testid="outcome-decision-error"
            >
              {error}
            </p>
          )}
          <div className="flex gap-2">
            <Button
              size="xs"
              disabled={busy}
              onClick={() => void submit(mode)}
              data-testid="outcome-decision-confirm"
            >
              {mode === 'accept_exception'
                ? cardActionText('confirmAccept')
                : cardActionText('confirmDismiss')}
            </Button>
            <Button
              size="xs"
              variant="ghost"
              onClick={() => {
                setMode('idle')
                setError(null)
              }}
            >
              {cardActionText('cancel')}
            </Button>
          </div>
        </div>
      )}
      {error && mode === 'idle' && (
        <p role="alert" className="text-xs text-destructive" data-testid="outcome-decision-error">
          {error}
        </p>
      )}
    </div>
  )
}
