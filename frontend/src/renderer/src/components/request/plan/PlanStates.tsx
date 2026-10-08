/**
 * PlanStates — CR-REQ-021-05
 *
 * Empty / loading / error presentations for the Plan tab. None of them is a red error
 * unless the load really failed: a missing Plan or channel is an expected state.
 *
 * @module components/request/plan/PlanStates
 */

import React from 'react'
import { AlertTriangle, ClipboardList, Loader2, Lock, Sparkles } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { requestErrorMessage } from '../request-error-message'
import { RequestUnsupportedNotice } from '../RequestUnsupportedNotice'
import type { PlanViewState } from './plan-view-state'

const P = 'auto.components.request.plan.PlanStates.'

type Props = {
  state: Exclude<PlanViewState, 'ready'>
  /** Label of the request's current stage, shown while no Plan is expected yet. */
  currentStepLabel?: string
  canGenerate: boolean
  generating: boolean
  /** Inline failure of the last generate attempt. */
  generateError?: string | null
  onGenerate: () => void
  onRetry: () => void
  /** RequestRpcErrorKind of a failed load. */
  loadErrorKind?: string | null
}

export function PlanStates({
  state,
  currentStepLabel,
  canGenerate,
  generating,
  generateError,
  onGenerate,
  onRetry,
  loadErrorKind
}: Props): React.JSX.Element {
  if (state === 'unsupported') {
    return <RequestUnsupportedNotice />
  }

  const generateButton = canGenerate && (
    <Button size="sm" disabled={generating} onClick={onGenerate} data-testid="plan-generate">
      {generating ? (
        <Loader2 className="size-3.5 animate-spin" aria-hidden />
      ) : (
        <Sparkles className="size-3.5" aria-hidden />
      )}
      {translate(`${P}generate`, 'Generate plan')}
    </Button>
  )
  const generateFailure = generateError && (
    <p role="alert" className="text-xs text-destructive" data-testid="plan-generate-error">
      {generateError}
    </p>
  )

  if (state === 'loading') {
    return (
      <div className="flex flex-col gap-2 p-4" aria-busy="true" data-testid="plan-loading">
        <Skeleton className="h-5 w-1/2" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    )
  }

  if (state === 'generating') {
    return (
      <div
        className="flex flex-col items-center gap-3 p-6 text-center"
        data-testid="plan-generating"
      >
        <div className="flex w-full max-w-md flex-col gap-2" aria-busy="true">
          <Skeleton className="h-5 w-1/2" />
          <Skeleton className="h-14 w-full" />
        </div>
        <p className="text-sm text-muted-foreground">
          {translate(`${P}generating`, 'AI is drafting the plan...')}
        </p>
        {generateButton}
        {generateFailure}
      </div>
    )
  }

  if (state === 'error') {
    return (
      <div
        role="alert"
        className="flex flex-col items-center gap-2 p-6 text-center"
        data-testid="plan-error"
      >
        <AlertTriangle className="size-6 text-destructive" aria-hidden />
        <p className="text-sm text-destructive">{requestErrorMessage(loadErrorKind)}</p>
        <Button size="sm" variant="outline" onClick={onRetry}>
          {translate(`${P}retry`, 'Try again')}
        </Button>
      </div>
    )
  }

  const body = {
    forbidden: {
      icon: Lock,
      text: translate(
        `${P}forbidden`,
        'You do not have permission to view the tasks of this request.'
      )
    },
    not_yet: { icon: ClipboardList, text: translate(`${P}notYet`, 'No plan yet.') },
    not_found: { icon: ClipboardList, text: translate(`${P}notFound`, 'Plan not found.') },
    empty: { icon: ClipboardList, text: translate(`${P}empty`, 'The plan is empty.') }
  }[state]
  const Icon = body.icon

  return (
    <div
      className="flex flex-col items-center gap-2 p-6 text-center"
      data-testid={`plan-state-${state}`}
    >
      <Icon className="size-8 text-muted-foreground" aria-hidden />
      <p className="text-sm text-foreground">{body.text}</p>
      {state === 'not_yet' && currentStepLabel && (
        <p className="text-xs text-muted-foreground" data-testid="plan-current-step">
          {translate(`${P}currentStep`, 'Current step: {{step}}', { step: currentStepLabel })}
        </p>
      )}
      {(state === 'empty' || state === 'not_found') && generateButton}
      {generateFailure}
    </div>
  )
}
