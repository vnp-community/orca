/**
 * Decision rules — FE-REQ-TASK-036-04
 *
 * Pure client-side mirrors of server rules (server stays the source of truth).
 *
 * @module components/request/decision/decision-rules
 */

import { REJECT_REASON_MIN_LENGTH } from '../solution/solution-view-model'
import type { Approval } from '../../../../../shared/request-types'
import type { Decision } from '../../../../../shared/request-artifact-types'

// Why: one threshold for "explain yourself" across reject and choose-differently.
export const DECISION_RATIONALE_MIN_LENGTH = REJECT_REASON_MIN_LENGTH

export function normalizeTitle(s: string): string {
  return s.normalize('NFC').trim().toLowerCase()
}

/** Same comparison the server uses (NFC, trimmed, case-insensitive). */
export function matchesConfirmation(input: string, title: string): boolean {
  const target = normalizeTitle(title)
  return target !== '' && normalizeTitle(input) === target
}

/** A rationale is required only when a recommendation exists and the user chose a different option. */
export function requiresRationale(chosenId: string | null | undefined, recommendedId: string | null | undefined): boolean {
  return Boolean(chosenId && recommendedId && chosenId !== recommendedId)
}

export function isRationaleValid(text: string, required: boolean): boolean {
  const len = text.trim().length
  return required ? len >= DECISION_RATIONALE_MIN_LENGTH : true
}

export type DecisionGate = 'ok' | 'noDecision' | 'needsConfirmation' | 'digestChanged' | 'superseded' | 'selfChoiceForbidden'

export function getDecisionGate(
  decision: Decision | null,
  approval: Pick<Approval, 'status'> | null,
  currentDigest: string | undefined,
  flags: { selfChoiceForbidden?: boolean } = {}
): DecisionGate {
  if (flags.selfChoiceForbidden) {return 'selfChoiceForbidden'}
  if (!decision) {return 'noDecision'}
  if (decision.status === 'superseded') {return 'superseded'}
  if (decision.subjectDigest && currentDigest && decision.subjectDigest !== currentDigest) {return 'digestChanged'}
  // Why: only high-risk decisions need the second confirmation; normal ones become effective on approval.
  if (decision.riskLevel === 'high' && decision.status === 'chosen' && approval?.status !== 'approved') {return 'needsConfirmation'}
  return 'ok'
}

/**
 * Plan approval is blocked when the Request went through solution selection
 * (it has solution-subject Decisions) but none is effective yet.
 * No Decisions at all (answer/diagnosis flows, unsupported runtime) never blocks.
 */
export function planBlockedByDecision(decisions: readonly Decision[]): boolean {
  const solutionDecisions = decisions.filter((d) => d.subjectKind === 'solution_option' || d.subjectKind === 'solution')
  return solutionDecisions.length > 0 && !solutionDecisions.some((d) => d.status === 'effective')
}
