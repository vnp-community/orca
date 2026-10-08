import type { MobileReviewSummaryState } from './mobile-review-summary-loaders'
import { mobileReviewRiskLabel } from './mobile-review-summary-model'

export type MobileReviewSummaryChipTone = 'danger' | 'warning' | 'neutral'

export type MobileReviewSummaryChip = {
  label: string
  detail: string
  tone: MobileReviewSummaryChipTone
}

export function buildMobileReviewSummaryRoute(input: {
  hostId: string
  worktreeId: string
  name?: string
}): string {
  const query = input.name ? `?${new URLSearchParams({ name: input.name }).toString()}` : ''
  return `/h/${encodeURIComponent(input.hostId)}/review-summary/${encodeURIComponent(input.worktreeId)}${query}`
}

// Why: SOL-062 shows the branch-card chip only once availability is known, so
// loading/unavailable/error states render no chip (the overflow entry stays).
export function buildMobileReviewSummaryChip(
  state: MobileReviewSummaryState
): MobileReviewSummaryChip | null {
  if (state.kind !== 'ready') {
    return null
  }
  const { summary } = state
  const open = summary.findings?.totalOpen ?? 0
  const errors = summary.findings?.bySeverity.error ?? 0
  const warnings = summary.findings?.bySeverity.warning ?? 0
  const risk = summary.risk?.level ?? 'UNKNOWN'
  const tone: MobileReviewSummaryChipTone =
    errors > 0 || risk === 'HIGH' || risk === 'CRITICAL'
      ? 'danger'
      : warnings > 0 || risk === 'MEDIUM'
        ? 'warning'
        : 'neutral'
  const parts = [`Risk ${mobileReviewRiskLabel(risk)}`]
  parts.push(open === 1 ? '1 open finding' : `${open} open findings`)
  if (summary.stale) {
    parts.push('index stale')
  }
  return { label: 'Review summary', detail: parts.join(' · '), tone }
}
