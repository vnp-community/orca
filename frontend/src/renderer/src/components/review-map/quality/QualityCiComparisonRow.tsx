/**
 * QualityCiComparisonRow.tsx — FE-CV-TASK-087-05
 *
 * Local versus CI. A disagreement is shown with a warning shape and its hints; it never carries
 * the pass icon, and nothing here summarises "local passed" as an overall pass.
 *
 * @module components/review-map/quality/QualityCiComparisonRow
 */

import { ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { openHttpLink } from '@/lib/http-link-routing'
import type { CiComparison } from '../../../../../shared/code-intel-quality-types'
import { SeverityGlyph } from '../../quality-charts/SeverityGlyph'
import type { EncodingShape } from '../../quality-charts/severity-encoding'
import { comparisonIsDisagreement } from './quality-scorecard-model'
import { scorecardCopy } from './quality-scorecard-copy'
import type { QualityScorecardCopyKey } from './quality-scorecard-copy'

function relationShape(comparison: CiComparison): { shape: EncodingShape; className: string } {
  if (comparisonIsDisagreement(comparison) || comparison.relation === 'sha_mismatch') {
    return { shape: 'triangle', className: 'text-quality-warning' }
  }
  if (comparison.relation === 'agree_fail') {
    return { shape: 'octagon', className: 'text-quality-error' }
  }
  if (comparison.relation === 'agree_pass') {
    return { shape: 'circle-check', className: 'text-quality-pass' }
  }
  return { shape: 'circle-dashed', className: 'text-quality-unknown' }
}

function safeHttpUrl(url: string | undefined): string | null {
  return url && /^https?:\/\//i.test(url) ? url : null
}

export function QualityCiComparisonRow({
  comparison
}: {
  comparison: CiComparison
}): React.JSX.Element {
  const visual = relationShape(comparison)
  const url = safeHttpUrl(comparison.ci.url)
  return (
    <li className="flex items-start gap-2 py-1.5 text-xs" data-relation={comparison.relation}>
      <SeverityGlyph shape={visual.shape} className={`mt-0.5 shrink-0 ${visual.className}`} />
      <div className="min-w-0 flex-1">
        <p className="break-words text-foreground">
          {scorecardCopy(`ci.${comparison.relation}` as QualityScorecardCopyKey)}
        </p>
        {comparison.reasonsHint && comparison.reasonsHint.length > 0 ? (
          <div className="mt-0.5 text-muted-foreground">
            <p>{scorecardCopy('ci.hints')}</p>
            <ul className="list-disc pl-4">
              {comparison.reasonsHint.map((hint, i) => (
                <li key={`${i}:${hint}`} className="break-words">
                  {hint}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
      {url ? (
        <Button type="button" variant="ghost" size="xs" onClick={() => openHttpLink(url)}>
          <ExternalLink aria-hidden />
          {scorecardCopy('ci.open')}
        </Button>
      ) : null}
    </li>
  )
}
