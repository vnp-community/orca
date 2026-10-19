/**
 * SolutionAskBackDialog — CONTRACT-request-ui-api 2.6, CR-REQ-028 §2.5
 *
 * "Ask back" on a Solution's blocking open questions: one `clarification.request` for the
 * ticked questions. The reporter then answers in the Clarification panel (no second answer UI).
 *
 * @module components/request/solution/SolutionAskBackDialog
 */

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { useRequestCooldown } from '../../../hooks/useRequestCooldown'
import { backendErrorCode } from './solution-error-classification'
import type { RequestRpcError } from '../../../../../shared/request-errors'
import type { SolutionOpenQuestion } from '../../../../../shared/request-types'
import { requestErrorMessageForError } from '../request-error-message'
import {
  ASK_BACK_REASON_MAX,
  askBackPrompt,
  clipAskBackReason,
  defaultAskBackSelection,
  type AskBackItem
} from './solution-ask-back'
import { useAskBack } from './useAskBack'

const P = 'auto.components.request.SolutionAskBackDialog.'

export type SolutionAskBackDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  requestId: string
  solutionId: string
  questions: SolutionOpenQuestion[]
  /** Ids the server refused approval on; they are ticked first. */
  refusedIds: string[]
  /** Called after the Clarification was created (or returned again for the same questions). */
  onAsked: () => void
}

function defaultReason(q: SolutionOpenQuestion): string {
  return clipAskBackReason(
    translate(
      `${P}reasonDefault`,
      'The solution cannot be approved until this is answered ({{id}}): {{text}}',
      {
        id: q.id,
        text: q.text
      }
    )
  )
}

function askBackErrorMessage(error: RequestRpcError): string {
  const code = backendErrorCode(error)
  if (code === 'REQUEST_CLARIFICATION_STATE_NOT_ALLOWED') {
    return translate(
      `${P}notAllowed`,
      'Questions cannot be asked now: another question round may already be open, or the request is in a state that does not allow it.'
    )
  }
  if (code === 'REQUEST_CLARIFICATION_INVALID_ANSWER') {
    return translate(
      `${P}invalid`,
      'The question was rejected. Check that every reason is filled in and not too long.'
    )
  }
  return requestErrorMessageForError(error)
}

export function SolutionAskBackDialog({
  open,
  onOpenChange,
  requestId,
  solutionId,
  questions,
  refusedIds,
  onAsked
}: SolutionAskBackDialogProps): React.JSX.Element {
  const blocking = useMemo(() => questions.filter((q) => q.blocking), [questions])
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  const [reasons, setReasons] = useState<Record<string, string>>({})
  const [error, setError] = useState<RequestRpcError | null>(null)
  const { ask, submitting } = useAskBack({ requestId, solutionId })
  const writeCooling = useRequestCooldown('write').coolingDown

  const latest = useRef({ questions, refusedIds })
  latest.current = { questions, refusedIds }
  // Why: each opening starts from the server's refusal (or every blocking question) and fresh reasons.
  useEffect(() => {
    if (!open) {
      return
    }
    const { questions: qs, refusedIds: refused } = latest.current
    setSelected(new Set(defaultAskBackSelection(qs, refused)))
    setReasons(
      Object.fromEntries(qs.filter((q) => q.blocking).map((q) => [q.id, defaultReason(q)]))
    )
    setError(null)
  }, [open])

  const chosen = blocking.filter((q) => selected.has(q.id))
  const reasonsOk = chosen.every((q) => (reasons[q.id] ?? '').trim() !== '')
  const canSubmit = chosen.length > 0 && reasonsOk && !submitting && !writeCooling

  const submit = async (): Promise<void> => {
    if (!canSubmit) {
      return
    }
    setError(null)
    const items: AskBackItem[] = chosen.map((q) => ({
      questionId: q.id,
      prompt: askBackPrompt(q),
      reason: reasons[q.id] ?? ''
    }))
    const res = await ask(items)
    if (res.ok) {
      onOpenChange(false)
      onAsked()
    } else {
      setError(res.error)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => (submitting ? undefined : onOpenChange(next))}>
      <DialogContent data-testid="solution-ask-back-dialog">
        <DialogHeader>
          <DialogTitle>{translate(`${P}title`, 'Ask back')}</DialogTitle>
          <DialogDescription>
            {translate(
              `${P}description`,
              'Send the blocking questions to the reporter. After they answer, the solution is generated again and you choose an option again.'
            )}
          </DialogDescription>
        </DialogHeader>
        <ul className="max-h-[50vh] space-y-3 overflow-y-auto">
          {blocking.map((q) => {
            const on = selected.has(q.id)
            return (
              <li key={q.id} className="space-y-1.5 rounded-md border border-border p-3 text-sm">
                <label className="flex items-start gap-2">
                  <Checkbox
                    checked={on}
                    disabled={submitting}
                    aria-label={q.id}
                    onCheckedChange={(v) =>
                      setSelected((prev) => {
                        const next = new Set(prev)
                        if (v === true) {
                          next.add(q.id)
                        } else {
                          next.delete(q.id)
                        }
                        return next
                      })
                    }
                  />
                  <span className="min-w-0">
                    <span className="text-xs text-muted-foreground">{q.id}</span>
                    <span className="block whitespace-pre-wrap">{q.text}</span>
                  </span>
                </label>
                <Textarea
                  data-testid="solution-ask-back-reason"
                  data-question-id={q.id}
                  aria-label={translate(`${P}reasonLabel`, 'Why you are asking ({{id}})', {
                    id: q.id
                  })}
                  value={reasons[q.id] ?? ''}
                  disabled={!on || submitting}
                  rows={2}
                  maxLength={ASK_BACK_REASON_MAX}
                  aria-invalid={on && (reasons[q.id] ?? '').trim() === ''}
                  onChange={(e) => setReasons((prev) => ({ ...prev, [q.id]: e.target.value }))}
                />
              </li>
            )
          })}
        </ul>
        {chosen.length === 0 && (
          <p className="text-xs text-muted-foreground">
            {translate(`${P}pickOne`, 'Select at least one question.')}
          </p>
        )}
        {chosen.length > 0 && !reasonsOk && (
          <p role="alert" className="text-xs text-destructive">
            {translate(`${P}reasonRequired`, 'Every selected question needs a reason.')}
          </p>
        )}
        {error && (
          <p
            role="alert"
            data-testid="solution-ask-back-error"
            className="text-sm text-destructive"
          >
            {askBackErrorMessage(error)}
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>
            {translate(`${P}cancel`, 'Cancel')}
          </Button>
          <Button
            data-testid="solution-ask-back-submit"
            disabled={!canSubmit}
            onClick={() => void submit()}
          >
            {submitting && <Loader2 className="size-4 animate-spin" aria-hidden />}
            {translate(`${P}submit`, 'Send to reporter')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
