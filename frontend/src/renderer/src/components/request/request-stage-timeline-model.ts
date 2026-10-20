/**
 * Request Stage Timeline Model — CR-REQ-019-01, redesigned by CR-REQ-037 2.3
 *
 * Pure TypeScript — no React dependency. Builds the ordered list of stages a request goes through, from the flow of its
 * type (backend `request.flow`, or the built-in fallback) and the engine it runs on, so the screen never branches on
 * `type === 'hotfix'`. There is no "Phases" stage: Phases live inside the Plan (the outline) and Execution (the progress).
 *
 * @module components/request/request-stage-timeline-model
 */

import { LOW_CONFIDENCE_THRESHOLD } from '../../../../shared/request-flow-registry'
import { fallbackRequestFlow } from '../../../../shared/request-flow-view'
import type { ExecutionSummary, RequestFlowView } from '../../../../shared/request-execution-types'
import type {
  RequestType,
  RequestStatus,
  RequestSize,
  RequestEngine,
  ReturnedFromStage,
  RequestLinkReason
} from '../../../../shared/request-types'
import {
  RUNNING_STATUSES,
  progressCaption,
  stepIdForResume,
  stepIdForReturnedStage,
  stepIdForStatus,
  stepsOf
} from './request-stage-timeline-steps'
import { withReviewStep, type ReviewTimelineInput } from './request-stage-timeline-review'
import type { StageTimeline, StepId } from './request-stage-timeline-steps'

export type {
  ReviewRowCaption,
  StageTimeline,
  Step,
  StepCaption,
  StepId,
  StepState
} from './request-stage-timeline-steps'

// ---------------------------------------------------------------------------
// Child request rules
// ---------------------------------------------------------------------------

export type ChildRequestRule = {
  reason: RequestLinkReason
  suggestedTypes: RequestType[]
}

export const CHILD_REQUEST_RULES: Partial<
  Record<Exclude<RequestType, 'unknown'>, ChildRequestRule>
> = {
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
  if (confidence === undefined) {
    return true
  }
  return confidence < LOW_CONFIDENCE_THRESHOLD
}

// ---------------------------------------------------------------------------
// buildStageTimeline
// ---------------------------------------------------------------------------

export type BuildInput = {
  type: RequestType
  size?: RequestSize
  status: RequestStatus
  returnedFromStage?: ReturnedFromStage
  /** Clarification.resumeStatus: while awaiting_information, the step the request resumes at. */
  resumeStatus?: string
  /** The flow of the type; the built-in table answers when absent. */
  flow?: RequestFlowView | null
  engine?: RequestEngine
  summary?: ExecutionSummary | null
  /** The Outcome Review row (CR-OR-020); absent or disabled adds nothing. */
  review?: ReviewTimelineInput
}

function buildCoreTimeline(input: BuildInput): StageTimeline {
  const { type, size, status, returnedFromStage, resumeStatus, engine, summary } = input
  // Gracefully handle unknown type/status
  if (type === 'unknown' || status === 'unknown') {
    return { steps: [], current: null }
  }
  const flow = input.flow ?? fallbackRequestFlow(type, size)
  if (!flow) {
    return { steps: [], current: null }
  }

  const steps = stepsOf(flow, engine)
  const has = (id: StepId): boolean => steps.some((s) => s.id === id)
  const stopped = status === 'request_backlog' || status === 'cancelled'

  if (status === 'completed') {
    for (const step of steps) {
      step.state = 'done'
    }
    const handover = steps.find((s) => s.id === 'handover')
    if (handover) {
      const state = summary?.handover.state
      if (state === 'archiving') {
        handover.state = 'running'
        handover.caption = { kind: 'archiving' }
      } else if (state === 'failed') {
        handover.state = 'stopped'
        handover.caption = { kind: 'handover_failed' }
      }
    }
    const exec = steps.find((s) => s.id === 'execution')
    if (exec) {
      exec.caption = progressCaption(summary, false)
    }
    return { steps, current: handover && handover.state !== 'done' ? 'handover' : null }
  }

  // Why: without a recorded stage, marking one as stopped would be a guess; the status chip already says it was cancelled.
  if (status === 'cancelled' && !returnedFromStage) {
    return { steps, current: null }
  }

  let currentId: StepId
  if (stopped) {
    currentId = stepIdForReturnedStage(returnedFromStage, has)
  } else if (status === 'awaiting_information') {
    currentId = stepIdForResume(resumeStatus, has)
  } else {
    currentId = stepIdForStatus(status, has)
  }
  if (!has(currentId)) {
    currentId = 'classification'
  }

  const currentIndex = steps.findIndex((s) => s.id === currentId)
  const needsYou = (summary?.needsYou.length ?? 0) > 0
  steps.forEach((step, i) => {
    if (i < currentIndex) {
      step.state = 'done'
    } else if (i === currentIndex) {
      step.state = stopped
        ? 'stopped'
        : RUNNING_STATUSES.has(status) && !(status === 'executing' && needsYou)
          ? 'running'
          : 'waiting'
    }
  })

  const current = steps[currentIndex]
  if (current.state === 'waiting') {
    if (status === 'executing') {
      current.caption = { kind: 'waiting_you' }
    } else if (
      status === 'awaiting_plan_approval' ||
      status === 'awaiting_analysis_approval' ||
      status === 'awaiting_type_confirmation'
    ) {
      current.caption = { kind: 'waiting_approval' }
    } else {
      current.caption = { kind: 'waiting_you' }
    }
  }
  if (current.id === 'execution' && (status === 'executing' || stopped)) {
    current.caption = progressCaption(summary, stopped) ?? current.caption
  }

  return { steps, current: current.id }
}

export function buildStageTimeline(input: BuildInput): StageTimeline {
  return withReviewStep(buildCoreTimeline(input), input.review)
}
