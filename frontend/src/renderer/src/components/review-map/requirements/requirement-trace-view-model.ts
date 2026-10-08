/**
 * requirement-trace-view-model.ts — FE-CV-TASK-092-01
 *
 * Pure view model for the requirements lens (CONTRACT ui-api 4.7 `RequirementTrace`).
 *
 * Wording rule: a trace is evidence found, never proof a requirement is met. Inferred
 * evidence is a suggestion and does not move a requirement out of "no evidence";
 * `unknown` (e.g. stale index) is a different state from `no_evidence`.
 *
 * @module components/review-map/requirements/requirement-trace-view-model
 */

const BASE = 'auto.components.reviewMap.requirements.trace'
const MAX_TEXT_CHARS = 300

export type RequirementState = 'has_evidence' | 'partial' | 'no_evidence' | 'manual_pending' | 'unknown'
export type EvidenceKind = 'change' | 'test' | 'check_run' | 'manual_confirmation' | 'unknown'
export type EvidenceConfidence = 'explicit' | 'derived' | 'inferred'
export type LinkConfidence = 'explicit' | 'derived' | 'inferred' | 'none'

export type RequirementEvidence = {
  kind: EvidenceKind
  ref: string
  label: string
  confidence: EvidenceConfidence
  matchedBy: string
}

export type Requirement = {
  key: string
  text: string
  origin: string
  verifyHint: string | null
  retired: boolean
  state: RequirementState
  evidence: RequirementEvidence[]
}

export type RequirementTrace = {
  subject: { source: 'task' | 'request' | 'unknown'; taskId: string | null; taskNumber: number | null; worktreeRef: string }
  linkConfidence: LinkConfidence
  requirements: Requirement[]
  unlinkedChanges: { file: string; symbols: string[]; reason: string }[]
  summary: { total: number; hasEvidence: number; partial: number; noEvidence: number; unknown: number }
  warnings: string[]
}

export type RequirementStateIcon = 'trace' | 'partial' | 'none' | 'manual' | 'unknown'

export type EvidenceRowViewModel = {
  ref: string
  label: string
  kind: EvidenceKind
  confidence: EvidenceConfidence
  matchedBy: string
  inferred: boolean
}

export type RequirementRowViewModel = {
  key: string
  text: string
  origin: string
  state: RequirementState
  stateLabelKey: string
  stateIcon: RequirementStateIcon
  evidence: EvidenceRowViewModel[]
  /** Inferred evidence: shown separately, never counted. */
  suggestions: EvidenceRowViewModel[]
}

export type RequirementGroupKey = 'noEvidence' | 'partial' | 'manualPending' | 'hasEvidence' | 'unknown'

export type RequirementTraceViewModel = {
  linkConfidence: LinkConfidence
  /** Display order: "no evidence" first. */
  groups: { key: RequirementGroupKey; labelKey: string; rows: RequirementRowViewModel[] }[]
  counts: Record<RequirementGroupKey, number>
  unlinked: RequirementTrace['unlinkedChanges']
  warnings: { code: string; labelKey: string }[]
  /** True when no task is linked: the lens offers "Link to task..." instead of inventing one. */
  canLinkTask: boolean
  isEmpty: boolean
}

const GROUP_FOR_STATE: Record<RequirementState, RequirementGroupKey> = {
  no_evidence: 'noEvidence',
  partial: 'partial',
  manual_pending: 'manualPending',
  has_evidence: 'hasEvidence',
  unknown: 'unknown'
}
const KNOWN_WARNINGS = new Set(['no_structured_criteria', 'index_stale'])
const GROUP_ORDER: RequirementGroupKey[] = ['noEvidence', 'partial', 'manualPending', 'hasEvidence', 'unknown']
const ICON_FOR_STATE: Record<RequirementState, RequirementStateIcon> = {
  has_evidence: 'trace',
  partial: 'partial',
  no_evidence: 'none',
  manual_pending: 'manual',
  unknown: 'unknown'
}

function evidenceRow(e: RequirementEvidence): EvidenceRowViewModel {
  return { ref: e.ref, label: e.label, kind: e.kind, confidence: e.confidence, matchedBy: e.matchedBy, inferred: e.confidence === 'inferred' }
}

function truncate(text: string): string {
  return text.length > MAX_TEXT_CHARS ? `${text.slice(0, MAX_TEXT_CHARS - 1)}…` : text
}

function rowFor(requirement: Requirement, showInferred: boolean): RequirementRowViewModel {
  const evidence = requirement.evidence.filter((e) => e.confidence !== 'inferred').map(evidenceRow)
  const suggestions = showInferred ? requirement.evidence.filter((e) => e.confidence === 'inferred').map(evidenceRow) : []
  let state = requirement.state
  // Why: a requirement backed only by inferred evidence has no confirmed trace, so it must not read as "has evidence".
  if ((state === 'has_evidence' || state === 'partial') && evidence.length === 0) {
    state = 'no_evidence'
  }
  return {
    key: requirement.key,
    text: truncate(requirement.text),
    origin: requirement.origin,
    state,
    stateLabelKey: `${BASE}.state.${state}`,
    stateIcon: ICON_FOR_STATE[state],
    evidence,
    suggestions
  }
}

