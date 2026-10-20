/**
 * PlanAutoSkippedNote — CR-REQ-044
 *
 * Neutral note for a Request whose Plan Orca could not start on its own; the Generate button next to it stays the way forward.
 *
 * @module components/request/plan/PlanAutoSkippedNote
 */

import React from 'react'
import { translate } from '@/i18n/i18n'

const P = 'auto.components.request.plan.PlanAutoSkippedNote.'

/** The reason in words for the codes a user can act on; any other code is shown as it is. */
function skipReason(code: string): string {
  switch (code) {
    case 'REQUEST_PLAN_AI_UNAVAILABLE':
      return translate(`${P}reason.noDevServer`, 'no dev server is connected')
    case 'REQUEST_ANALYSIS_NO_REPO_PATH':
      return translate(`${P}reason.noRepoPath`, 'the project has no repository path')
    case 'REQUEST_ENGINE_UNAVAILABLE':
      return translate(`${P}reason.engineUnavailable`, 'the planning engine is not available')
    default:
      return code
  }
}

export function PlanAutoSkippedNote({ reasonCode }: { reasonCode: string }): React.JSX.Element {
  return (
    <p className="max-w-md text-sm text-muted-foreground" data-testid="plan-auto-skipped">
      {translate(
        `${P}text`,
        'Orca could not start the plan on its own ({{reason}}). Generate it when you are ready.',
        { reason: skipReason(reasonCode) }
      )}
    </p>
  )
}
