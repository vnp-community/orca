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

// ---------------------------------------------------------------------------
// Step types
// ---------------------------------------------------------------------------

export type StepId = 'classification' | 'analysis' | 'plan' | 'execution' | 'handover'

/**
 * done: finished. running: Orca or an agent is working on it. waiting: a person has to act. stopped: the request left the
 * flow here (back to the backlog, or cancelled). pending: not reached.
 */
export type StepState = 'done' | 'running' | 'waiting' | 'stopped' | 'pending'

/** A short line under a step; the component translates it. */
export type StepCaption =
  | { kind: 'progress'; phase?: number; phases?: number; tasksDone: number; tasksTotal: number }
  | { kind: 'stopped_progress'; tasksDone: number; tasksTotal: number }
  | { kind: 'waiting_approval' }
  | { kind: 'waiting_you' }
  | { kind: 'archiving' }
  | { kind: 'handover_failed' }

export type Step = {
  id: StepId
  labelKey: string
  state: StepState
  /** analysisKind ('solution'|'diagnosis'|...) for the analysis step, planKind for the plan step */
  variant?: string
  caption?: StepCaption
}

export type StageTimeline = {
  steps: Step[]
  current: StepId | null
}

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
}

const STEP_LABEL = 'auto.components.request.StageTimeline.step.'

function stepsOf(flow: RequestFlowView, engine: RequestEngine | undefined): Step[] {
  const steps: Step[] = [
    { id: 'classification', labelKey: `${STEP_LABEL}classification`, state: 'pending' }
  ]
  if (flow.analysisKind !== null) {
    steps.push({
      id: 'analysis',
      labelKey: `${STEP_LABEL}analysis`,
      state: 'pending',
      variant: flow.analysisKind
    })
  }
  if (flow.planKind === 'plan' || flow.planKind === 'task_list') {
    steps.push({
      id: 'plan',
      labelKey: `${STEP_LABEL}${flow.planKind === 'task_list' ? 'taskList' : 'plan'}`,
      state: 'pending',
      variant: flow.planKind
    })
  }
  if (flow.planKind !== 'none') {
    steps.push({ id: 'execution', labelKey: `${STEP_LABEL}execution`, state: 'pending' })
    if (engine === 'openspec') {
      steps.push({ id: 'handover', labelKey: `${STEP_LABEL}handover`, state: 'pending' })
    }
  }
  return steps
}

function stepIdForStatus(status: RequestStatus, has: (id: StepId) => boolean): StepId {
  switch (status) {
    case 'submitted':
    case 'classifying':
    case 'awaiting_type_confirmation':
      return 'classification'
    case 'analyzing':
    case 'awaiting_analysis_approval':
      return 'analysis'
    case 'planning':
    case 'awaiting_plan_approval':
      // A single-task flow (hotfix) has no Plan stage: its pre-deploy approval holds the Execution stage.
      return has('plan') ? 'plan' : 'execution'
    case 'executing':
    case 'completed':
      return 'execution'
    default:
      return 'classification'
  }
}

function stepIdForReturnedStage(
  stage: ReturnedFromStage | undefined,
  has: (id: StepId) => boolean
): StepId {
  switch (stage) {
    case 'analysis':
      return has('analysis') ? 'analysis' : 'classification'
    case 'plan':
      return has('plan') ? 'plan' : has('execution') ? 'execution' : 'classification'
    case 'phase':
    case 'task':
      return has('execution') ? 'execution' : 'classification'
    default:
      return 'classification'
  }
}

function stepIdForResume(resume: string | undefined, has: (id: StepId) => boolean): StepId {
  switch (resume) {
    case 'planning':
      return has('plan') ? 'plan' : 'execution'
    case 'executing':
      return 'execution'
    default:
      return has('analysis') ? 'analysis' : 'classification'
  }
}

const RUNNING_STATUSES: ReadonlySet<RequestStatus> = new Set([
  'submitted',
  'classifying',
  'analyzing',
  'planning',
  'executing'
])

function progressCaption(
  summary: ExecutionSummary | null | undefined,
  stopped: boolean
): StepCaption | undefined {
  if (!summary || summary.progress.tasksTotal === 0) {
    return undefined
  }
  const { tasksDone, tasksTotal, phasesTotal, phasesDone } = summary.progress
  if (stopped) {
    return { kind: 'stopped_progress', tasksDone, tasksTotal }
  }
  if (phasesTotal > 0) {
    const current = Math.min(phasesTotal, phasesDone + (phasesDone < phasesTotal ? 1 : 0))
    return { kind: 'progress', phase: current, phases: phasesTotal, tasksDone, tasksTotal }
  }
  return { kind: 'progress', tasksDone, tasksTotal }
}

export function buildStageTimeline(input: BuildInput): StageTimeline {
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
