/**
 * SolutionPanelLoadStates — CR-REQ-020-02
 *
 * The Analysis tab while its first read is in flight, and when that read failed.
 *
 * @module components/request/solution/SolutionPanelLoadStates
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { requestErrorMessage } from '../request-error-message'

export function SolutionPanelLoading(): React.JSX.Element {
  return (
    <div data-testid="solution-panel-loading" aria-busy="true" className="space-y-3 p-4">
      <Skeleton className="h-8 w-1/3" />
      <Skeleton className="h-24 w-full" />
    </div>
  )
}

export function SolutionPanelError({
  error,
  onRetry
}: {
  error: Parameters<typeof requestErrorMessage>[0]
  onRetry: () => void
}): React.JSX.Element {
  return (
    <div role="alert" data-testid="solution-panel-error" className="space-y-2 p-4">
      <p className="text-sm text-destructive">
        {error === 'forbidden'
          ? translate(
              'auto.components.request.SolutionPanel.forbidden',
              'You do not have permission to view the analysis.'
            )
          : requestErrorMessage(error)}
      </p>
      {error !== 'forbidden' && (
        <Button size="sm" variant="outline" onClick={onRetry}>
          {translate('auto.components.request.SolutionPanel.retry', 'Retry')}
        </Button>
      )}
    </div>
  )
}
