/**
 * Impact assessment parsers — FE-REQ-TASK-036-01
 *
 * @module shared/request-impact-parsers
 */

import { parseGraphRisk } from './graph-wire-parsers'
import type {
  ImpactComparison,
  ImpactDrift,
  ImpactFinding,
  ImpactStatus,
  ImpactSummary
} from './request-artifact-types'
import { num, oneOf, pick, rec, str, strings } from './request-artifact-wire-fields'

export function parseImpactSummary(raw: unknown): ImpactSummary | null {
  const o = rec(raw)
  if (!o) {
    return null
  }
  const assessmentId = str(pick(o, 'assessmentId', 'assessment_id', 'id'))
  if (!assessmentId) {
    return null
  }
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
  if (age !== undefined) {
    s.indexAgeCommits = age
  }
  const narrative = str(o.narrative)
  if (narrative) {
    s.narrative = narrative
  }
  return s
}

export function parseImpactFinding(raw: unknown): ImpactFinding | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {
    return null
  }
  const f: ImpactFinding = {
    id,
    dimension: str(o.dimension) ?? 'unknown',
    level: parseGraphRisk(pick(o, 'level', 'risk')),
    title: str(o.title) ?? id
  }
  const ev = str(pick(o, 'evidenceRef', 'evidence_ref'))
  if (ev) {
    f.evidenceRef = ev
  }
  const nodes = strings(pick(o, 'nodeIds', 'node_ids'))
  if (nodes.length > 0) {
    f.nodeIds = nodes
  }
  return f
}

export function parseImpactComparison(raw: unknown): ImpactComparison | null {
  const o = rec(raw)
  const optionId = o ? str(pick(o, 'optionId', 'option_id')) : undefined
  if (!o || !optionId) {
    return null
  }
  const dimensions: ImpactComparison['dimensions'] = {}
  for (const [k, v] of Object.entries(rec(o.dimensions) ?? {})) {
    const d = rec(v)
    if (!d) {
      continue
    }
    dimensions[k] = {
      level: parseGraphRisk(pick(d, 'level', 'risk')),
      score: num(d.score) ?? null,
      ...(str(d.note) ? { note: str(d.note) } : {})
    }
  }
  return { optionId, dimensions }
}

export function parseImpactDrift(raw: unknown): ImpactDrift | null {
  const o = rec(raw)
  const phaseId = o ? str(pick(o, 'phaseId', 'phase_id')) : undefined
  if (!o || !phaseId) {
    return null
  }
  const items = (Array.isArray(o.items) ? o.items : []).flatMap((x) => {
    const r = rec(x)
    const taskId = r ? str(pick(r, 'taskId', 'task_id')) : undefined
    return r && taskId
      ? [{ taskId, expected: str(r.expected) ?? '', actual: str(r.actual) ?? '' }]
      : []
  })
  return { phaseId, drifted: o.drifted === true || items.length > 0, items }
}
