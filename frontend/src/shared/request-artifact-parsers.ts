/**
 * Request artifact parsers — FE-REQ-TASK-036-01
 *
 * Never throw. Unknown enums become 'unknown'; missing `id` returns null so
 * callers drop the item. JSON-in-string fields are parsed when possible and
 * kept raw when broken.
 *
 * @module shared/request-artifact-parsers
 */

import { parseGraphRisk } from './graph-wire-parsers'
import type {
  Clarification,
  ClarificationQuestion,
  ClarificationQuestionKind,
  ClarificationSource,
  ClarificationStatus,
  Decision,
  DecisionStatus,
  ExecutionFailureClass,
  ExecutionResult,
  ImpactComparison,
  ImpactDrift,
  ImpactFinding,
  ImpactStatus,
  ImpactSummary,
  TaskReadinessOutcome,
  TaskReadinessReport
} from './request-artifact-types'

type Rec = Record<string, unknown>

function rec(v: unknown): Rec | null {
  return typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Rec) : null
}
function str(v: unknown): string | undefined {
  return typeof v === 'string' && v !== '' ? v : undefined
}
function num(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}
function pick(o: Rec, ...keys: string[]): unknown {
  for (const k of keys) {
    if (o[k] !== undefined) {return o[k]}
  }
  return undefined
}
function strings(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}
/** Accepts an object/array, or a JSON string; broken JSON stays a raw string. */
function jsonish(v: unknown): unknown {
  if (typeof v !== 'string') {return v}
  try {
    return JSON.parse(v)
  } catch {
    return v
  }
}
function oneOf<T extends string>(v: unknown, allowed: readonly T[], fallback: T): T {
  return typeof v === 'string' && (allowed as readonly string[]).includes(v) ? (v as T) : fallback
}

const KIND_ALIASES: Record<string, ClarificationQuestionKind> = {
  text: 'text', single: 'single_choice', single_choice: 'single_choice', multi: 'multi_choice',
  multi_choice: 'multi_choice', file: 'file', confirm: 'boolean', boolean: 'boolean'
}

function parseQuestion(raw: unknown): ClarificationQuestion | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {return null}
  const kindRaw = String(o.kind ?? '').toLowerCase()
  const options = jsonish(pick(o, 'options', 'optionsJson', 'options_json'))
  const q: ClarificationQuestion = {
    id,
    seq: num(o.seq) ?? 0,
    questionKey: str(pick(o, 'questionKey', 'question_key')) ?? id,
    kind: KIND_ALIASES[kindRaw] ?? 'text',
    prompt: str(o.prompt) ?? '',
    required: o.required !== false
  }
  const reason = str(o.reason)
  if (reason) {q.reason = reason}
  if (Array.isArray(options)) {
    q.options = options.flatMap((x) => {
      if (typeof x === 'string') {return [{ value: x, label: x }]}
      const r = rec(x)
      const value = r ? str(r.value) : undefined
      return r && value ? [{ value, label: str(r.label) ?? value }] : []
    })
  }
  const def = jsonish(pick(o, 'suggestedDefault', 'suggestedDefaultJson', 'suggested_default_json'))
  if (def !== undefined && def !== null && def !== '') {q.suggestedDefault = def}
  const ans = jsonish(pick(o, 'answer', 'answerJson', 'answer_json'))
  if (ans !== undefined && ans !== null && ans !== '') {q.answer = ans}
  return q
}

export function parseClarification(raw: unknown): Clarification | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {return null}
  const questions = (Array.isArray(o.questions) ? o.questions : [])
    .map(parseQuestion)
    .filter((q): q is ClarificationQuestion => q !== null)
    .sort((a, b) => a.seq - b.seq)
  const c: Clarification = {
    id,
    displayId: str(pick(o, 'displayId', 'display_id')) ?? id,
    requestId: str(pick(o, 'requestId', 'request_id')) ?? '',
    source: oneOf<ClarificationSource>(o.source, ['readiness', 'solution_open_question', 'plan_assumption', 'task_blocked'], 'unknown'),
    status: oneOf<ClarificationStatus>(o.status, ['open', 'answered', 'expired', 'cancelled'], 'unknown'),
    round: num(o.round) ?? 1,
    questions,
    version: num(o.version) ?? 0
  }
  const sourceRef = str(pick(o, 'sourceRef', 'source_ref'))
  if (sourceRef) {c.sourceRef = sourceRef}
  const resume = str(pick(o, 'resumeStatus', 'resume_status'))
  if (resume) {c.resumeStatus = resume}
  const due = str(pick(o, 'dueAt', 'due_at'))
  if (due) {c.dueAt = due}
  const assignees = strings(pick(o, 'assigneeIds', 'assignee_ids'))
  if (assignees.length > 0) {c.assigneeIds = assignees}
  return c
}

