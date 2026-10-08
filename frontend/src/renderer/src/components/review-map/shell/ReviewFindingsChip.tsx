/**
 * ReviewFindingsChip.tsx — FE-CV-TASK-059-05
 *
 * "N findings" chip in the summary bar: open structural findings in the changed scope. Clicking
 * it opens the dock's findings panel. Uses the same server filters as the panel's first load so
 * both share one cached query.
 *
 * @module components/review-map/shell/ReviewFindingsChip
 */

import { useCodeIntelFindings } from '../../../hooks/useCodeIntelFindings'
import type { FindingsServerFilters } from '../../../hooks/useCodeIntelFindings'
import { translate } from '@/i18n/i18n'
import { requestReviewDockPanel } from './review-dock-focus'

const FILTERS: FindingsServerFilters = { scope: 'changed', includeDismissed: false, severities: [] }

export function ReviewFindingsChip({
  worktreeId,
  environmentId
}: {
  worktreeId: string
  environmentId: string | null
}): React.JSX.Element | null {
  const findings = useCodeIntelFindings(worktreeId, environmentId, FILTERS)
  if (findings.status !== 'success') {
    return null
  }
  const count = findings.findings.filter((f) => !f.dismissed).length
  const label = findings.hasNextPage
    ? translate('auto.components.reviewMap.shell.chip.findingsMore', '{{count}}+ findings', {
        count
      })
    : translate('auto.components.reviewMap.shell.chip.findings', '{{count}} findings', { count })
  return (
    <button
      type="button"
      data-chip="findings"
      disabled={count === 0}
      onClick={() => requestReviewDockPanel(worktreeId, 'findings')}
      className={`h-6 rounded-full border px-2 text-xs text-muted-foreground ${
        count === 0 ? 'opacity-50' : 'hover:bg-accent/50'
      }`}
    >
      {label}
    </button>
  )
}
