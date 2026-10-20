/**
 * PlanRunFailedCard — CR-REQ-044
 *
 * The failure of the latest Plan run, read from the server so it survives a reload and shows in every tab. Props are generic
 * on purpose: any view that offers to try the Plan run again can show it.
 *
 * @module components/request/plan/PlanRunFailedCard
 */

import React from 'react'
import { AlertTriangle, Loader2, RotateCw } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { useRequestCooldown } from '../../../hooks/useRequestCooldown'
import { RequestAiSetupBanner } from '../RequestAiSetupBanner'
import { firstAiSetupCode } from '../request-ai-setup'

const P = 'auto.components.request.plan.PlanRunFailedCard.'

const TIMEOUT_CODES: ReadonlySet<string> = new Set([
  'REQUEST_AI_COMPLETE_TIMEOUT',
  'REQUEST_OPENSPEC_TIMEOUT'
])
const INVALID_OUTPUT_CODES: ReadonlySet<string> = new Set([
  'REQUEST_PLAN_AI_INVALID_JSON',
  'REQUEST_OPENSPEC_INVALID_OUTPUT'
])

/** One line of advice per family of failure; the exact code is always shown beside it. */
function failureHint(code: string): string {
  if (TIMEOUT_CODES.has(code)) {
    return translate(`${P}hint.timeout`, 'The AI took too long to answer.')
  }
  if (code === 'REQUEST_SOLUTION_RUN_INTERRUPTED') {
    return translate(`${P}hint.interrupted`, 'The run was interrupted before it finished.')
  }
  if (INVALID_OUTPUT_CODES.has(code)) {
    return translate(
      `${P}hint.invalidOutput`,
      'The AI answered in a form that is not a valid plan.'
    )
  }
  if (code === 'REQUEST_ANALYSIS_BUSY') {
    return translate(`${P}hint.busy`, 'The AI is busy with other work.')
  }
  if (code === 'REQUEST_PLAN_COMMIT_FAILED') {
    return translate(`${P}hint.commit`, 'The plan was written but could not be saved.')
  }
  // CR-REQ-045: the two refusals a change request can end with.
  if (code === 'REQUEST_PLAN_REVISION_NO_CHANGE') {
    return translate(
      `${P}hint.noChange`,
      'The agent did not change any document. Say more precisely what to change.'
    )
  }
  if (code === 'REQUEST_BASE_REF_NOT_FOUND') {
    return translate(
      `${P}hint.baseRefGone`,
      'The branch this request starts from no longer exists.'
    )
  }
  return translate(`${P}hint.generic`, 'Something went wrong while writing the plan.')
}

type Props = {
  /** Backend code of the failure, shown as it is. */
  code: string
  /** `revise`: a change request on a plan that exists failed, so the words are about the revision and the reviewer may write again. */
  variant?: 'generate' | 'revise'
  onRetry: () => void
  retrying?: boolean
  /** Message of the last failed retry. */
  retryError?: string | null
}

export function PlanRunFailedCard({
  code,
  variant = 'generate',
  onRetry,
  retrying = false,
  retryError
}: Props): React.JSX.Element {
  const aiCooldown = useRequestCooldown('ai')
  const aiSetupCode = firstAiSetupCode(code)
  return (
    <div
      role="alert"
      className="flex w-full max-w-md flex-col gap-2 rounded-md border border-destructive/30 px-3 py-2 text-left"
      data-testid="plan-run-failed"
      data-error-code={code}
    >
      <p className="flex items-center gap-2 text-sm text-destructive">
        <AlertTriangle className="size-4 shrink-0" aria-hidden />
        {variant === 'revise'
          ? translate(`${P}titleRevise`, 'The plan could not be revised ({{code}})', { code })
          : translate(`${P}title`, 'Plan generation failed ({{code}})', { code })}
      </p>
      {aiSetupCode ? (
        <RequestAiSetupBanner code={aiSetupCode} />
      ) : (
        <p className="text-xs text-muted-foreground" data-testid="plan-run-failed-hint">
          {failureHint(code)}
        </p>
      )}
      <div>
        <Button
          size="sm"
          variant="outline"
          disabled={retrying || aiCooldown.coolingDown}
          onClick={onRetry}
          data-testid="plan-run-retry"
        >
          {retrying ? (
            <Loader2 className="size-3.5 animate-spin" aria-hidden />
          ) : (
            <RotateCw className="size-3.5" aria-hidden />
          )}
          {variant === 'revise'
            ? translate(`${P}tryAgain`, 'Try again')
            : translate(`${P}retry`, 'Retry')}
        </Button>
      </div>
      {variant === 'revise' && (
        <p className="text-xs text-muted-foreground" data-testid="plan-run-failed-write-new">
          {translate(`${P}writeNew`, 'Or write a new request below.')}
        </p>
      )}
      {retryError && (
        <p role="alert" className="text-xs text-destructive" data-testid="plan-run-retry-error">
          {retryError}
        </p>
      )}
    </div>
  )
}