export function parseDecision(raw: unknown): Decision | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {return null}
  const d: Decision = {
    id,
    displayId: str(pick(o, 'displayId', 'display_id')) ?? id,
    subjectKind: str(pick(o, 'subjectKind', 'subject_kind')) ?? '',
    subjectId: str(pick(o, 'subjectId', 'subject_id')) ?? '',
    subjectDigest: str(pick(o, 'subjectDigest', 'subject_digest')) ?? '',
    rationale: str(o.rationale) ?? '',
    riskLevel: pick(o, 'riskLevel', 'risk_level') === 'high' ? 'high' : 'normal',
    riskReasons: strings(pick(o, 'riskReasons', 'risk_reasons')),
    status: oneOf<DecisionStatus>(o.status, ['open', 'chosen', 'effective', 'superseded'], 'unknown'),
    version: num(o.version) ?? 0
  }
  const chosen = str(pick(o, 'chosenOptionId', 'chosen_option_id'))
  if (chosen) {d.chosenOptionId = chosen}
  const rec2 = str(pick(o, 'recommendedOptionId', 'recommended_option_id'))
  if (rec2) {d.recommendedOptionId = rec2}
  const chooser = str(pick(o, 'chooserId', 'chooser_id'))
  if (chooser) {d.chooserId = chooser}
  const confirmed = str(pick(o, 'confirmedBy', 'confirmed_by'))
  if (confirmed) {d.confirmedBy = confirmed}
  return d
}

export function parseImpactSummary(raw: unknown): ImpactSummary | null {
  const o = rec(raw)
  if (!o) {return null}
  const assessmentId = str(pick(o, 'assessmentId', 'assessment_id', 'id'))
  if (!assessmentId) {return null}
  const conf = o.confidence
  const s: ImpactSummary = {
    assessmentId,
    digest: str(o.digest) ?? '',
    // Why: a missing level is "not assessed", never "low".
    level: parseGraphRisk(pick(o, 'level', 'risk')),
    score: num(o.score) ?? null,
    topReasons: strings(pick(o, 'topReasons', 'top_reasons')).slice(0, 3),
    confidence: conf === 'low' || conf === 'medium' || conf === 'high' ? conf : null,
    assessedAt: str(pick(o, 'assessedAt', 'assessed_at')) ?? null,
    tool: str(o.tool) ?? null,
    stale: o.stale === true,
    mode: o.mode === 'enforce' ? 'enforce' : 'shadow',
    status: oneOf<ImpactStatus>(o.status, ['collecting', 'ready', 'partial', 'failed'], 'unknown'),
    hardRules: strings(pick(o, 'hardRules', 'hard_rules'))
  }
  const age = num(pick(o, 'indexAgeCommits', 'index_age_commits'))
  if (age !== undefined) {s.indexAgeCommits = age}
  const narrative = str(o.narrative)
  if (narrative) {s.narrative = narrative}
  return s
}

export function parseImpactFinding(raw: unknown): ImpactFinding | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {return null}
  const f: ImpactFinding = {
    id,
    dimension: str(o.dimension) ?? 'unknown',
    level: parseGraphRisk(pick(o, 'level', 'risk')),
    title: str(o.title) ?? id
  }
  const ev = str(pick(o, 'evidenceRef', 'evidence_ref'))
  if (ev) {f.evidenceRef = ev}
  const nodes = strings(pick(o, 'nodeIds', 'node_ids'))
  if (nodes.length > 0) {f.nodeIds = nodes}
  return f
}

export function parseImpactComparison(raw: unknown): ImpactComparison | null {
  const o = rec(raw)
  const optionId = o ? str(pick(o, 'optionId', 'option_id')) : undefined
  if (!o || !optionId) {return null}
  const dimensions: ImpactComparison['dimensions'] = {}
  for (const [k, v] of Object.entries(rec(o.dimensions) ?? {})) {
    const d = rec(v)
    if (!d) {continue}
    dimensions[k] = { level: parseGraphRisk(pick(d, 'level', 'risk')), score: num(d.score) ?? null, ...(str(d.note) ? { note: str(d.note) } : {}) }
  }
  return { optionId, dimensions }
}

