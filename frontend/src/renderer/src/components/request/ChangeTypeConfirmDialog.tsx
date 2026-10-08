/**
 * ChangeTypeConfirmDialog — CR-REQ-019-04
 *
 * Changes the type of an already-confirmed request. A reason is mandatory;
 * when the backend says the change must go through a child request
 * (REQUEST_TYPE_CHANGE_USE_CHILD) the dialog explains and offers that path.
 *
 * @module components/request/ChangeTypeConfirmDialog
 */

import React, { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import { useRequestActions } from '../../hooks/useRequestActions'
import { notifyRequestActionFailure } from './request-action-feedback'
import { FILTERABLE_TYPES } from './request-list-filters'
import { RequestReasonDialog } from './RequestReasonDialog'
import type { OrcaRequest, RequestType } from '../../../../shared/request-types'

const T = 'auto.components.request.ChangeTypeConfirmDialog.'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  request: OrcaRequest
  onChanged: () => void
  onSpawnChild: () => void
}

export function ChangeTypeConfirmDialog({ open, onOpenChange, request, onChanged, onSpawnChild }: Props): React.JSX.Element {
  const { changeType } = useRequestActions()
  const [toType, setToType] = useState<RequestType | ''>('')
  const [useChild, setUseChild] = useState(false)

  const submit = async (reason: string): Promise<boolean> => {
    if (toType === '') {return false}
    const result = await changeType({ id: request.id, toType, reason })
    if (!result.ok) {
      if (result.error.code === 'REQUEST_TYPE_CHANGE_USE_CHILD') {setUseChild(true)}
      else {notifyRequestActionFailure(result.error, onChanged)}
      return false
    }
    toast.success(translate(`${T}done`, 'Type changed'))
    onChanged()
    return true
  }

  return (
    <RequestReasonDialog
      open={open}
      onOpenChange={onOpenChange}
      title={translate(`${T}title`, 'Change request type')}
      description={translate(
        `${T}description`,
        'Existing solutions, plans and tasks are kept as references and the request returns to type confirmation.'
      )}
      reasonLabel={translate(`${T}reason`, 'Why is the type changing?')}
      submitLabel={translate(`${T}confirm`, 'Change type')}
      reasonRequired
      onSubmit={submit}
    >
      <div className="flex flex-col gap-1.5">
        <Label>{translate(`${T}newType`, 'New type')}</Label>
        <Select value={toType} onValueChange={(v) => setToType(v as RequestType)}>
          <SelectTrigger aria-label={translate(`${T}newType`, 'New type')}>
            <SelectValue placeholder={translate(`${T}placeholder`, 'Choose a type')} />
          </SelectTrigger>
          <SelectContent>
            {FILTERABLE_TYPES.filter((t) => t !== request.type).map((t) => (
              <SelectItem key={t} value={t}>
                {translate(`auto.components.request.RequestType.${t}.label`, t)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {useChild && (
        <div role="alert" className="flex flex-col items-start gap-2 rounded border border-border bg-muted/40 p-2 text-sm">
          <span>{translate(`${T}useChild`, 'This type change must be done with a child request.')}</span>
          <Button
            size="xs"
            variant="outline"
            onClick={() => {
              onOpenChange(false)
              onSpawnChild()
            }}
          >
            {translate(`${T}spawnChild`, 'Create child request')}
          </Button>
        </div>
      )}
    </RequestReasonDialog>
  )
}
