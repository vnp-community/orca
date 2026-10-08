/**
 * TypeConfirmationCard — CR-REQ-019-04
 *
 * Lets a human confirm or correct the AI-proposed request type (and size /
 * urgency) while the request is awaiting type confirmation, and shows the
 * classifying state.
 *
 * @module components/request/TypeConfirmationCard
 */

import React, { useEffect, useState } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { translate } from '@/i18n/i18n'
import { getScreenSubmitShortcutLabel, isScreenSubmitShortcut } from '@/lib/screen-submit-shortcut'
import { useRequestActions } from '../../hooks/useRequestActions'
import { notifyRequestActionFailure } from './request-action-feedback'
import { FILTERABLE_TYPES } from './request-list-filters'
import { summarizeRequestFlow } from './request-flow-summary'
import { isLowConfidence } from './request-stage-timeline-model'
import { RequestTypeBadge } from './RequestTypeBadge'
import type { OrcaRequest, RequestSize, RequestType, RequestUrgency } from '../../../../shared/request-types'

const T = 'auto.components.request.TypeConfirmationCard.'
const STUCK_CLASSIFYING_MS = 60_000
const SIZES: RequestSize[] = ['S', 'M', 'L']
const URGENCIES: Exclude<RequestUrgency, 'unknown'>[] = ['normal', 'urgent']

type Props = {
  request: OrcaRequest
  onChanged: () => void
}