export function parseImpactDrift(raw: unknown): ImpactDrift | null {
  const o = rec(raw)
  const phaseId = o ? str(pick(o, 'phaseId', 'phase_id')) : undefined
  if (!o || !phaseId) {return null}
  const items = (Array.isArray(o.items) ? o.items : []).flatMap((x) => {
    const r = rec(x)
    const taskId = r ? str(pick(r, 'taskId', 'task_id')) : undefined
    return r && taskId ? [{ taskId, expected: str(r.expected) ?? '', actual: str(r.actual) ?? '' }] : []
  })
  return { phaseId, drifted: o.drifted === true || items.length > 0, items }
}

export function parseTaskReadinessReport(raw: unknown): TaskReadinessReport | null {
  const o = rec(raw)
  const taskId = o ? str(pick(o, 'taskId', 'task_id')) : undefined
  if (!o || !taskId) {return null}
  const findings = (Array.isArray(o.findings) ? o.findings : []).flatMap((x) => {
    const r = rec(x)
    const code = r ? str(r.code) : undefined
    return r && code ? [{ code, tier: str(r.tier) ?? '', message: str(r.message) ?? '', ...(str(r.path) ? { path: str(r.path) } : {}) }] : []
  })
  const rep: TaskReadinessReport = {
    taskId,
    outcome: oneOf<TaskReadinessOutcome>(o.outcome, ['ready', 'needs_info', 'spec_defect', 'env_defect'], 'unknown'),
    findings
  }
  const tier = str(o.tier)
  if (tier) {rep.tier = tier}
  const digest = str(pick(o, 'specDigest', 'spec_digest'))
  if (digest) {rep.specDigest = digest}
  const dur = num(pick(o, 'durationMs', 'duration_ms'))
  if (dur !== undefined) {rep.durationMs = dur}
  const at = str(pick(o, 'checkedAt', 'checked_at'))
  if (at) {rep.checkedAt = at}
  return rep
}

export function parseExecutionResult(raw: unknown): ExecutionResult | null {
  const o = rec(raw)
  const taskId = o ? str(pick(o, 'taskId', 'task_id')) : undefined
  if (!o || !taskId) {return null}
  const verdictRaw = rec(o.verdict)
  const r: ExecutionResult = {
    taskId,
    attempt: num(o.attempt) ?? 1,
    parseStatus: oneOf(pick(o, 'parseStatus', 'parse_status'), ['ok', 'missing', 'invalid'] as const, 'invalid'),
    filesChanged: strings(pick(o, 'filesChanged', 'files_changed')),
    checksRun: (Array.isArray(pick(o, 'checksRun', 'checks_run')) ? (pick(o, 'checksRun', 'checks_run') as unknown[]) : []).flatMap((x) => {
      const c = rec(x)
      const id = c ? str(c.id) : undefined
      return c && id ? [{ id, exit: num(c.exit) ?? -1 }] : []
    })
  }
  const status = o.status
  if (status === 'done' || status === 'blocked' || status === 'failed' || status === 'needs_info') {r.status = status}
  const summary = str(o.summary)
  if (summary) {r.summary = summary}
  if (verdictRaw) {
    r.verdict = {
      status: verdictRaw.status === 'passed' ? 'passed' : 'failed',
      findings: (Array.isArray(verdictRaw.findings) ? verdictRaw.findings : []).flatMap((x) => {
        const f = rec(x)
        const code = f ? str(f.code) : undefined
        return f && code ? [{ code, message: str(f.message) ?? '' }] : []
      })
    }
  }
  const fc = pick(o, 'failureClass', 'failure_class')
  if (fc !== undefined) {
    r.failureClass = oneOf<ExecutionFailureClass>(fc, ['retryable', 'needs_info', 'spec_defect', 'env_defect', 'agent_defect'], 'unknown')
  }
  const tail = str(pick(o, 'stdoutTail', 'stdout_tail'))
  if (tail) {r.stdoutTail = tail}
  const outputs = rec(jsonish(o.outputs))
  if (outputs) {r.outputs = outputs}
  return r
}
