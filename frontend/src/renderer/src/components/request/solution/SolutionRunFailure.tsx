/**
 * SolutionRunFailure — TASK-FE-E2E-005
 *
 * Why the analysis is missing: the AI run (or the generate call) failed. An AI setup problem
 * gets the banner with the fix; anything else gets the generic failure with its code.
 *
 * @module components/request/solution/SolutionRunFailure
 */

import React from 'react'
import { RequestAiSetupBanner } from '../RequestAiSetupBanner'
import { firstAiSetupCode } from '../request-ai-setup'
import { SolutionGenerationState } from './SolutionGenerationState'

type Props = {
  /** Backend code of the failure: a run's errorCode or the failed call's backendCode. */
  code?: string | null
  /** Kind of a failed call; absent for a failed run. */
  errorKind?: string | null
  errorText?: string | null
  onRegenerate: () => void
  regenerateDisabled: boolean
}

export function SolutionRunFailure({
  code,
  errorKind,
  errorText,
  onRegenerate,
  regenerateDisabled
}: Props): React.JSX.Element {
  const aiCode = firstAiSetupCode(code)
  if (aiCode) {
    return (
      <RequestAiSetupBanner
        code={aiCode}
        onRetry={onRegenerate}
        retryDisabled={regenerateDisabled}
      />
    )
  }
  return (
    <SolutionGenerationState
      variant="failed"
      errorKind={errorKind}
      errorCode={code}
      errorText={errorText}
      onRegenerate={onRegenerate}
      regenerateDisabled={regenerateDisabled}
    />
  )
}
