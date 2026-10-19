/**
 * RequestFlowView — CR-REQ-037 2.2
 *
 * Reads the flow of a request type the way the screens need it: from `request.flow` when the gateway answers, from the built-in
 * fallback table otherwise.
 *
 * @module shared/request-flow-view
 */

import { REQUEST_FLOW_REGISTRY, flowHasPhase } from './request-flow-registry'
import type { ExecutionGate, FlowEntry, PlanKind, StartGate } from './request-flow-registry'
import type { RequestFlowView } from './request-execution-types'
import type { RequestSize, RequestType } from './request-types'

const PLAN_KINDS = new Set<PlanKind>(['plan', 'task_list', 'single_task', 'none'])
const START_GATES = new Set<string>(['plan', 'task_list', 'pre_deploy'])
const EXECUTION_GATES = new Set<string>(['phase', 'pre_deploy'])
const ANALYSIS_KINDS = new Set<string>(['solution', 'diagnosis', 'findings', 'answer'])
const ANALYSIS_GATES = new Set<string>(['solution', 'findings', 'answer'])
const PROFILES = new Set<string>(['full', 'light', 'none'])

export function flowViewOfEntry(
  type: string,
  entry: FlowEntry,
  size: RequestSize | undefined
): RequestFlowView {
  const phased = entry.phase === 'always' || (entry.phase === 'size_l' && size === 'L')
  return {
    type,
    analysisKind: entry.analysisKind,
    analysisGate: entry.analysisGate,
    planKind: entry.plan,
    hasPhases: phased,
    startGate: entry.startGate,
    executionGates: entry.executionGates,
    completesAfterAnalysis: entry.completesAfterAnalysis,
    humanConfirmRequired: entry.humanConfirmRequired,
    openSpecProfile: entry.openSpecProfile,
    source: 'fallback'
  }
}

/** The built-in answer; null for an unknown type. */
export function fallbackRequestFlow(
  type: RequestType,
  size: RequestSize | undefined
): RequestFlowView | null {
  if (type === 'unknown') {
    return null
  }
  return flowViewOfEntry(type, REQUEST_FLOW_REGISTRY[type], size)
}

/** Parses the answer of `request.flow`; null when the payload is not a flow. */
export function parseRequestFlow(raw: unknown): RequestFlowView | null {
  if (!raw || typeof raw !== 'object') {
    return null
  }
  const r = raw as Record<string, unknown>
  if (
    typeof r.type !== 'string' ||
    typeof r.planKind !== 'string' ||
    !PLAN_KINDS.has(r.planKind as PlanKind)
  ) {
    return null
  }
  const analysis =
    typeof r.analysisKind === 'string' && ANALYSIS_KINDS.has(r.analysisKind) ? r.analysisKind : null
  const gate =
    typeof r.analysisGate === 'string' && ANALYSIS_GATES.has(r.analysisGate) ? r.analysisGate : null
  const start =
    typeof r.startGate === 'string' && START_GATES.has(r.startGate)
      ? (r.startGate as StartGate)
      : null
  const execution = Array.isArray(r.executionGates)
    ? (r.executionGates.filter(
        (g) => typeof g === 'string' && EXECUTION_GATES.has(g)
      ) as ExecutionGate[])
    : []
  return {
    type: r.type,
    analysisKind: analysis as RequestFlowView['analysisKind'],
    analysisGate: gate as RequestFlowView['analysisGate'],
    planKind: r.planKind as PlanKind,
    hasPhases: r.hasPhases === true,
    startGate: start,
    executionGates: execution,
    completesAfterAnalysis: r.completesAfterAnalysis === true,
    humanConfirmRequired: r.humanConfirmRequired === true,
    openSpecProfile:
      typeof r.openSpecProfile === 'string' && PROFILES.has(r.openSpecProfile)
        ? (r.openSpecProfile as RequestFlowView['openSpecProfile'])
        : 'none',
    source: 'backend'
  }
}

/** Whether the flow's execution gates (the plan's mode removing the Phase gate when continuous) include `gate`. */
export function flowHasExecutionGate(
  flow: Pick<RequestFlowView, 'executionGates'>,
  gate: ExecutionGate,
  mode: 'step_by_step' | 'continuous' | undefined
): boolean {
  if (gate === 'phase' && mode === 'continuous') {
    return false
  }
  return flow.executionGates.includes(gate)
}

export { flowHasPhase }
