/**
 * ClarificationLoadFailureNotice — TASK-FE-E2E-002
 *
 * Named dead-end state: the Request waits for a person but its questions could not be
 * loaded. Always shows the cause, the backend code and a retry, never an empty panel.
 *
 * @module components/request/clarification/ClarificationLoadFailureNotice
 */

import React from 'react'
import { AlertCircle } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { requestErrorMessage } from '../request-error-message'
import type { ClarificationLoadFailure } from '../../../hooks/useClarifications'

const T = 'auto.components.request.clarification.'

type Props = {
  failure: ClarificationLoadFailure
  onRetry: () => void
}

export function ClarificationLoadFailureNotice({ failure, onRetry }: Props): React.JSX.Element {
  const cause =
    failure.kind === 'unsupported'
      ? translate(
          `${T}loadFailedUnsupported`,
          'This server does not support answering questions yet. Ask your administrator to update it.'
        )
      : requestErrorMessage(failure.kind)
  return (
    <div
      role="alert"
      data-testid="clarification-load-failed"
      data-error-kind={failure.kind}
      className="flex flex-col gap-2 rounded-md border border-destructive/30 px-3 py-2"
    >
      <p className="flex items-center gap-2 text-sm text-destructive">
        <AlertCircle className="size-4" aria-hidden />
        {translate(`${T}loadFailedTitle`, 'Could not load the questions')}
      </p>
      <p className="text-xs text-muted-foreground">{cause}</p>
      <p className="text-xs text-muted-foreground" data-testid="clarification-load-failed-code">
        {translate(`${T}loadFailedCode`, 'Error code: {{code}}', { code: failure.code })}
      </p>
      <div>
        <Button size="xs" variant="outline" onClick={onRetry}>
          {translate(`${T}retry`, 'Retry')}
        </Button>
      </div>
    </div>
  )
}
