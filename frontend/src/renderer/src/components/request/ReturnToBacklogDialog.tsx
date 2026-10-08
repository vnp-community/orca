/**
 * ReturnToBacklogDialog — CR-REQ-019-03
 *
 * Sends a request back to the backlog from the chosen stage; the reason is
 * mandatory (backend: REQUEST_REASON_REQUIRED).
 *
 * @module components/request/ReturnToBacklogDialog
 */

import React, { useState } from 'react'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import { RequestReasonDialog } from './RequestReasonDialog'
import { RETURN_STAGES, defaultReturnStage } from './request-action-rules'
import type { RequestStatus, ReturnedFromStage } from '../../../../shared/request-types'

const T = 'auto.components.request.ReturnToBacklogDialog.'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  status: RequestStatus
  onConfirm: (stage: ReturnedFromStage, reason: string) => Promise<boolean>
}

export function ReturnToBacklogDialog({ open, onOpenChange, status, onConfirm }: Props): React.JSX.Element {
  const [stage, setStage] = useState<ReturnedFromStage>(() => defaultReturnStage(status))

  return (
    <RequestReasonDialog
      open={open}
      onOpenChange={onOpenChange}
      title={translate(`${T}title`, 'Return to backlog')}
      description={translate(`${T}description`, 'The request leaves the active flow and can be reopened later.')}
      reasonLabel={translate(`${T}reason`, 'Why is it being returned?')}
      submitLabel={translate(`${T}confirm`, 'Return to backlog')}
      reasonRequired
      onSubmit={(reason) => onConfirm(stage, reason)}
    >
      <div className="flex flex-col gap-1.5">
        <Label>{translate(`${T}stage`, 'Returned from stage')}</Label>
        <Select value={stage} onValueChange={(v) => setStage(v as ReturnedFromStage)}>
          <SelectTrigger aria-label={translate(`${T}stage`, 'Returned from stage')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {RETURN_STAGES.map((s) => (
              <SelectItem key={s} value={s}>
                {translate(`auto.components.request.StageTimeline.step.${s === 'task' ? 'execution' : s}`, s)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </RequestReasonDialog>
  )
}
