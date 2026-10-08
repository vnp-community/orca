/**
 * quality-gate-copy.ts — FE-CV-TASK-087-04
 *
 * Wording of the gate verdict. `unknown` is "not enough data to conclude" and never borrows the
 * words of a pass; `mode: block` is still shown as a warning (O9). Strings are built at call time.
 *
 * @module components/review-map/quality/quality-gate-copy
 */

import type { GateResult, QualityGate } from '../../../../../shared/code-intel-quality-types'
import { splitProfileRef } from './quality-profile-selection'
import { scorecardCopy } from './quality-scorecard-copy'

export function verdictHeadline(verdict: GateResult, profileRef: string): string {
  switch (verdict) {
    case 'pass':
      return scorecardCopy('verdict.pass', {
        profile: splitProfileRef(profileRef).name || scorecardCopy('verdict.profileMissing')
      })
    case 'warn':
      return scorecardCopy('verdict.warn')
    case 'fail':
      return scorecardCopy('verdict.fail')
    default:
      return scorecardCopy('verdict.unknown')
  }
}

export function modeNotice(mode: QualityGate['mode']): string {
  switch (mode) {
    case 'inform':
      return scorecardCopy('mode.inform')
    case 'block':
      return scorecardCopy('mode.block')
    default:
      return scorecardCopy('mode.unknown')
  }
}

/** Why the verdict is unknown: the first reason the backend gave, or an explicit "no reason". */
export function unknownReason(gate: QualityGate): string {
  const first = gate.reasons.find((r) => r.check !== '' || (r.code ?? '') !== '')
  return first ? first.check || (first.code ?? '') : scorecardCopy('verdict.unknownNoReason')
}
