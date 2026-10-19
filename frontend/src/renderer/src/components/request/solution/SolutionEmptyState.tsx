/**
 * SolutionEmptyState — CR-REQ-020-02
 *
 * What the Analysis tab shows while the Request has no Solution yet: a failure with its fix, the start prompt, the
 * "AI is working" state, or plain "No analysis yet".
 *
 * @module components/request/solution/SolutionEmptyState
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { AnalysisRun, RequestStatus } from '../../../../../shared/request-types'
import { requestErrorMessageForError } from '../request-error-message'
import { SolutionGenerationState } from './SolutionGenerationState'
import { SolutionRunFailure } from './SolutionRunFailure'
import { SolutionStartPrompt } from './SolutionStartPrompt'
import type { SolutionDecision } from './useSolutionDecision'
import type { useSolutionStart } from './useSolutionStart'

export type SolutionEmptyStateProps = {
  requestStatus: RequestStatus
  runPending: boolean
  failedRun: AnalysisRun | null
  decision: SolutionDecision
  start: ReturnType<typeof useSolutionStart>
  cooldown: boolean
}

export function SolutionEmptyState({
  requestStatus,
  runPending,
  failedRun,
  decision,
  start,
  cooldown
}: SolutionEmptyStateProps): React.JSX.Element {
  const failure = decision.failure
  return (
    <div data-testid="solution-panel-empty" className="p-4">
      {failure || failedRun ? (
        <SolutionRunFailure
          code={failure ? failure.error.backendCode : failedRun?.errorCode}
          errorKind={failure?.error.kind}
          errorText={failure ? requestErrorMessageForError(failure.error) : null}
          onRegenerate={() => void decision.regenerate()}
          regenerateDisabled={decision.submitting || cooldown}
        />
      ) : start.failure ? (
        <SolutionRunFailure
          code={start.failure.backendCode}
          errorKind={start.failure.kind}
          errorText={requestErrorMessageForError(start.failure)}
          onRegenerate={() => void start.start()}
          regenerateDisabled={start.submitting || cooldown}
        />
      ) : requestStatus === 'analyzing' ? (
        runPending || start.started ? (
          <SolutionGenerationState variant="analyzing" />
        ) : (
          <SolutionStartPrompt
            onStart={() => void start.start()}
            disabled={start.submitting || cooldown}
            submitting={start.submitting}
          />
        )
      ) : (
        <p className="text-sm text-muted-foreground">
          {translate('auto.components.request.SolutionPanel.empty', 'No analysis yet.')}
        </p>
      )}
    </div>
  )
}
