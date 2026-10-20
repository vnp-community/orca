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
import { useRequestCooldown } from '../../../hooks/useRequestCooldown'
import { RequestAiSetupBanner } from '../RequestAiSetupBanner'
import type { AiSetupCode } from '../request-ai-setup'
import { requestErrorMessage } from '../request-error-message'
import { RequestUnsupportedNotice } from '../RequestUnsupportedNotice'
import { PlanAutoSkippedNote } from './PlanAutoSkippedNote'
import { PlanAutoWriting } from './PlanAutoWriting'
import { PlanRunFailedCard } from './PlanRunFailedCard'
import type { PlanAutoDisplay } from './plan-auto-display'
import type { PlanViewState } from './plan-view-state'

const P = 'auto.components.request.plan.PlanStates.'

const NO_AUTO_DISPLAY: PlanAutoDisplay = { kind: 'none' }

type Props = {
  state: Exclude<PlanViewState, 'ready'>
  /** Label of the request's current stage, shown while no Plan is expected yet. */
  currentStepLabel?: string
  canGenerate: boolean
  generating: boolean
  /** Inline failure of the last generate attempt. */
  generateError?: string | null
  /** Set when the last generate attempt failed because the dev server's AI cannot run. */
  generateAiCode?: AiSetupCode | null
  /** Backend code of that failure, to tell it apart from the one the failed-run card already shows. */
  generateErrorCode?: string | null
  onGenerate: () => void
  onRetry: () => void
  /** What the server is doing about the Plan (CR-REQ-044); absent means nothing to say. */
  autoDisplay?: PlanAutoDisplay
  /** Retry of a failed Plan run; separate from `onRetry`, which reloads the tab. */
  onRetryRun?: () => void
  retryingRun?: boolean
  retryRunError?: string | null
  /** RequestRpcErrorKind of a failed load. */
  loadErrorKind?: string | null
}

export function PlanStates({
  state,
  currentStepLabel,
  canGenerate,
  generating,
  generateError,
  generateAiCode,
  generateErrorCode,
  onGenerate,
  onRetry,
  autoDisplay = NO_AUTO_DISPLAY,
  onRetryRun,
  retryingRun = false,
  retryRunError,
  loadErrorKind
}: Props): React.JSX.Element {
  const aiCooldown = useRequestCooldown('ai')
  if (state === 'unsupported') {
    return <RequestUnsupportedNotice />
  }

  const writing = autoDisplay.kind === 'writing'
  const failed = autoDisplay.kind === 'failed'
  // Why: while Orca writes the Plan a second Generate would only compete with the run; after a failure it is the way around Retry.
  const generateButton = canGenerate && !writing && (
    <Button
      size="sm"
      variant={failed ? 'ghost' : 'default'}
      disabled={generating || aiCooldown.coolingDown}
      onClick={onGenerate}
      data-testid="plan-generate"
    >
      {generating ? (
        <Loader2 className="size-3.5 animate-spin" aria-hidden />
      ) : (
        <Sparkles className="size-3.5" aria-hidden />
      )}
      {failed
        ? translate(
            'auto.components.request.plan.PlanRunFailedCard.generateManually',
            'Generate manually'
          )
        : translate(`${P}generate`, 'Generate plan')}
    </Button>
  )
  // Why: a failed run already shows its own card; a second message for the same failure would only confuse, but a refusal to
  // even start (another code) still has to be seen.
  const sameAsCard = autoDisplay.kind === 'failed' && generateErrorCode === autoDisplay.code
  const generateFailure = sameAsCard ? null : generateAiCode ? (
    <RequestAiSetupBanner
      code={generateAiCode}
      onRetry={canGenerate ? onGenerate : undefined}
      retryDisabled={generating || aiCooldown.coolingDown}
    />
  ) : (
    generateError && (
      <p role="alert" className="text-xs text-destructive" data-testid="plan-generate-error">
        {generateError}
      </p>
    )
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
        {autoDisplay.kind === 'writing' && (
          <PlanAutoWriting
            attempt={autoDisplay.attempt}
            maxAttempts={autoDisplay.maxAttempts}
            waiting={autoDisplay.waiting}
            starting={autoDisplay.starting}
          />
        )}
        {autoDisplay.kind === 'failed' && (
          <PlanRunFailedCard
            code={autoDisplay.code}
            onRetry={onRetryRun ?? onGenerate}
            retrying={retryingRun}
            retryError={retryRunError}
          />
        )}
        {autoDisplay.kind === 'skipped' && (
          <PlanAutoSkippedNote reasonCode={autoDisplay.reasonCode} />
        )}
        {autoDisplay.kind === 'none' && (
          <>
            <div className="flex w-full max-w-md flex-col gap-2" aria-busy="true">
              <Skeleton className="h-5 w-1/2" />
              <Skeleton className="h-14 w-full" />
            </div>
            <p className="text-sm text-muted-foreground">
              {translate(`${P}generating`, 'AI is drafting the plan...')}
            </p>
          </>
        )}
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
      {state === 'not_found' && (
        <>
          <p className="text-xs text-muted-foreground" data-testid="plan-not-found-hint">
            {translate(
              `${P}notFoundHint`,
              'It may have been deleted, or you may not have access to its tasks.'
            )}
          </p>
          <Button size="sm" variant="outline" onClick={onRetry} data-testid="plan-not-found-retry">
            {translate(`${P}retry`, 'Try again')}
          </Button>
        </>
      )}
      {(state === 'empty' || state === 'not_found') && generateButton}
      {generateFailure}
    </div>
  )
}
