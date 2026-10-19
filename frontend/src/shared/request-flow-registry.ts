/**
 * Request Flow Registry — v6 (CR-REQ-018)
 *
 * Fallback mapping of RequestType → analysis/plan/phase/gate behaviour. It mirrors
 * request-service `domain.FlowFor` (README v6 §3.4; request-flow-registry.fixture.json
 * is generated from the backend and tested against this table). Screens read the flow
 * from `request.flow` (see request-flow-view.ts) and use this table only when the
 * gateway has no such channel. UI logic never branches on `type === 'hotfix'` etc.
 *
 * @module shared/request-flow-registry
 */

import type { RequestType, RequestSize } from './request-types'

// ---------------------------------------------------------------------------
// Registry types
// ---------------------------------------------------------------------------

export type AnalysisKind = 'solution' | 'diagnosis' | 'findings' | 'answer' | null

export type PlanKind = 'plan' | 'task_list' | 'single_task' | 'none'

export type PhaseRule = 'always' | 'size_l' | 'never'

/** The approval that occupies awaiting_plan_approval; null when the flow starts without one. */
export type StartGate = 'plan' | 'task_list' | 'pre_deploy' | null

/** Gates that block inside `executing` without changing the request status. */
export type ExecutionGate = 'phase' | 'pre_deploy'

export type OpenSpecProfile = 'full' | 'light' | 'none'

export type FlowGates = {
  /** Whether an approval gate exists before analysis result is acted on */
  analysisApproval: boolean
  /** Whether a plan or task-list approval starts the run */
  planApproval: boolean
  /** Whether a pre-deploy approval exists (as the start gate or inside executing) */
  preDeployApproval: boolean
}

export type FlowEntry = {
  analysisKind: AnalysisKind
  /** Approval that ends the analysis; null when analysis needs none (hotfix) or does not exist. */
  analysisGate: 'solution' | 'findings' | 'answer' | null
  plan: PlanKind
  phase: PhaseRule
  startGate: StartGate
  executionGates: readonly ExecutionGate[]
  /** question and spike end after analysis: there is no plan and nothing to execute. */
  completesAfterAnalysis: boolean
  humanConfirmRequired: boolean
  /** Whether this type can run on the OpenSpec engine at all; the engine a Request really runs on is `OrcaRequest.engine`. */
  openSpecProfile: OpenSpecProfile
  /** Derived from the fields above; kept for the screens that read the three yes/no questions. */
  gates: FlowGates
}

type FlowEntryInput = Omit<FlowEntry, 'gates'>

function entry(input: FlowEntryInput): FlowEntry {
  return {
    ...input,
    gates: {
      analysisApproval: input.analysisGate !== null,
      planApproval: input.startGate === 'plan' || input.startGate === 'task_list',
      preDeployApproval:
        input.startGate === 'pre_deploy' || input.executionGates.includes('pre_deploy')
    }
  }
}

// ---------------------------------------------------------------------------
// Registry — mirrors request-service `domain.FlowFor` (the fixture
// request-flow-registry.fixture.json is generated from it and a test compares the two).
// It is only the fallback: screens read the flow from `request.flow` when the gateway has it.
// ---------------------------------------------------------------------------

