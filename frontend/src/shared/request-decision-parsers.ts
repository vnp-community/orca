/**
 * Decision parser — FE-REQ-TASK-036-01
 *
 * @module shared/request-decision-parsers
 */

import type { Decision, DecisionStatus } from './request-artifact-types'
import { num, oneOf, pick, rec, str, strings } from './request-artifact-wire-fields'

export function parseDecision(raw: unknown): Decision | null {
  const o = rec(raw)
  const id = o ? str(o.id) : undefined
  if (!o || !id) {
    return null
  }
  const d: Decision = {
    id,
    displayId: str(pick(o, 'displayId', 'display_id')) ?? id,
    subjectKind: str(pick(o, 'subjectKind', 'subject_kind')) ?? '',
    subjectId: str(pick(o, 'subjectId', 'subject_id')) ?? '',
    subjectDigest: str(pick(o, 'subjectDigest', 'subject_digest')) ?? '',
    rationale: str(o.rationale) ?? '',
    riskLevel: pick(o, 'riskLevel', 'risk_level') === 'high' ? 'high' : 'normal',
    riskReasons: strings(pick(o, 'riskReasons', 'risk_reasons')),
    status: oneOf<DecisionStatus>(
      o.status,
      ['open', 'chosen', 'effective', 'superseded'],
      'unknown'
    ),
    version: num(o.version) ?? 0
  }
  const chosen = str(pick(o, 'chosenOptionId', 'chosen_option_id'))
  if (chosen) {
    d.chosenOptionId = chosen
  }
  const rec2 = str(pick(o, 'recommendedOptionId', 'recommended_option_id'))
  if (rec2) {
    d.recommendedOptionId = rec2
  }
  const chooser = str(pick(o, 'chooserId', 'chooser_id'))
  if (chooser) {
    d.chooserId = chooser
  }
  const confirmed = str(pick(o, 'confirmedBy', 'confirmed_by'))
  if (confirmed) {
    d.confirmedBy = confirmed
  }
  return d
}
