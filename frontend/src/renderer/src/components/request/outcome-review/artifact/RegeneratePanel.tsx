/**
 * RegeneratePanel — FE-OR-T215
 *
 * Starts a regeneration of what the review drew or read. It costs an agent run and replaces what others may have looked at, so a
 * reason is required, the server's estimate is shown before anything starts (asking for it starts nothing), and a cost above
 * the limit needs a second yes. A button pressed twice is one run: the request carries a key.
 *
 * @module components/request/outcome-review/artifact/RegeneratePanel
 */

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
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
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  estimateRegeneration,
  regenerate,
  type RegenerationReason,
  type RegenerationScope
} from '../../../../hooks/reviewArtifactCalls'
import { errorText } from '../outcome-review-state-copy'
import { estimateText, reasonText, scopeText } from './review-artifact-copy'
import type { RegeneratePreset } from './ReviewArtifactView'
import type { RegenerationEstimate } from '../../../../../../shared/outcome-review-types'

const T = 'auto.components.request.outcomeReview.artifact.RegeneratePanel.'
const REASONS: RegenerationReason[] = [
  'looks_wrong',
  'kg_changed',
  'code_changed',
  'style_changed',
  'retry_failed',
  'other'
]
const SCOPES: RegenerationScope[] = ['artifact', 'context', 'all']
export const MAX_NOTE_CHARS = 200

const SELECT =
  'h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30'

type Props = {
  requestId: string
  /** The version of the review the person is looking at; the server refuses a regeneration based on an older one. */
  reviewVersion: number
  preset: RegeneratePreset
  onClose: () => void
  /** A regeneration started: read the review again. */
  onStarted: () => void
}

export function RegeneratePanel({
  requestId,
  reviewVersion,
  preset,
  onClose,
  onStarted
}: Props): React.JSX.Element {
  const [scope, setScope] = useState<RegenerationScope>('artifact')
  const [reason, setReason] = useState<RegenerationReason | ''>(preset.reason ?? '')
  const [note, setNote] = useState('')
  const [confirm, setConfirm] = useState(false)
  const [estimate, setEstimate] = useState<RegenerationEstimate | null>(null)
  const [estimateFailure, setEstimateFailure] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [failure, setFailure] = useState<string | null>(null)
  // One key per opening of the panel, so a button pressed twice is one run.
  const key = useRef(globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`)
  const sections = useMemo(
    () => (scope === 'context' ? [] : preset.sections),
    [scope, preset.sections]
  )

  useEffect(() => {
    let live = true
    setEstimate(null)
    setEstimateFailure(null)
    void estimateRegeneration(requestId, { scope, sections }).then((res) => {
      if (!live) {
        return
      }
      if (res.ok) {
        setEstimate(res.value.regeneration ?? null)
      } else {
        setEstimateFailure(errorText(res.error.code))
      }
    })
    return () => {
      live = false
    }
  }, [requestId, scope, sections])

  const needsConfirm = estimate?.confirmRequired === true
  const ready = reason !== '' && (!needsConfirm || confirm) && !submitting

  const submit = async (): Promise<void> => {
    if (reason === '') {
      return
    }
    setSubmitting(true)
    setFailure(null)
    const res = await regenerate(requestId, {
      scope,
      sections,
      reason,
      note: note.trim(),
      confirmCost: confirm,
      clientRequestId: key.current,
      expectedVersion: reviewVersion
    })
    setSubmitting(false)
    if (!res.ok) {
      setFailure(errorText(res.error.code))
      return
    }
    toast.success(
      res.value.replayed
        ? translate(`${T}replayed`, 'That request was already received')
        : translate(`${T}started`, 'Regeneration started')
    )
    onStarted()
    onClose()
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md" data-testid="regenerate-panel">
        <DialogHeader>
          <DialogTitle>{translate(`${T}title`, 'Regenerate')}</DialogTitle>
          <DialogDescription>
            {translate(
              `${T}description`,
              'This costs an agent run and replaces what others may have looked at.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1">
            <Label htmlFor="regen-scope">{translate(`${T}scope`, 'What to do')}</Label>
            <select
              id="regen-scope"
              className={SELECT}
              value={scope}
              onChange={(e) => setScope(e.target.value as RegenerationScope)}
            >
              {SCOPES.map((s) => (
                <option key={s} value={s}>
                  {scopeText(s)}
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="regen-reason">{translate(`${T}reason`, 'Why')}</Label>
            <select
              id="regen-reason"
              className={SELECT}
              value={reason}
              onChange={(e) => setReason(e.target.value as RegenerationReason | '')}
              aria-required="true"
            >
              <option value="">{translate(`${T}reasonPlaceholder`, 'Choose a reason')}</option>
              {REASONS.map((r) => (
                <option key={r} value={r}>
                  {reasonText(r)}
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor="regen-note">{translate(`${T}note`, 'Note (optional)')}</Label>
            <Textarea
              id="regen-note"
              rows={2}
              maxLength={MAX_NOTE_CHARS}
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
          <div
            className="rounded-md border border-border p-2 text-xs text-muted-foreground"
            aria-live="polite"
            data-testid="regenerate-estimate"
          >
            {estimateFailure ? (
              <span role="alert">{estimateFailure}</span>
            ) : estimate === null ? (
              translate(`${T}estimating`, 'Working out what this would do…')
            ) : (
              estimateText(estimate).map((line) => <p key={line}>{line}</p>)
            )}
          </div>
          {needsConfirm && (
            <div className="flex items-center gap-2">
              <Checkbox
                id="regen-confirm"
                checked={confirm}
                onCheckedChange={(v) => setConfirm(v === true)}
              />
              <Label htmlFor="regen-confirm">
                {translate(`${T}confirm`, 'This costs more than the limit. Go ahead anyway.')}
              </Label>
            </div>
          )}
          {failure && (
            <p role="alert" className="text-sm text-destructive" data-testid="regenerate-failure">
              {failure}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose}>
            {translate(`${T}cancel`, 'Cancel')}
          </Button>
          <Button
            size="sm"
            disabled={!ready}
            onClick={() => void submit()}
            data-testid="regenerate-submit"
          >
            {translate(`${T}submit`, 'Regenerate')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
