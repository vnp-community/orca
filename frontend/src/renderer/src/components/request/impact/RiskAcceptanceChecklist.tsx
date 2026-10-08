/**
 * RiskAcceptanceChecklist — FE-REQ-TASK-036-06
 *
 * One reasoned acceptance per high-or-above finding. Drafts stay in state so a
 * digest change lets the approver re-confirm quickly.
 *
 * @module components/request/impact/RiskAcceptanceChecklist
 */

import React, { useState } from 'react'
import { Check } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { RiskBadge } from '../../graph/RiskBadge'
import { RISK_ACCEPT_MIN_LENGTH, type ApprovalRequirements } from './risk-approval-rules'
import type { AcceptedInfo } from './useRiskApprovalGate'
import type { Result } from '../../../runtime/request-rpc-client'

const T = 'auto.components.request.impact.'

type Props = {
  requirements: ApprovalRequirements
  accepted: ReadonlySet<string>
  info: Readonly<Record<string, AcceptedInfo>>
  invalidated: boolean
  onAccept: (findingId: string, rationale: string) => Promise<Result<unknown>>
  disabled?: boolean
}

export function RiskAcceptanceChecklist({ requirements, accepted, info, invalidated, onAccept, disabled }: Props): React.JSX.Element | null {
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [touched, setTouched] = useState<Record<string, boolean>>({})
  const [busyId, setBusyId] = useState<string | null>(null)
  const [errorId, setErrorId] = useState<string | null>(null)

  if (requirements.mustAccept.length === 0 && !requirements.needsSecondApprover) {return null}

  const submit = async (id: string): Promise<void> => {
    setTouched((t) => ({ ...t, [id]: true }))
    const text = (drafts[id] ?? '').trim()
    if (text.length < RISK_ACCEPT_MIN_LENGTH || busyId) {return}
    setBusyId(id)
    setErrorId(null)
    const res = await onAccept(id, text)
    setBusyId(null)
    if (!res.ok) {setErrorId(id)}
  }

  return (
    <section className="flex flex-col gap-2 rounded-md border border-border px-3 py-2" data-testid="risk-acceptance-checklist" aria-label={translate(`${T}acceptance.title`, 'Accept the remaining risks')}>
      <h4 className="text-xs font-medium">{translate(`${T}acceptance.title`, 'Accept the remaining risks')}</h4>
      {invalidated ? (
        <p role="alert" className="text-xs text-destructive" data-testid="risk-acceptance-invalidated">
          {translate(`${T}acceptance.invalidated`, 'The assessment changed. Please confirm again.')}
        </p>
      ) : null}
      <ul className="flex flex-col gap-2">
        {requirements.mustAccept.map((f) => {
          const done = accepted.has(f.id)
          const text = drafts[f.id] ?? ''
          const invalid = touched[f.id] === true && text.trim().length < RISK_ACCEPT_MIN_LENGTH
          return (
            <li key={f.id} className="flex flex-col gap-1 text-xs" data-finding-id={f.id}>
              <div className="flex flex-wrap items-center gap-2">
                <RiskBadge level={f.level} size="sm" />
                <span className="min-w-0 flex-1 break-words font-medium">{f.title}</span>
                {done ? (
                  <span className="inline-flex items-center gap-1 text-muted-foreground">
                    <Check className="size-3" aria-hidden />
                    {translate(`${T}acceptance.recorded`, 'Accepted')}
                    {info[f.id]?.acceptedBy ? ` · ${info[f.id].acceptedBy}` : ''}
                    {info[f.id]?.createdAt ? ` · ${info[f.id].createdAt}` : ''}
                  </span>
                ) : null}
              </div>
              {!done ? (
                <>
                  <Label htmlFor={`accept-${f.id}`} className="text-[11px] font-normal text-muted-foreground">
                    {translate(`${T}acceptance.reasonLabel`, 'Reason for accepting')}
                  </Label>
                  <Textarea
                    id={`accept-${f.id}`}
                    rows={2}
                    value={text}
                    disabled={disabled || busyId !== null}
                    aria-invalid={invalid ? true : undefined}
                    onBlur={() => setTouched((t) => ({ ...t, [f.id]: true }))}
                    onChange={(e) => setDrafts((d) => ({ ...d, [f.id]: e.target.value }))}
                  />
                  <div className="flex items-center justify-between">
                    <span className={invalid ? 'text-destructive' : 'text-muted-foreground'}>
                      {translate(`${T}acceptance.reasonRequired`, 'At least {{min}} characters', { min: RISK_ACCEPT_MIN_LENGTH })}
                    </span>
                    <Button size="xs" variant="outline" disabled={disabled || busyId !== null || text.trim().length < RISK_ACCEPT_MIN_LENGTH} onClick={() => void submit(f.id)}>
                      {translate(`${T}acceptance.record`, 'Record acceptance')}
                    </Button>
                  </div>
                  {errorId === f.id ? <p role="alert" className="text-destructive">{translate(`${T}acceptance.error`, 'Could not record the acceptance')}</p> : null}
                </>
              ) : null}
            </li>
          )
        })}
      </ul>
      {requirements.needsSecondApprover ? (
        <p className="text-xs text-muted-foreground" data-testid="risk-second-approver">
          {translate(`${T}acceptance.secondApprover`, 'Waiting for a second approver')}. {translate(`${T}acceptance.splitPhase`, 'Consider splitting into smaller phases and rehearsing a rollback.')}
        </p>
      ) : null}
    </section>
  )
}
