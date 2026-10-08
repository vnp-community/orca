/**
 * ClarificationPanel — FE-REQ-TASK-036-03
 *
 * Shown while a Request is `awaiting_information`. Submits all answers at once
 * and never changes request state locally: the status event drives the UI.
 *
 * @module components/request/clarification/ClarificationPanel
 */

import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { MessageCircleQuestion } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useClarifications } from '../../../hooks/useClarifications'
import { requestErrorMessage } from '../request-error-message'
import { ClarificationDeadlineNote } from './ClarificationDeadlineNote'
import { ClarificationQuestionList } from './ClarificationQuestionList'
import { canSubmit, dueState, missingRequired, toAnswerPayload } from './clarification-answer-validation'
import type { OrcaRequest } from '../../../../../shared/request-types'

const T = 'auto.components.request.clarification.'
const WAIT_NOTE_DELAY_MS = 3000

const RESUME_STEP: Record<string, string> = { analyzing: 'analysis', planning: 'plan', executing: 'execution' }

type Props = { request: OrcaRequest; currentUserId: string | null; isAdmin: boolean }

/** Server messages may name the offending question: `... question_id=q1 ...`. */
function questionIdFromMessage(message: string, known: readonly string[]): string | null {
  const m = /question[_ ]?id[=: ]+([\w-]+)/i.exec(message)
  return m && known.includes(m[1]) ? m[1] : null
}

