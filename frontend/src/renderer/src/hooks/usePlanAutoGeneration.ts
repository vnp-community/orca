/**
 * usePlanAutoGeneration — CR-REQ-044
 *
 * Everything the Plan tab needs to follow a Plan the server writes (or failed to write): the polled run state, what to show for
 * it, and the Retry. The pieces stay separate hooks so other views of the Plan run can pick only what they need.
 *
 * @module hooks/usePlanAutoGeneration
 */

import { useMemo } from 'react'
import { useAppStore } from '../store'
import { supportsPlanAuto } from '../store/slices/request'
import {
  derivePlanAutoDisplay,
  type PlanAutoDisplay
} from '../components/request/plan/plan-auto-display'
import { usePlanRetry, type PlanRetry } from './usePlanRetry'
import { usePlanRunState } from './usePlanRunState'
import type { PlanRunView } from '../../../shared/request-plan-auto'
import type { PlanDecisionOutcome } from './usePlanDecision'
import type { OrcaRequest } from '../../../shared/request-types'

export type PlanAutoGeneration = {
  display: PlanAutoDisplay
  /** The latest Plan run as the server reports it; null when there is none (CR-REQ-045 tells a new run's failure from an old one's). */
  run: PlanRunView | null
  /** Read the run state now instead of at the next poll. */
  refresh: () => void
  /** The server is writing (or about to save) the Plan. */
  busy: boolean
  retry: PlanRetry
}

export function usePlanAutoGeneration(
  request: Pick<OrcaRequest, 'id' | 'status' | 'planTaskId'>,
  onFinished: () => void,
  regenerateManually: () => Promise<PlanDecisionOutcome>
): PlanAutoGeneration {
  const planAuto = useAppStore(supportsPlanAuto)
  const { view, busy, graceElapsed, refresh } = usePlanRunState(request, onFinished)
  const display = useMemo(
    () =>
      derivePlanAutoDisplay({
        request: { status: request.status, planTaskId: request.planTaskId },
        run: view,
        supportsPlanAuto: planAuto,
        graceElapsed
      }),
    [request.status, request.planTaskId, view, planAuto, graceElapsed]
  )
  const retry = usePlanRetry({ request, run: view, regenerateManually, refresh })
  return { display, run: view, refresh, busy, retry }
}
