/**
 * PlanRevisionSection — CR-REQ-045
 *
 * Everything the Plan tab says about changing a plan that already exists, in one place under the approval bar: the server's
 * rewrite or its failure, and the box where the reviewer asks for a change. It keeps RequestPlanTab to a single line for this.
 *
 * @module components/request/plan/revision/PlanRevisionSection
 */

import React from 'react'
import { PlanRunNotice } from '../PlanRunNotice'
import type { PlanAutoDisplay } from '../plan-auto-display'
import { PlanRevisionPanel } from './PlanRevisionPanel'
import type { PlanRevisionApi } from './usePlanRevision'

type Props = {
  revision: PlanRevisionApi
  display: PlanAutoDisplay
  /** The engine that wrote the plan; picks the suggestion chips. */
  engine: string | undefined
}

export function PlanRevisionSection({ revision, display, engine }: Props): React.JSX.Element {
  return (
    <>
      <PlanRunNotice
        display={display}
        onRetry={() => void revision.retry()}
        retrying={revision.retrying}
        retryError={revision.retryError}
        hideStrip={revision.visible}
        hideFailure={revision.busy}
      />
      {revision.visible && (
        <div className="border-b border-border px-4 py-3" data-testid="plan-revision-section">
          <PlanRevisionPanel revision={revision} engine={engine} />
        </div>
      )}
    </>
  )
}
