/**
 * SolutionGenerationState — CR-REQ-020-02
 *
 * Empty / drafting / failed states of the analysis, with a regenerate action.
 *
 * @module components/request/solution/SolutionGenerationState
 */

import React from 'react'
import { AlertCircle, Loader2 } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { requestErrorMessage } from '../request-error-message'

export type SolutionGenerationStateProps = {
  /** 'analyzing' = no solution yet, AI working; 'drafting' = a draft exists; 'failed' = generate failed. */
  variant: 'analyzing' | 'drafting' | 'failed'
  errorKind?: string | null
  onRegenerate?: () => void
  /** True while a generate call is in flight or briefly rate-limited. */
  regenerateDisabled?: boolean
}

export function SolutionGenerationState({
  variant,
  errorKind,
  onRegenerate,
  regenerateDisabled = false
}: SolutionGenerationStateProps): React.JSX.Element {
  if (variant === 'failed') {
    return (
      <div role="alert" data-testid="solution-generation-failed" className="space-y-2 rounded-md border border-destructive/30 p-3">
        <p className="flex items-center gap-2 text-sm text-destructive">
          <AlertCircle className="size-4" aria-hidden />
          {translate('auto.components.request.SolutionGenerationState.failed', 'Could not generate the analysis.')}
        </p>
        {errorKind && <p className="text-xs text-muted-foreground">{requestErrorMessage(errorKind)}</p>}
        {onRegenerate && (
          <Button size="sm" variant="outline" disabled={regenerateDisabled} onClick={onRegenerate}>
            {translate('auto.components.request.SolutionGenerationState.regenerate', 'Regenerate')}
          </Button>
        )}
      </div>
    )
  }
  return (
    <div data-testid="solution-generation-pending" aria-busy="true" className="space-y-3">
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" aria-hidden />
        {variant === 'analyzing'
          ? translate('auto.components.request.SolutionGenerationState.analyzing', 'AI is analyzing this request')
          : translate('auto.components.request.SolutionGenerationState.drafting', 'AI is drafting the analysis')}
      </p>
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-5/6" />
    </div>
  )
}
