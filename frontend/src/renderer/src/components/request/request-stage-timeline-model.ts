/**
 * Request Stage Timeline Model — CR-REQ-019-01
 *
 * Pure TypeScript — no React dependency. Builds the ordered list of
 * pipeline steps for a given request so the UI can render a timeline
 * without branching on `type === 'hotfix'` directly. All decisions
 * are delegated to REQUEST_FLOW_REGISTRY (018-01).
 *
 * @module components/request/request-stage-timeline-model
 */

import {
  REQUEST_FLOW_REGISTRY,
  flowHasPhase,
  LOW_CONFIDENCE_THRESHOLD
} from '../../../../shared/request-flow-registry'
import type { RequestType, RequestStatus, RequestSize, ReturnedFromStage, RequestLinkReason } from '../../../../shared/request-types'

// ---------------------------------------------------------------------------
// Step types
// ---------------------------------------------------------------------------

export type StepId = 'classification' | 'analysis' | 'plan' | 'phase' | 'execution'
export type StepState = 'done' | 'current' | 'pending' | 'skipped'

export type Step = {
  id: StepId
  labelKey: string
  state: StepState
  /** analysisKind ('solution'|'diagnosis'|...) for analysis step, planKind for plan step */
  variant?: string
}

export type StageTimeline = {
  steps: Step[]
  current: StepId | null
  /** Set for hotfix requests: 'hotfix_fast_diagnosis' */
  note?: string
}

// ---------------------------------------------------------------------------
// Child request rules
// ---------------------------------------------------------------------------

export type ChildRequestRule = {
  reason: RequestLinkReason
  suggestedTypes: RequestType[]
}

export const CHILD_REQUEST_RULES: Partial<Record<Exclude<RequestType, 'unknown'>, ChildRequestRule>> = {
  spike: { reason: 'spawned_by_spike', suggestedTypes: ['task', 'change_request', 'refactor'] },
  question: { reason: 'spawned_by_question', suggestedTypes: ['task', 'docs'] },
  hotfix: { reason: 'followup_hotfix', suggestedTypes: ['bug', 'task'] },
  bug: { reason: 'escalation', suggestedTypes: ['change_request', 'security'] },
  security: { reason: 'escalation', suggestedTypes: ['change_request'] },
  change_request: { reason: 'escalation', suggestedTypes: ['bug', 'security'] }
}

// ---------------------------------------------------------------------------
// isLowConfidence
// ---------------------------------------------------------------------------

export function isLowConfidence(confidence: number | undefined): boolean {
  if (confidence === undefined) return true
  return confidence < LOW_CONFIDENCE_THRESHOLD
}

// ---------------------------------------------------------------------------
// buildStageTimeline
// ---------------------------------------------------------------------------

type BuildInput = {
  type: RequestType
  size?: RequestSize
  status: RequestStatus
  returnedFromStage?: ReturnedFromStage
}

// Map: which status is 'current' for which step
const STATUS_TO_CURRENT_STEP: Partial<Record<RequestStatus, StepId>> = {
  submitted: 'classification',
  classifying: 'classification',
  awaiting_type_confirmation: 'classification',
  analyzing: 'analysis',
  awaiting_analysis_approval: 'analysis',
  awaiting_information: 'analysis',
  planning: 'plan',
  awaiting_plan_approval: 'plan',
  executing: 'execution',
  completed: 'execution'
}

// Steps that come before request_backlog based on where it was returned from
const RETURNED_STAGE_TO_STEP: Partial<Record<ReturnedFromStage, StepId>> = {
  classification: 'classification',
  analysis: 'analysis',
  plan: 'plan',
  phase: 'phase',
  task: 'execution'
}

export function buildStageTimeline({ type, size, status, returnedFromStage }: BuildInput): StageTimeline {
  // Gracefully handle unknown type/status
  if (type === 'unknown' || status === 'unknown') {
    return { steps: [], current: null }
  }

  const entry = REQUEST_FLOW_REGISTRY[type]
  const includeAnalysis = entry.analysisKind !== null
  const includePlan = entry.plan !== 'none'
  const includePhase = flowHasPhase(type, size ?? 'M')

  // Determine the current step
  let currentStepId: StepId | null = null

  if (status === 'request_backlog') {
    // Show current at the step it was returned from
    const returnedStep = returnedFromStage ? RETURNED_STAGE_TO_STEP[returnedFromStage] : null
    currentStepId = returnedStep ?? 'classification'
  } else if (status === 'cancelled') {
    currentStepId = STATUS_TO_CURRENT_STEP[status] ?? null
  } else {
    currentStepId = STATUS_TO_CURRENT_STEP[status] ?? null
  }

  // Build ordered steps
  const stepOrder: StepId[] = ['classification']
  if (includeAnalysis) stepOrder.push('analysis')
  if (includePlan) stepOrder.push('plan')
  if (includePhase) stepOrder.push('phase')
  stepOrder.push('execution')

  const currentIndex = currentStepId !== null ? stepOrder.indexOf(currentStepId) : -1

  const steps: Step[] = stepOrder.map((id, i) => {
    let state: StepState
    if (currentIndex === -1) {
      state = 'pending'
    } else if (i < currentIndex) {
      state = 'done'
    } else if (i === currentIndex) {
      state = status === 'cancelled' ? 'skipped' : 'current'
    } else {
      state = 'pending'
    }

    const step: Step = {
      id,
      labelKey: `auto.components.request.StageTimeline.step.${id}`,
      state
    }

    // Add variant info
    if (id === 'analysis' && includeAnalysis) {
      step.variant = entry.analysisKind ?? undefined
    }
    if (id === 'plan' && includePlan) {
      step.variant = entry.plan !== 'none' ? entry.plan : undefined
    }

    return step
  })

  const result: StageTimeline = { steps, current: currentStepId }

  // Hotfix fast-path note
  if (type === 'hotfix') {
    result.note = 'hotfix_fast_diagnosis'
  }

  return result
}