export const REQUEST_FLOW_REGISTRY: Readonly<Record<Exclude<RequestType, 'unknown'>, FlowEntry>> = {
  bug: entry({
    analysisKind: 'diagnosis',
    analysisGate: 'solution',
    plan: 'plan',
    phase: 'size_l',
    startGate: 'plan',
    executionGates: [],
    completesAfterAnalysis: false,
    humanConfirmRequired: false,
    openSpecProfile: 'light'
  }),
  task: entry({
    analysisKind: null,
    analysisGate: null,
    plan: 'task_list',
    phase: 'never',
    startGate: 'task_list',
    executionGates: [],
    completesAfterAnalysis: false,
    humanConfirmRequired: false,
    openSpecProfile: 'none'
  }),
  docs: entry({
    analysisKind: null,
    analysisGate: null,
    plan: 'task_list',
    phase: 'never',
    startGate: 'task_list',
    executionGates: [],
    completesAfterAnalysis: false,
    humanConfirmRequired: false,
    openSpecProfile: 'none'
  }),
  question: entry({
    analysisKind: 'answer',
    analysisGate: 'answer',
    plan: 'none',
    phase: 'never',
    startGate: null,
    executionGates: [],
    completesAfterAnalysis: true,
    humanConfirmRequired: false,
    openSpecProfile: 'none'
  }),
  hotfix: entry({
    analysisKind: 'diagnosis',
    analysisGate: null,
    plan: 'single_task',
    phase: 'never',
    startGate: 'pre_deploy',
    executionGates: [],
    completesAfterAnalysis: false,
    humanConfirmRequired: true,
    openSpecProfile: 'none'
  }),
  security: entry({
    analysisKind: 'diagnosis',
    analysisGate: 'solution',
    plan: 'plan',
    phase: 'never',
    startGate: 'pre_deploy',
    executionGates: [],
    completesAfterAnalysis: false,
    humanConfirmRequired: true,
    openSpecProfile: 'light'
  }),
  ops_request: entry({
    analysisKind: null,
    analysisGate: null,
    plan: 'plan',
    phase: 'never',
    startGate: 'plan',
    executionGates: ['pre_deploy'],
    completesAfterAnalysis: false,
    humanConfirmRequired: false,
    openSpecProfile: 'none'
  }),
  change_request: entry({
    analysisKind: 'solution',
    analysisGate: 'solution',
    plan: 'plan',
    phase: 'always',
    startGate: 'plan',
    executionGates: ['phase'],
    completesAfterAnalysis: false,
    humanConfirmRequired: false,
    openSpecProfile: 'full'
  }),
  refactor: entry({
    analysisKind: 'solution',
    analysisGate: 'solution',
    plan: 'plan',
    phase: 'size_l',
    startGate: 'plan',
    executionGates: [],
    completesAfterAnalysis: false,
    humanConfirmRequired: false,
    openSpecProfile: 'full'
  }),
  spike: entry({
    analysisKind: 'findings',
    analysisGate: 'findings',
    plan: 'none',
    phase: 'never',
    startGate: null,
    executionGates: [],
    completesAfterAnalysis: true,
    humanConfirmRequired: false,
    openSpecProfile: 'none'
  }),
  performance: entry({
    analysisKind: 'diagnosis',
    analysisGate: 'solution',
    plan: 'plan',
    phase: 'never',
    startGate: 'plan',
    executionGates: [],
    completesAfterAnalysis: false,
    humanConfirmRequired: false,
    openSpecProfile: 'light'
  })
} as const

// ---------------------------------------------------------------------------
// Helper — whether a given type+size combination includes a phase
// ---------------------------------------------------------------------------

export function flowHasPhase(type: RequestType, size: RequestSize): boolean {
  if (type === 'unknown') {
    return false
  }
  const entry = REQUEST_FLOW_REGISTRY[type]
  if (entry.phase === 'always') {
    return true
  }
  return entry.phase === 'size_l' && size === 'L'
}

// ---------------------------------------------------------------------------
// Canonical status order (for display / sort purposes)
// ---------------------------------------------------------------------------

export const REQUEST_STATUS_ORDER: readonly string[] = [
  'submitted',
  'classifying',
  'awaiting_type_confirmation',
  'analyzing',
  'awaiting_analysis_approval',
  'awaiting_information',
  'planning',
  'awaiting_plan_approval',
  'executing',
  'completed',
  'cancelled',
  'request_backlog'
] as const

// ---------------------------------------------------------------------------
// Low-confidence threshold
// NOTE: 0.6 is a provisional suggestion — no empirical data yet.
// ---------------------------------------------------------------------------

export const LOW_CONFIDENCE_THRESHOLD = 0.6

// ---------------------------------------------------------------------------
// Interrupt statuses (CR-REQ-028): not part of any flow's own step list.
// The step the request resumes at comes from Clarification.resumeStatus.
// ---------------------------------------------------------------------------

export const REQUEST_INTERRUPT_STATUSES = ['awaiting_information'] as const

export function isInterruptStatus(status: string): boolean {
  return (REQUEST_INTERRUPT_STATUSES as readonly string[]).includes(status)
}
