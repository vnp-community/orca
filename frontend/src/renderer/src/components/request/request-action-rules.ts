/**
 * request-action-rules.ts — CR-REQ-019-03/05
 *
 * Which header actions a request offers in each status. The backend stays the
 * authority (REQUEST_TRANSITION_NOT_ALLOWED etc.); these rules only decide
 * which buttons are worth showing.
 *
 * @module components/request/request-action-rules
 */

import { CHILD_REQUEST_RULES } from './request-stage-timeline-model'
import { REQUEST_FLOW_REGISTRY } from '../../../../shared/request-flow-registry'
import type { OrcaRequest, RequestStatus, ReturnedFromStage } from '../../../../shared/request-types'

const RETURNABLE_STATUSES: RequestStatus[] = [
  'analyzing', 'awaiting_analysis_approval', 'planning', 'awaiting_plan_approval', 'executing'
]

const PRE_CONFIRMATION_STATUSES: RequestStatus[] = ['submitted', 'classifying', 'awaiting_type_confirmation', 'unknown']

export const RETURN_STAGES: Exclude<ReturnedFromStage, 'unknown'>[] = [
  'classification', 'analysis', 'plan', 'phase', 'task'
]

const STATUS_TO_RETURN_STAGE: Partial<Record<RequestStatus, Exclude<ReturnedFromStage, 'unknown'>>> = {
  analyzing: 'analysis',
  awaiting_analysis_approval: 'analysis',
  planning: 'plan',
  awaiting_plan_approval: 'plan',
  executing: 'task'
}

export function defaultReturnStage(status: RequestStatus): Exclude<ReturnedFromStage, 'unknown'> {
  return STATUS_TO_RETURN_STAGE[status] ?? 'analysis'
}

export type RequestActionAvailability = {
  canCancel: boolean
  canReopen: boolean
  canReturnToBacklog: boolean
  canChangeType: boolean
  canSpawnChild: boolean
}

export function getRequestActionAvailability(request: Pick<OrcaRequest, 'status' | 'type'>): RequestActionAvailability {
  const { status, type } = request
  const terminal = status === 'completed' || status === 'cancelled'
  const childRule = type === 'unknown' ? undefined : CHILD_REQUEST_RULES[type]

  // Why: spike/question/hotfix hand off what the analysis found, so they need to be past it;
  // escalation types may spawn a sibling at any time after classification.
  const analysisHandoff = type === 'spike' || type === 'question' || type === 'hotfix'
  const pastAnalysis = !PRE_CONFIRMATION_STATUSES.includes(status) && status !== 'analyzing'

  return {
    canCancel: !terminal && status !== 'unknown',
    canReopen: status === 'request_backlog',
    canReturnToBacklog: RETURNABLE_STATUSES.includes(status),
    canChangeType: !terminal && !PRE_CONFIRMATION_STATUSES.includes(status),
    canSpawnChild:
      Boolean(childRule) &&
      status !== 'cancelled' &&
      !PRE_CONFIRMATION_STATUSES.includes(status) &&
      (analysisHandoff ? pastAnalysis : true)
  }
}

/** Detail tabs the registry says exist for this type; unknown type shows neither. */
export function getVisibleDetailTabs(type: OrcaRequest['type']): { analysis: boolean; plan: boolean } {
  if (type === 'unknown') {return { analysis: false, plan: false }}
  const entry = REQUEST_FLOW_REGISTRY[type]
  return { analysis: entry.analysisKind !== null, plan: entry.plan !== 'none' }
}
