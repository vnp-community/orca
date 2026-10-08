/**
 * Risk approval rules — FE-REQ-TASK-036-06
 *
 * What the approver must do before Approve unlocks. Only `enforce` mode blocks;
 * `shadow`, unassessed and low never block. Server remains the authority.
 *
 * @module components/request/impact/risk-approval-rules
 */

import { GRAPH_RISK_ORDER } from '../../../../../shared/graph-types'
import type { GraphRisk } from '../../../../../shared/graph-types'
import type { ImpactFinding, ImpactSummary } from '../../../../../shared/request-artifact-types'

export const RISK_ACCEPT_MIN_LENGTH = 10

export type ApprovalAcceptanceState = {
  viewedImpact: boolean
  acceptedFindingIds: ReadonlySet<string>
  /** Assessment digest the acceptances were recorded against. */
  digestAtAcceptance: string | null
}

export type ApprovalRequirements = {
  level: GraphRisk
  mode: 'shadow' | 'enforce'
  requiresView: boolean
  mustAccept: ImpactFinding[]
  unaccepted: ImpactFinding[]
  needsSecondApprover: boolean
  canApprove: boolean
  blockedReasonKey: string | null
}

const T = 'auto.components.request.impact.'
export const NO_ACCEPTANCES: ApprovalAcceptanceState = { viewedImpact: false, acceptedFindingIds: new Set(), digestAtAcceptance: null }

export function getApprovalRequirements(
  summary: ImpactSummary | null,
  findings: readonly ImpactFinding[],
  state: ApprovalAcceptanceState
): ApprovalRequirements {
  const open = (level: GraphRisk, mode: 'shadow' | 'enforce'): ApprovalRequirements => ({
    level, mode, requiresView: false, mustAccept: [], unaccepted: [], needsSecondApprover: false, canApprove: true, blockedReasonKey: null
  })
  // Why: no assessment (or an unknown level) keeps SOL-020 behavior: never block on missing data.
  if (!summary || summary.level === 'unknown') {return open('unknown', summary?.mode ?? 'shadow')}
  if (summary.mode !== 'enforce' || summary.level === 'low') {return open(summary.level, summary.mode)}

  if (summary.level === 'medium') {
    return { ...open('medium', 'enforce'), requiresView: true, canApprove: state.viewedImpact, blockedReasonKey: state.viewedImpact ? null : `${T}gate.viewImpactFirst` }
  }

  const mustAccept = findings.filter((f) => GRAPH_RISK_ORDER[f.level] >= GRAPH_RISK_ORDER.high)
  // Why: acceptances belong to one assessment digest; a new digest voids them all.
  const valid = state.digestAtAcceptance === summary.digest
  const unaccepted = mustAccept.filter((f) => !(valid && state.acceptedFindingIds.has(f.id)))
  const ok = unaccepted.length === 0
  return {
    level: summary.level,
    mode: 'enforce',
    requiresView: true,
    mustAccept,
    unaccepted,
    needsSecondApprover: summary.level === 'critical',
    canApprove: ok,
    blockedReasonKey: ok ? null : `${T}gate.needsAcceptance`
  }
}

/** Ids to send with approve (only the still-valid ones). */
export function acceptedIdsForApprove(summary: ImpactSummary | null, state: ApprovalAcceptanceState): string[] {
  if (!summary || state.digestAtAcceptance !== summary.digest) {return []}
  return [...state.acceptedFindingIds]
}
