/**
 * Stage timeline steps — CR-REQ-019-01, CR-REQ-037 2.3
 *
 * The steps a request goes through and the step each status belongs to. Split from the model so that adding a row (the Outcome
 * Review, CR-OR-020) does not push either file past the line limit.
 *
 * @module components/request/request-stage-timeline-steps
 */

import type { ExecutionSummary, RequestFlowView } from '../../../../shared/request-execution-types'
import type {
  RequestEngine,
  RequestStatus,
  ReturnedFromStage
} from '../../../../shared/request-types'

export type StepId = 'classification' | 'analysis' | 'plan' | 'execution' | 'handover' | 'review'

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
  | { kind: 'review'; row: ReviewRowCaption; openDecisions?: number }

/** What the Outcome Review row says: the phase of the review, or that it is out of date, approved, or waiting for decisions. */
export type ReviewRowCaption =
  | 'none'
  | 'design'
  | 'delivered'
  | 'stale'
  | 'approved'
  | 'decisions'
  | 'unknown'

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

export const STEP_LABEL = 'auto.components.request.StageTimeline.step.'

export function stepsOf(flow: RequestFlowView, engine: RequestEngine | undefined): Step[] {
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

export function stepIdForStatus(status: RequestStatus, has: (id: StepId) => boolean): StepId {
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

export function stepIdForReturnedStage(
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

export function stepIdForResume(resume: string | undefined, has: (id: StepId) => boolean): StepId {
  switch (resume) {
    case 'planning':
      return has('plan') ? 'plan' : 'execution'
    case 'executing':
      return 'execution'
    default:
      return has('analysis') ? 'analysis' : 'classification'
  }
}

export const RUNNING_STATUSES: ReadonlySet<RequestStatus> = new Set([
  'submitted',
  'classifying',
  'analyzing',
  'planning',
  'executing'
])

export function progressCaption(
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
