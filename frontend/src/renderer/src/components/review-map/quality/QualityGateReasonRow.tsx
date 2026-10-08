/**
 * QualityGateReasonRow.tsx — FE-CV-TASK-087-05
 *
 * One check behind the verdict: label from the reason code (falling back to the check name),
 * observed versus threshold as the backend formatted them, a result shown by shape and text,
 * and a way into the findings list when the reason names a category or tool.
 *
 * @module components/review-map/quality/QualityGateReasonRow
 */

import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import type { QualityGateReason } from '../../../../../shared/code-intel-quality-types'
import { SeverityGlyph } from '../../quality-charts/SeverityGlyph'
import { VERDICT_ENCODING, toVerdictLevel } from '../../quality-charts/severity-encoding'
import { REVIEW_QUALITY_COPY_ROOT } from './quality-copy-factory'
import { reasonCodeKeySegment, reasonFilter } from './quality-scorecard-model'
import { QUALITY_SCORECARD_COPY_GROUP, scorecardCopy } from './quality-scorecard-copy'

export type ReasonFilter = { category?: string; tool?: string }

/** Backend text is plain text; an unknown code falls back to the check name. */
export function reasonLabel(reason: QualityGateReason): string {
  const segment = reasonCodeKeySegment(reason.code)
  const fallback = reason.check || reason.code || scorecardCopy('verdict.profileMissing')
  if (!segment) {
    return fallback
  }
  return translate(
    `${REVIEW_QUALITY_COPY_ROOT}${QUALITY_SCORECARD_COPY_GROUP}.reasonCode.${segment}`,
    fallback,
    reason.params
  )
}

export function QualityGateReasonRow({
  reason,
  onShowFindings
}: {
  reason: QualityGateReason
  onShowFindings?: (filter: ReasonFilter | null) => void
}): React.JSX.Element {
  const level = toVerdictLevel(reason.result)
  const entry = VERDICT_ENCODING[level]
  const filter = reasonFilter(reason)
  const values =
    reason.observed || reason.threshold
      ? scorecardCopy('reason.values', {
          observed: reason.observed || '—',
          threshold: reason.threshold || '—'
        })
      : null
  return (
    <li
      className="flex items-start gap-2 py-1.5 text-xs"
      data-result={level}
      data-testid="quality-reason"
    >
      <SeverityGlyph shape={entry.shape} className={`mt-0.5 shrink-0 ${entry.textClass}`} />
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium text-foreground">{reasonLabel(reason)}</p>
        {values ? <p className="text-muted-foreground">{values}</p> : null}
        {reason.waivedCount ? (
          <p className="text-muted-foreground">
            {scorecardCopy('reason.waived', { count: reason.waivedCount })}
          </p>
        ) : null}
      </div>
      <span className="shrink-0 text-muted-foreground">{scorecardCopy(`reason.${level}`)}</span>
      {onShowFindings ? (
        <Button
          type="button"
          variant="ghost"
          size="xs"
          title={filter ? undefined : scorecardCopy('reason.noFilter')}
          onClick={() => onShowFindings(filter)}
        >
          {scorecardCopy('reason.show')}
        </Button>
      ) : null}
    </li>
  )
}
