/**
 * QualityGateChip.tsx — FE-CV-TASK-087-05
 *
 * Compact gate verdict for headers (Review header, Create PR notice). It renders nothing unless
 * quality support is enabled and a verdict is known; the tooltip carries the first reason.
 *
 * @module components/review-map/quality/QualityGateChip
 */

import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { GateVerdictBadge } from '../../quality-charts/GateVerdictBadge'
import { useQualityGate } from '../../../hooks/useQualityGate'
import { isGateStale } from './quality-stale-model'
import { reasonLabel } from './QualityGateReasonRow'
import { scorecardCopy } from './quality-scorecard-copy'

export function QualityGateChip({
  worktreeId,
  currentHead,
  onOpen
}: {
  worktreeId: string
  currentHead?: string | null
  onOpen?: () => void
}): React.JSX.Element | null {
  const { support, response, cacheStale } = useQualityGate(worktreeId)
  if (support !== 'enabled' || !response) {
    return null
  }
  const { gate } = response
  const stale = isGateStale({ gate, cacheStale, currentHead }).stale
  const first = gate.reasons[0]
  const open = onOpen ?? (() => useAppStore.getState().setReviewLens(worktreeId, 'quality'))
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={open}
          aria-label={scorecardCopy('chip.open')}
          className="inline-flex items-center gap-1 rounded-md outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          data-testid="quality-gate-chip"
        >
          <span className="text-xs text-muted-foreground">{scorecardCopy('chip.label')}</span>
          <GateVerdictBadge verdict={gate.verdict} stale={stale} compact />
        </button>
      </TooltipTrigger>
      {first ? <TooltipContent>{reasonLabel(first)}</TooltipContent> : null}
    </Tooltip>
  )
}