export function TypeConfirmationCard({ request, onChanged }: Props): React.JSX.Element {
  const { confirmType, classify } = useRequestActions()
  const proposed: RequestType | '' = request.type === 'unknown' ? '' : request.type
  const [type, setType] = useState<RequestType | ''>(proposed)
  const [size, setSize] = useState<RequestSize | undefined>(request.size)
  const [urgency, setUrgency] = useState<RequestUrgency | undefined>(request.urgency)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [classifyLocked, setClassifyLocked] = useState(false)
  const [showAllReason, setShowAllReason] = useState(false)
  const [now, setNow] = useState(() => Date.now())

  // A new proposal (after reclassify / refetch) resets the editable fields.
  useEffect(() => {
    setType(request.type === 'unknown' ? '' : request.type)
    setSize(request.size)
    setUrgency(request.urgency)
  }, [request.type, request.size, request.urgency])

  useEffect(() => {
    if (request.status !== 'classifying') {return}
    const timer = setInterval(() => setNow(Date.now()), 5_000)
    return () => clearInterval(timer)
  }, [request.status])

  const runClassify = async (): Promise<void> => {
    if (busy) {return}
    setBusy(true)
    const result = await classify(request.id)
    setBusy(false)
    if (!result.ok) {
      if (result.error.kind === 'rate_limited' || result.error.code === 'REQUEST_CLASSIFICATION_LIMIT') {
        setClassifyLocked(true)
      } else {
        notifyRequestActionFailure(result.error, onChanged)
      }
      return
    }
    onChanged()
  }

  if (request.status === 'classifying') {
    const stuck = now - Date.parse(request.updatedAt) > STUCK_CLASSIFYING_MS
    return (
      <div className="flex items-center gap-2 rounded border border-border px-3 py-2 text-sm" data-testid="request-classifying">
        <Loader2 className="size-4 animate-spin text-primary" aria-hidden />
        <span className="flex-1">{translate(`${T}classifying`, 'AI is classifying this request')}</span>
        {stuck && (
          <Button size="xs" variant="outline" disabled={busy || classifyLocked} onClick={() => void runClassify()}>
            {translate(`${T}reclassify`, 'Reclassify')}
          </Button>
        )}
      </div>
    )
  }

  const changed = type !== '' && type !== proposed
  const confidencePct = request.confidence === undefined ? null : Math.round(request.confidence * 100)
  const reasonText = request.classificationReason ?? ''
  const longReason = reasonText.split('\n').length > 6 || reasonText.length > 400

  const confirm = async (): Promise<void> => {
    if (busy || type === '') {return}
    setBusy(true)
    const result = await confirmType({
      id: request.id,
      type,
      ...(size ? { size } : {}),
      ...(urgency && urgency !== 'unknown' ? { urgency } : {}),
      ...(reason.trim() ? { reason: reason.trim() } : {})
    })
    setBusy(false)
    if (!result.ok) {
      notifyRequestActionFailure(result.error, onChanged)
      return
    }
    toast.success(translate(`${T}confirmed`, 'Type confirmed'))
    onChanged()
  }

  return (
    <section
      className="flex flex-col gap-3 rounded border border-primary/40 bg-primary/5 p-3"
      aria-label={translate(`${T}title`, 'Confirm request type')}
      data-testid="type-confirmation-card"
    >
      <div className="flex items-center gap-2">
        <h2 className="text-sm font-medium text-foreground">{translate(`${T}title`, 'Confirm request type')}</h2>
        {proposed && <RequestTypeBadge type={proposed} />}
        {changed && (
          <span className="rounded border border-border px-1 text-[10px] text-muted-foreground">
            {translate(`${T}editedByHuman`, 'Edited by you')}
          </span>
        )}
      </div>

      {confidencePct !== null && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>{translate(`${T}confidence`, 'Confidence')}</span>
          <Progress value={confidencePct} className="h-1.5 w-32" aria-label={translate(`${T}confidence`, 'Confidence')} />
          <span>{confidencePct}%</span>
        </div>
      )}
      {proposed !== '' && isLowConfidence(request.confidence) && (
        <p role="alert" className="flex items-center gap-1 text-xs text-destructive">
          <AlertTriangle className="size-3.5" aria-hidden />
          {translate(`${T}lowConfidence`, 'The AI is not confident about this type. Please check it.')}
        </p>
      )}
      {reasonText && (
        <div className="text-xs text-muted-foreground">
          <p className={showAllReason ? 'whitespace-pre-wrap' : 'line-clamp-6 whitespace-pre-wrap'}>{reasonText}</p>
          {longReason && (
            <button type="button" className="mt-0.5 text-primary hover:underline" onClick={() => setShowAllReason((v) => !v)}>
              {showAllReason ? translate(`${T}showLess`, 'Show less') : translate(`${T}showMore`, 'Show more')}
            </button>
          )}
        </div>
      )}

      <div className="flex flex-wrap items-end gap-3">
        <div className="flex min-w-48 flex-col gap-1">
          <label className="text-xs text-muted-foreground" htmlFor="type-confirmation-type">
            {translate(`${T}type`, 'Type')}
          </label>
          <Select value={type} onValueChange={(v) => setType(v as RequestType)}>
            <SelectTrigger id="type-confirmation-type" size="sm" aria-invalid={type === ''}>
              <SelectValue placeholder={translate(`${T}typePlaceholder`, 'Choose a type')} />
            </SelectTrigger>
            <SelectContent>
              {FILTERABLE_TYPES.map((t) => (
                <SelectItem key={t} value={t}>
                  <span className="flex flex-col items-start">
                    <span>{translate(`auto.components.request.RequestType.${t}.label`, t)}</span>
                    <span className="text-[10px] text-muted-foreground">
                      {translate(`auto.components.request.RequestType.${t}.description`, '')}
                    </span>
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-muted-foreground">{translate(`${T}size`, 'Size')}</span>
          <Select value={size ?? ''} onValueChange={(v) => setSize(v as RequestSize)}>
            <SelectTrigger size="sm" className="w-20" aria-label={translate(`${T}size`, 'Size')}>
              <SelectValue placeholder="-" />
            </SelectTrigger>
            <SelectContent>
              {SIZES.map((s) => (
                <SelectItem key={s} value={s}>
                  {s}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-muted-foreground">{translate(`${T}urgency`, 'Urgency')}</span>
          <Select value={urgency && urgency !== 'unknown' ? urgency : ''} onValueChange={(v) => setUrgency(v as RequestUrgency)}>
            <SelectTrigger size="sm" className="w-28" aria-label={translate(`${T}urgency`, 'Urgency')}>
              <SelectValue placeholder="-" />
            </SelectTrigger>
            <SelectContent>
              {URGENCIES.map((u) => (
                <SelectItem key={u} value={u}>
                  {translate(`auto.components.request.RequestOverviewTab.urgency.${u}`, u)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {type === '' ? (
        <p role="alert" className="text-xs text-destructive">
          {translate(`${T}typeRequired`, 'Pick a type to continue.')}
        </p>
      ) : (
        <p className="text-xs text-muted-foreground">{summarizeRequestFlow(type)}</p>
      )}

      {changed && (
        <Textarea
          value={reason}
          rows={2}
          placeholder={translate(`${T}reasonPlaceholder`, 'Why this type? (optional)')}
          aria-label={translate(`${T}reason`, 'Reason')}
          onChange={(e) => setReason(e.target.value)}
          onKeyDown={(e) => {
            if (isScreenSubmitShortcut(e)) {
              e.preventDefault()
              void confirm()
            }
          }}
        />
      )}

      <div className="flex items-center gap-2">
        <Button size="sm" disabled={busy || type === ''} onClick={() => void confirm()} title={getScreenSubmitShortcutLabel()}>
          {translate(`${T}confirm`, 'Confirm type')}
        </Button>
        <Button size="sm" variant="outline" disabled={busy || classifyLocked} onClick={() => void runClassify()}>
          {translate(`${T}reclassify`, 'Reclassify')}
        </Button>
        {classifyLocked && (
          <span className="text-xs text-muted-foreground">
            {translate(`${T}classifyLimit`, 'Reclassification limit reached (5 attempts).')}
          </span>
        )}
      </div>
    </section>
  )
}
