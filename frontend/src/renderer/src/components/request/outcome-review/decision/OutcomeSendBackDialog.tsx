/**
 * OutcomeSendBackDialog — FE-OR-T311
 *
 * "Request changes": the findings to hand back, then the follow-up Request filled in from them for the person to read and send.
 * Sending back is a Request of its own (this one is finished and stays as it is). The items are recorded as sent back first, and
 * once the follow-up exists they are tied to it so the review can say where each went.
 *
 * @module components/request/outcome-review/decision/OutcomeSendBackDialog
 */

import React, { useMemo, useState } from 'react'
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
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { SpawnChildRequestDialog } from '../../SpawnChildRequestDialog'
import { errorText } from '../outcome-review-state-copy'
import { sectionText } from '../outcome-review-copy'
import { cardActionText, sendBackDialogText } from './outcome-decision-copy'
import { buildSendBackDraft } from './outcome-send-back-draft'
import type { DecisionResult } from './use-outcome-decisions'
import type { ChildRequestRule } from '../../request-stage-timeline-model'
import type { OutcomeReview, ReviewItem } from '../../../../../../shared/outcome-review-types'
import type { OrcaRequest } from '../../../../../../shared/request-types'

/** Anything may be escalated into a follow-up, whatever the type of the finished Request. */
const ESCALATION: ChildRequestRule = { reason: 'escalation', suggestedTypes: ['bug', 'task'] }

type Props = {
  request: OrcaRequest
  review: OutcomeReview
  /** Items the person can send back: the ones that ask for a decision and the ones already sent back. */
  candidates: ReviewItem[]
  /** Keys already recorded as sent back. */
  sentBack: ReadonlySet<string>
  decide: (itemKey: string, note: string) => Promise<DecisionResult>
  linkChild: (itemKeys: string[], childRequestId: string) => Promise<DecisionResult>
  onClose: () => void
  onChanged: () => void
}

export function OutcomeSendBackDialog({
  request,
  review,
  candidates,
  sentBack,
  decide,
  linkChild,
  onClose,
  onChanged
}: Props): React.JSX.Element {
  const [picked, setPicked] = useState<ReadonlySet<string>>(
    () => new Set(candidates.filter((c) => sentBack.has(c.key)).map((c) => c.key))
  )
  const [note, setNote] = useState('')
  const [step, setStep] = useState<'pick' | 'spawn'>('pick')
  const [working, setWorking] = useState(false)
  const [failure, setFailure] = useState<string | null>(null)
  const chosen = useMemo(() => candidates.filter((c) => picked.has(c.key)), [candidates, picked])
  const draft = useMemo(
    () => buildSendBackDraft({ requestTitle: request.title, review, items: chosen, note }),
    [request.title, review, chosen, note]
  )

  const toggle = (key: string, on: boolean): void =>
    setPicked((current) => {
      const next = new Set(current)
      if (on) {
        next.add(key)
      } else {
        next.delete(key)
      }
      return next
    })

  const proceed = async (): Promise<void> => {
    if (chosen.length === 0) {
      return
    }
    setWorking(true)
    setFailure(null)
    // Why one after the other: each write carries the version the one before produced.
    for (const item of chosen.filter((c) => !sentBack.has(c.key))) {
      const res = await decide(item.key, '')
      if (!res.ok) {
        setWorking(false)
        setFailure(errorText(res.error.code))
        return
      }
    }
    setWorking(false)
    setStep('spawn')
  }

  if (step === 'spawn') {
    return (
      <SpawnChildRequestDialog
        open
        request={request}
        prefill={{
          title: draft.title,
          body: draft.body,
          type: draft.suggestedType,
          rule: ESCALATION
        }}
        onOpenChange={(open) => !open && onClose()}
        onChanged={onChanged}
        onCreated={(child) =>
          void linkChild(
            chosen.map((c) => c.key),
            child.id
          )
        }
      />
    )
  }
  return (
    <Dialog open onOpenChange={(open) => !open && !working && onClose()}>
      <DialogContent
        className="max-h-[90vh] overflow-y-auto sm:max-w-xl"
        data-testid="outcome-send-back-dialog"
      >
        <DialogHeader>
          <DialogTitle>{sendBackDialogText('title')}</DialogTitle>
          <DialogDescription>{sendBackDialogText('description')}</DialogDescription>
        </DialogHeader>
        <ul className="flex flex-col gap-2">
          {candidates.map((item) => (
            <li
              key={item.key}
              className="flex items-start gap-2 text-sm"
              data-testid="outcome-send-back-candidate"
            >
              <Checkbox
                id={`sb-${item.key}`}
                checked={picked.has(item.key)}
                onCheckedChange={(v) => toggle(item.key, v === true)}
              />
              <Label
                htmlFor={`sb-${item.key}`}
                className="flex min-w-0 flex-col gap-0.5 font-normal"
              >
                <span className="font-medium">{item.title}</span>
                <span className="text-xs text-muted-foreground">{sectionText(item.section)}</span>
                {item.detail && (
                  <span className="text-xs text-muted-foreground">{item.detail}</span>
                )}
              </Label>
            </li>
          ))}
        </ul>
        <div className="flex flex-col gap-1">
          <Label htmlFor="sb-note">{sendBackDialogText('noteLabel')}</Label>
          <Textarea id="sb-note" rows={3} value={note} onChange={(e) => setNote(e.target.value)} />
        </div>
        {chosen.length === 0 && (
          <p className="text-xs text-muted-foreground">{sendBackDialogText('nothing')}</p>
        )}
        {failure && (
          <p
            role="alert"
            className="text-sm text-destructive"
            data-testid="outcome-send-back-failure"
          >
            {failure}
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" size="sm" disabled={working} onClick={onClose}>
            {cardActionText('cancel')}
          </Button>
          <Button
            size="sm"
            disabled={chosen.length === 0 || working}
            onClick={() => void proceed()}
            data-testid="outcome-send-back-continue"
          >
            {working ? sendBackDialogText('working') : sendBackDialogText('continue')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
