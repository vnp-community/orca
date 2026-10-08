/**
 * CancelRequestDialog — CR-REQ-019-03
 *
 * @module components/request/CancelRequestDialog
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { RequestReasonDialog } from './RequestReasonDialog'

const T = 'auto.components.request.CancelRequestDialog.'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (reason: string) => Promise<boolean>
  /** Set after the server answered REQUEST_REASON_REQUIRED (CR-REQ-023). */
  reasonRequired?: boolean
}

export function CancelRequestDialog({ open, onOpenChange, onConfirm, reasonRequired = false }: Props): React.JSX.Element {
  return (
    <RequestReasonDialog
      open={open}
      onOpenChange={onOpenChange}
      title={translate(`${T}title`, 'Cancel this request?')}
      description={translate(`${T}description`, 'Work in progress stops. You can still read the request afterwards.')}
      reasonLabel={reasonRequired ? translate(`${T}reasonRequired`, 'Reason (required)') : translate(`${T}reason`, 'Reason (optional)')}
      submitLabel={translate(`${T}confirm`, 'Cancel request')}
      reasonRequired={reasonRequired}
      destructive
      onSubmit={onConfirm}
    />
  )
}
