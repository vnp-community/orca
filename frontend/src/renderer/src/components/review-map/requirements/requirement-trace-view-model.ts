/**
 * requirement-trace-view-model.ts — FE-CV-TASK-092-01
 *
 * Pure view model for requirement trace display.
 *
 * Language rules:
 * - NEVER use: "met", "fulfilled", "completed", "satisfied" for requirements
 * - unknown ≠ no_evidence (different states)
 * - inferred traces are suggestions, not definitive evidence
 *
 * @module components/review-map/requirements/requirement-trace-view-model
 */

// ---------------------------------------------------------------------------
// Input types
// ---------------------------------------------------------------------------

export type RequirementTraceState =
  | 'covered'
  | 'partial'
  | 'no_evidence'
  | 'unknown'
  | 'risk'

export type RequirementTrace = {
  id: string
  requirementId: string
  requirementText: string
  state: RequirementTraceState | string
  origin: 'observed' | 'inferred' | 'declared' | string
  confidence: number
  evidence: string[]
  warnings: string[]
  /** Link to source in diff or task */
  sourceRef?: string | null
}

// ---------------------------------------------------------------------------
// View model types
// ---------------------------------------------------------------------------

export type TraceGroupKey = 'no_evidence' | 'partial' | 'covered' | 'risk' | 'unknown'

export type TraceViewModel = {
  id: string
  requirementId: string
  requirementText: string
  state: RequirementTraceState | string
  /** i18n key for state label */
  stateLabelKey: string
  origin: string
  confidence: number
  evidence: string[]
  /** Warning codes shown as-is (unknown codes preserved) */
  warnings: string[]
  /** Inferred traces are suggestions; not added to definitive evidence groups */
  isSuggestion: boolean
  sourceRef: string | null
}

export type RequirementTraceViewModel = {
  /** Ordered groups: no_evidence → partial → risk → unknown → covered */
  groups: Array<{
    key: TraceGroupKey
    groupLabelKey: string
    traces: TraceViewModel[]
  }>
  /** Inferred traces (suggestions) separated from definitive evidence */
  suggestions: TraceViewModel[]
  hasTraces: boolean
}

// ---------------------------------------------------------------------------
// State label mapping
// ---------------------------------------------------------------------------

const STATE_LABEL_KEYS: Record<string, string> = {
  covered: 'auto.components.reviewMap.requirements.trace.state.covered',
  partial: 'auto.components.reviewMap.requirements.trace.state.partial',
  no_evidence: 'auto.components.reviewMap.requirements.trace.state.noEvidence',
  unknown: 'auto.components.reviewMap.requirements.trace.state.unknown',
  risk: 'auto.components.reviewMap.requirements.trace.state.risk'
}

const GROUP_LABEL_KEYS: Record<string, string> = {
  no_evidence: 'auto.components.reviewMap.requirements.trace.group.noEvidence',
  partial: 'auto.components.reviewMap.requirements.trace.group.partial',
  risk: 'auto.components.reviewMap.requirements.trace.group.risk',
  unknown: 'auto.components.reviewMap.requirements.trace.group.unknown',
  covered: 'auto.components.reviewMap.requirements.trace.group.covered'
}

// Ordered display priority: unverified items first
const GROUP_ORDER: TraceGroupKey[] = ['no_evidence', 'partial', 'risk', 'unknown', 'covered']

function toGroupKey(state: string): TraceGroupKey {
  if (['covered', 'partial', 'no_evidence', 'unknown', 'risk'].includes(state)) {
    return state as TraceGroupKey
  }
  return 'unknown'
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

/**
 * Build the requirement trace view model.
 * Groups traces by state (definitive) and separates inferred (suggestions).
 */
export function buildRequirementTraceViewModel(
  traces: RequirementTrace[]
): RequirementTraceViewModel {
  if (!traces.length) {
    return { groups: [], suggestions: [], hasTraces: false }
  }

  const definitive: TraceViewModel[] = []
  const suggestions: TraceViewModel[] = []

  for (const trace of traces) {
    const vm: TraceViewModel = {
      id: trace.id,
      requirementId: trace.requirementId,
      requirementText: trace.requirementText,
      state: trace.state,
      stateLabelKey: STATE_LABEL_KEYS[trace.state] ?? STATE_LABEL_KEYS.unknown,
      origin: trace.origin,
      confidence: trace.confidence,
      evidence: trace.evidence,
      warnings: trace.warnings,
      isSuggestion: trace.origin === 'inferred',
      sourceRef: trace.sourceRef ?? null
    }

    if (trace.origin === 'inferred') {
      suggestions.push(vm)
    } else {
      definitive.push(vm)
    }
  }

  // Build groups in display order
  const grouped = new Map<TraceGroupKey, TraceViewModel[]>()
  for (const trace of definitive) {
    const key = toGroupKey(trace.state as string)
    if (!grouped.has(key)) grouped.set(key, [])
    grouped.get(key)!.push(trace)
  }

  const groups = GROUP_ORDER
    .filter((key) => grouped.has(key))
    .map((key) => ({
      key,
      groupLabelKey: GROUP_LABEL_KEYS[key],
      traces: grouped.get(key)!
    }))

  return { groups, suggestions, hasTraces: true }
}