export function ClarificationPanel({ request, currentUserId, isAdmin }: Props): React.JSX.Element | null {
  const awaiting = request.status === 'awaiting_information'
  const c = useClarifications(awaiting ? request.id : null)
  const [accepted, setAccepted] = useState<ReadonlySet<string>>(new Set())
  const [showErrors, setShowErrors] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [banner, setBanner] = useState<{ text: string; retry: boolean } | null>(null)
  const [submitted, setSubmitted] = useState(false)
  const [stillMissing, setStillMissing] = useState(false)
  const [lockedReadOnly, setLockedReadOnly] = useState(false)
  const [showWaitNote, setShowWaitNote] = useState(false)

  const open = c.open
  useEffect(() => {
    setAccepted(new Set())
    setShowErrors(false)
    setFieldErrors({})
    setBanner(null)
    setLockedReadOnly(false)
    setSubmitted(false)
  }, [open?.id])

  useEffect(() => {
    if (!awaiting || open) {
      setShowWaitNote(false)
      return
    }
    const t = setTimeout(() => setShowWaitNote(true), WAIT_NOTE_DELAY_MS)
    return () => clearTimeout(t)
  }, [awaiting, open])

  // Why: answers may be sensitive and live only in memory, so warn before a reload drops them.
  useEffect(() => {
    if (!c.hasDraft) {return}
    const onBeforeUnload = (e: BeforeUnloadEvent): void => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [c.hasDraft])

  const assignees = open?.assigneeIds
  const notAssignee = Boolean(assignees && assignees.length > 0 && !isAdmin && (!currentUserId || !assignees.includes(currentUserId)))
  const overdue = dueState(open?.dueAt, Date.now()).kind === 'overdue'
  const readOnly = notAssignee || lockedReadOnly

  const ready = useMemo(() => (open ? canSubmit(open, c.draft, accepted) : false), [open, c.draft, accepted])

  const submit = useCallback(async () => {
    if (!open || readOnly || overdue || c.submitting) {return}
    if (!canSubmit(open, c.draft, accepted)) {
      setShowErrors(true)
      const first = missingRequired(open, c.draft, accepted)[0]
      document.querySelector<HTMLElement>(`[data-question-id="${CSS.escape(first)}"] textarea, [data-question-id="${CSS.escape(first)}"] input, [data-question-id="${CSS.escape(first)}"] button`)?.focus()
      return
    }
    setBanner(null)
    setFieldErrors({})
    const res = await c.answer(toAnswerPayload(open, c.draft, accepted))
    if (res.ok) {
      setSubmitted(true)
      setStillMissing(res.value.stillMissing)
      c.clearDraft()
      return
    }
    const { kind, message } = res.error
    if (kind === 'conflict') {
      c.refetch()
      setBanner({ text: requestErrorMessage('conflict'), retry: false })
    } else if (kind === 'expired' || kind === 'invalid_state' || kind === 'forbidden') {
      setLockedReadOnly(true)
      c.refetch()
      setBanner({ text: requestErrorMessage(kind === 'expired' ? 'invalid_state' : kind), retry: false })
    } else if (kind === 'validation') {
      const qid = questionIdFromMessage(message, open.questions.map((q) => q.id))
      if (qid) {setFieldErrors({ [qid]: message })}
      else {setBanner({ text: requestErrorMessage('validation'), retry: false })}
    } else {
      setBanner({ text: requestErrorMessage(kind), retry: true })
    }
  }, [open, readOnly, overdue, c, accepted])

  if (!awaiting) {return null}

  if (!open) {
    if (c.loading || !showWaitNote) {
      return (
        <div data-testid="clarification-skeleton">
          <Skeleton className="h-16 w-full" />
        </div>
      )
    }
    return (
      <p className="rounded-md border border-border px-3 py-2 text-sm text-muted-foreground" data-testid="clarification-waiting">
        {translate(`${T}waitingForInfo`, 'Waiting for the information to be provided')}
      </p>
    )
  }

  const resumeStep = open.resumeStatus ? RESUME_STEP[open.resumeStatus] : undefined
  const stepLabel = resumeStep ? translate(`auto.components.request.StageTimeline.step.${resumeStep}`, resumeStep) : ''

  return (
    <section
      className="flex flex-col gap-3 rounded-md border border-border bg-card px-4 py-3"
      aria-label={translate(`${T}title`, 'Information needed')}
      data-testid="clarification-panel"
    >
      <header className="flex items-center gap-2">
        <MessageCircleQuestion className="size-4 text-muted-foreground" aria-hidden />
        <h3 className="text-sm font-semibold">{translate(`${T}title`, 'Information needed')}</h3>
        <span className="text-xs text-muted-foreground">{open.displayId} · {translate(`${T}round`, 'Round {{round}}', { round: open.round })}</span>
      </header>
      <ClarificationDeadlineNote dueAt={open.dueAt} now={Date.now()} />
      {readOnly ? (
        <p className="text-xs text-muted-foreground" data-testid="clarification-readonly">
          {translate(`${T}readOnlyWaiting`, 'Waiting for the assigned person to answer')}
        </p>
      ) : null}
      {submitted ? (
        <p role="status" className="text-xs text-muted-foreground" data-testid="clarification-submitted">
          {stillMissing
            ? translate(`${T}stillMissing`, 'Some information is still missing; a new round was opened.')
            : translate(`${T}answeredResume`, 'Received. The AI is re-running the {{step}} step.', { step: stepLabel })}
        </p>
      ) : null}
      {banner ? (
        <div role="alert" className="flex items-center gap-2 text-xs text-destructive">
          <span>{banner.text}</span>
          {banner.retry ? (
            <Button size="xs" variant="outline" onClick={() => void submit()}>
              {translate(`${T}retry`, 'Retry')}
            </Button>
          ) : null}
        </div>
      ) : null}
      <ClarificationQuestionList
        clarification={open}
        draft={c.draft}
        acceptedDefaults={accepted}
        onChange={c.setDraftValue}
        onAcceptDefault={(id, on) =>
          setAccepted((prev) => {
            const next = new Set(prev)
            if (on) {next.add(id)} else {next.delete(id)}
            return next
          })
        }
        fieldErrors={fieldErrors}
        showErrors={showErrors}
        readOnly={readOnly}
        onSubmitShortcut={() => void submit()}
      />
      {!readOnly ? (
        <div className="flex justify-end">
          <Button size="sm" disabled={!ready || overdue || c.submitting} onClick={() => void submit()}>
            {translate(`${T}submit`, 'Submit answers')}
          </Button>
        </div>
      ) : null}
    </section>
  )
}