export function buildRequirementTraceViewModel(
  trace: RequirementTrace | null,
  ctx: { showInferred: boolean }
): RequirementTraceViewModel {
  const counts: Record<RequirementGroupKey, number> = {
    noEvidence: 0, partial: 0, manualPending: 0, hasEvidence: 0, unknown: 0
  }
  if (!trace) {
    return { linkConfidence: 'none', groups: [], counts, unlinked: [], warnings: [], canLinkTask: true, isEmpty: true }
  }
  const byGroup = new Map<RequirementGroupKey, RequirementRowViewModel[]>()
  for (const requirement of trace.requirements) {
    if (requirement.retired) {continue}
    const row = rowFor(requirement, ctx.showInferred)
    const group = GROUP_FOR_STATE[row.state]
    byGroup.set(group, [...(byGroup.get(group) ?? []), row])
    counts[group]++
  }
  const groups = GROUP_ORDER.filter((key) => byGroup.has(key)).map((key) => ({
    key,
    labelKey: `${BASE}.group.${key}`,
    rows: byGroup.get(key) ?? []
  }))
  return {
    linkConfidence: trace.linkConfidence,
    groups,
    counts,
    unlinked: trace.unlinkedChanges,
    warnings: trace.warnings.map((code) => ({
      code,
      labelKey: KNOWN_WARNINGS.has(code) ? `${BASE}.warning.${code}` : `${BASE}.warning.generic`
    })),
    canLinkTask: trace.linkConfidence === 'none' || trace.linkConfidence === 'inferred',
    isEmpty: groups.length === 0
  }
}

// ---------------------------------------------------------------------------
// Wire parsing (never throws; unknown enums become 'unknown')
// ---------------------------------------------------------------------------

type Rec = Record<string, unknown>
const rec = (v: unknown): Rec => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Rec) : {})
const str = (v: unknown): string => (typeof v === 'string' ? v : '')
const arr = (v: unknown): unknown[] => (Array.isArray(v) ? v : [])
const int = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) && v >= 0 ? Math.floor(v) : 0)
const STATES = new Set(['has_evidence', 'partial', 'no_evidence', 'manual_pending', 'unknown'])
const KINDS = new Set(['change', 'test', 'check_run', 'manual_confirmation'])
const CONFIDENCES = new Set(['explicit', 'derived', 'inferred'])
const LINKS = new Set(['explicit', 'derived', 'inferred', 'none'])

export function parseRequirementTrace(raw: unknown): RequirementTrace {
  const r = rec(raw)
  const subject = rec(r.subject)
  const summary = rec(r.summary)
  return {
    subject: {
      source: subject.source === 'task' || subject.source === 'request' ? subject.source : 'unknown',
      taskId: typeof subject.taskId === 'string' ? subject.taskId : null,
      taskNumber: typeof subject.taskNumber === 'number' ? subject.taskNumber : null,
      worktreeRef: str(subject.worktreeRef)
    },
    // Why: an unrecognized link level must not look like a confirmed link.
    linkConfidence: typeof r.linkConfidence === 'string' && LINKS.has(r.linkConfidence) ? (r.linkConfidence as LinkConfidence) : 'none',
    requirements: arr(r.requirements).map((x) => {
      const q = rec(x)
      return {
        key: str(q.key),
        text: str(q.text),
        origin: str(q.origin),
        verifyHint: typeof q.verifyHint === 'string' ? q.verifyHint : null,
        retired: q.retired === true,
        state: typeof q.state === 'string' && STATES.has(q.state) ? (q.state as RequirementState) : 'unknown',
        evidence: arr(q.evidence).map((y) => {
          const e = rec(y)
          return {
            kind: typeof e.kind === 'string' && KINDS.has(e.kind) ? (e.kind as EvidenceKind) : 'unknown',
            ref: str(e.ref),
            label: str(e.label),
            // Unknown confidence is treated as inferred: the weakest claim.
            confidence: typeof e.confidence === 'string' && CONFIDENCES.has(e.confidence) ? (e.confidence as EvidenceConfidence) : 'inferred',
            matchedBy: str(e.matchedBy)
          }
        })
      }
    }),
    unlinkedChanges: arr(r.unlinkedChanges).map((x) => {
      const u = rec(x)
      return { file: str(u.file), symbols: arr(u.symbols).filter((s): s is string => typeof s === 'string'), reason: str(u.reason) }
    }),
    summary: {
      total: int(summary.total), hasEvidence: int(summary.hasEvidence), partial: int(summary.partial),
      noEvidence: int(summary.noEvidence), unknown: int(summary.unknown)
    },
    warnings: arr(r.warnings).filter((w): w is string => typeof w === 'string')
  }
}
