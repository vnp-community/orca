/**
 * plan-view-state.ts — CR-REQ-021-05
 *
 * Pure decision of what the Plan tab shows. "Missing" states are never errors: the
 * backend may simply not have produced a Plan yet.
 *
 * @module components/request/plan/plan-view-state
 */

import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { OrcaRequest, RequestStatus } from '../../../../../shared/request-types'

export type PlanViewState =
  | 'unsupported'
  | 'forbidden'
  | 'error'
  | 'loading'
  | 'not_yet'
  | 'generating'
  | 'not_found'
  | 'empty'
  | 'ready'

/** Statuses before a Plan can exist. */
const BEFORE_PLAN: ReadonlySet<RequestStatus> = new Set([
  'submitted',
  'classifying',
  'awaiting_type_confirmation',
  'analyzing',
  'awaiting_analysis_approval',
  'awaiting_information',
  'request_backlog'
])

export type PlanViewInput = {
  request: Pick<OrcaRequest, 'status' | 'planTaskId'>
  tree: PlanSubtree | null
  isLoading: boolean
  /** RequestRpcErrorKind of the failed load. */
  error: string | null
  flowSupported: boolean
}

export function derivePlanViewState({
  request,
  tree,
  isLoading,
  error,
  flowSupported
}: PlanViewInput): PlanViewState {
  if (!flowSupported || error === 'unsupported') {
    return 'unsupported'
  }
  if (error === 'forbidden') {
    return 'forbidden'
  }
  const hasPlan = !!tree?.plan
  if (error && !hasPlan) {
    return 'error'
  }
  if (!request.planTaskId) {
    if (request.status === 'planning') {
      return 'generating'
    }
    return BEFORE_PLAN.has(request.status) ? 'not_yet' : 'not_found'
  }
  if (isLoading && !hasPlan) {
    return 'loading'
  }
  if (!hasPlan) {
    return request.status === 'planning' ? 'generating' : 'not_found'
  }
  if (tree && tree.phases.length === 0 && tree.flatTasks.length === 0) {
    return 'empty'
  }
  return 'ready'
}

/** Whether a manual "generate / regenerate" action makes sense in this request status. */
export function canGeneratePlan(status: RequestStatus): boolean {
  return status === 'planning' || status === 'awaiting_plan_approval'
}
