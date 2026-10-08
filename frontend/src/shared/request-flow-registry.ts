/**
 * Request Flow Registry — v6 (CR-REQ-018)
 *
 * Canonical mapping of RequestType → analysis/plan/phase/gate behaviour
 * as defined in docs/README v6 §3.4. All UI logic (stage timeline,
 * board filtering, child-request rules) reads this table — never
 * branches on `type === 'hotfix'` etc. directly.
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

export type FlowGates = {
  /** Whether an approval gate exists before analysis result is acted on */
  analysisApproval: boolean
  /** Whether an approval gate exists before the plan is executed */
  planApproval: boolean
  /** Whether a pre-deploy approval gate exists */
  preDeployApproval: boolean
}

export type FlowEntry = {
  analysisKind: AnalysisKind
  plan: PlanKind
  phase: PhaseRule
  gates: FlowGates
}

// ---------------------------------------------------------------------------
// Registry — matches README v6 §3.4 exactly (11 rows)
// ---------------------------------------------------------------------------

export const REQUEST_FLOW_REGISTRY: Readonly<Record<Exclude<RequestType, 'unknown'>, FlowEntry>> = {
  bug: {
    analysisKind: 'diagnosis',
    plan: 'task_list',
    phase: 'size_l',
    gates: { analysisApproval: false, planApproval: true, preDeployApproval: false }
  },
  task: {
    analysisKind: null,
    plan: 'task_list',
    phase: 'never',
    gates: { analysisApproval: false, planApproval: true, preDeployApproval: false }
  },
  docs: {
    analysisKind: null,
    plan: 'task_list',
    phase: 'never',
    gates: { analysisApproval: false, planApproval: false, preDeployApproval: false }
  },
  question: {
    analysisKind: 'answer',
    plan: 'none',
    phase: 'never',
    gates: { analysisApproval: false, planApproval: false, preDeployApproval: false }
  },
  hotfix: {
    analysisKind: 'diagnosis',
    plan: 'single_task',
    phase: 'never',
    gates: { analysisApproval: false, planApproval: false, preDeployApproval: true }
  },
  security: {
    analysisKind: 'findings',
    plan: 'plan',
    phase: 'size_l',
    gates: { analysisApproval: true, planApproval: true, preDeployApproval: true }
  },
  ops_request: {
    analysisKind: null,
    plan: 'task_list',
    phase: 'never',
    gates: { analysisApproval: false, planApproval: true, preDeployApproval: false }
  },
  change_request: {
    analysisKind: 'solution',
    plan: 'plan',
    phase: 'always',
    gates: { analysisApproval: true, planApproval: true, preDeployApproval: true }
  },
  refactor: {
    analysisKind: 'solution',
    plan: 'plan',
    phase: 'size_l',
    gates: { analysisApproval: true, planApproval: true, preDeployApproval: false }
  },
  spike: {
    analysisKind: 'findings',
    plan: 'none',
    phase: 'never',
    gates: { analysisApproval: true, planApproval: false, preDeployApproval: false }
  },
  performance: {
    analysisKind: 'findings',
    plan: 'task_list',
    phase: 'size_l',
    gates: { analysisApproval: true, planApproval: true, preDeployApproval: false }
  }
} as const

// ---------------------------------------------------------------------------
// Helper — whether a given type+size combination includes a phase
// ---------------------------------------------------------------------------

export function flowHasPhase(type: RequestType, size: RequestSize): boolean {
  if (type === 'unknown') return false
  const entry = REQUEST_FLOW_REGISTRY[type]
  if (entry.phase === 'always') return true
  if (entry.phase === 'size_l') return size === 'L'
  return false
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
