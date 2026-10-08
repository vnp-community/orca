/**
 * QualityGateVerdictHeader.tsx — FE-CV-TASK-087-05
 *
 * Verdict badge, headline and mode notice. An unknown verdict names its first reason and
 * never borrows the wording or icon of a pass; the gate is advisory in every mode.
 *
 * @module components/review-map/quality/QualityGateVerdictHeader
 */

import type { QualityGate } from '../../../../../shared/code-intel-quality-types'
import { GateVerdictBadge } from '../../quality-charts/GateVerdictBadge'
import { modeNotice, unknownReason, verdictHeadline } from './quality-gate-copy'

export function QualityGateVerdictHeader({
  gate,
  stale
}: {
  gate: QualityGate
  stale: boolean
}): React.JSX.Element {
  return (
    <div className="flex flex-col gap-1" data-testid="quality-verdict" data-verdict={gate.verdict}>
      <div className="flex flex-wrap items-center gap-2">
        <GateVerdictBadge verdict={gate.verdict} stale={stale} />
        {gate.verdict === 'unknown' ? (
          <span className="text-xs text-muted-foreground">{unknownReason(gate)}</span>
        ) : gate.verdict === 'pass' ? (
          // Why: warn/fail are fully named by the badge; only a pass needs the profile spelled out.
          <span className="text-sm font-medium text-foreground">
            {verdictHeadline(gate.verdict, gate.profile)}
          </span>
        ) : null}
      </div>
      <p className="text-xs text-muted-foreground">{modeNotice(gate.mode)}</p>
    </div>
  )
}
