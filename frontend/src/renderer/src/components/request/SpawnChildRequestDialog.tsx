/**
 * SpawnChildRequestDialog — CR-REQ-019-05
 *
 * Creates a child request (spike/question hand-off, hotfix follow-up,
 * escalation). The link reason comes from CHILD_REQUEST_RULES.
 *
 * @module components/request/SpawnChildRequestDialog
 */

import React, { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { translate } from '@/i18n/i18n'
import { getScreenSubmitShortcutLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import { useAppStore } from '@/store'
import { useRequestActions } from '../../hooks/useRequestActions'
import { notifyRequestActionFailure } from './request-action-feedback'
import { CHILD_REQUEST_RULES } from './request-stage-timeline-model'
import type { OrcaRequest, RequestType } from '../../../../shared/request-types'

const T = 'auto.components.request.SpawnChildRequestDialog.'
const LET_AI_CLASSIFY = '__ai__'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  request: OrcaRequest
  /** Plain-text summary of the analysis result used to pre-fill the body. */
  bodyHint?: string
  onChanged: () => void
}

export function SpawnChildRequestDialog({ open, onOpenChange, request, bodyHint, onChanged }: Props): React.JSX.Element | null {
  const { spawnChild } = useRequestActions()
  const rule = request.type === 'unknown' ? undefined : CHILD_REQUEST_RULES[request.type]
  const [title, setTitle] = useState(() =>
    rule ? `${request.title} (${translate(`${T}reason.${rule.reason}`, rule.reason)})` : request.title
  )
  const [body, setBody] = useState(bodyHint ?? '')
  const [type, setType] = useState<string>(LET_AI_CLASSIFY)
  const [submitting, setSubmitting] = useState(false)
  if (!rule) {return null}

  const blocked = submitting || title.trim().length === 0

  const submit = async (): Promise<void> => {
    if (blocked) {return}
    setSubmitting(true)
    const result = await spawnChild({
      id: request.id,
      reason: rule.reason,
      title: title.trim(),
      body: body.trim() || undefined,
      type: type === LET_AI_CLASSIFY ? undefined : (type as RequestType),
      clientRequestId: crypto.randomUUID()
    })
    setSubmitting(false)
    if (!result.ok) {
      notifyRequestActionFailure(result.error, onChanged)
      return
    }
    const child = result.value.request
    onOpenChange(false)
    onChanged()
    useAppStore.getState().upsertRequests([child])
    useAppStore.getState().setRequestPageData({ section: 'requests', requestId: child.id })
    toast.success(translate(`${T}created`, 'Child request created'), {
      action: {
        label: translate(`${T}backToParent`, 'Back to parent'),
        onClick: () => useAppStore.getState().setRequestPageRequest(request.id)
      }
    })
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !submitting && onOpenChange(next)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{translate(`${T}title`, 'Create child request')}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="spawn-child-title">{translate(`${T}titleLabel`, 'Title')}</Label>
            <Input id="spawn-child-title" value={title} maxLength={500} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="spawn-child-body">{translate(`${T}body`, 'Description')}</Label>
            <Textarea
              id="spawn-child-body"
              rows={5}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              onKeyDown={(e) => {
                if (isScreenSubmitShortcut(e)) {
                  e.preventDefault()
                  void submit()
                }
              }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>{translate(`${T}type`, 'Type')}</Label>
            <Select value={type} onValueChange={setType}>
              <SelectTrigger aria-label={translate(`${T}type`, 'Type')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={LET_AI_CLASSIFY}>{translate(`${T}letAi`, 'Let the AI classify')}</SelectItem>
                {rule.suggestedTypes.map((t) => (
                  <SelectItem key={t} value={t}>
                    {translate(`auto.components.request.RequestType.${t}.label`, t)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>
            {translate('auto.components.request.RequestReasonDialog.cancel', 'Cancel')}
          </Button>
          <Button disabled={blocked} onClick={() => void submit()} title={getScreenSubmitShortcutLabel()}>
            {translate(`${T}submit`, 'Create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
