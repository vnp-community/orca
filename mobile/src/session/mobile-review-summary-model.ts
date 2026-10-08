import type {
  MobileReviewSummary,
  MobileReviewSummaryFinding,
  MobileReviewSummaryOrigin,
  MobileReviewSummaryRiskLevel,
  MobileReviewSummarySeverity
} from './mobile-review-summary-rpc'

export type MobileReviewSummaryFilter = 'all' | 'error' | 'warning' | 'introduced'

export const MOBILE_REVIEW_SUMMARY_FILTERS: MobileReviewSummaryFilter[] = [
  'all',
  'error',
  'warning',
  'introduced'
]

const FILTER_LABELS: Record<MobileReviewSummaryFilter, string> = {
  all: 'All',
  error: 'Error',
  warning: 'Warning',
  introduced: 'Only this change'
}

const SEVERITY_RANK: Record<MobileReviewSummarySeverity, number> = {
  error: 0,
  warning: 1,
  info: 2
}
const ORIGIN_RANK: Record<MobileReviewSummaryOrigin, number> = {
  introduced: 0,
  touched: 1,
  unknown: 2,
  preexisting: 3
}

export function mobileReviewSummaryFilterLabel(filter: MobileReviewSummaryFilter): string {
  return FILTER_LABELS[filter]
}

export function mobileReviewSeverityLabel(severity: MobileReviewSummarySeverity): string {
  return severity === 'error' ? 'Error' : severity === 'warning' ? 'Warning' : 'Info'
}

export function mobileReviewOriginLabel(origin: MobileReviewSummaryOrigin): string {
  switch (origin) {
    case 'introduced':
      return 'introduced by this change'
    case 'touched':
      return 'in code this change touches'
    case 'preexisting':
      return 'pre-existing'
    default:
      return 'origin unknown'
  }
}

export function mobileReviewRiskLabel(level: MobileReviewSummaryRiskLevel): string {
  return level === 'UNKNOWN' ? 'Unknown' : level.charAt(0) + level.slice(1).toLowerCase()
}

export function filterMobileReviewFindings(
  items: MobileReviewSummaryFinding[],
  filter: MobileReviewSummaryFilter
): MobileReviewSummaryFinding[] {
  switch (filter) {
    case 'error':
    case 'warning':
      return items.filter((item) => item.severity === filter)
    case 'introduced':
      return items.filter((item) => item.origin === 'introduced')
    default:
      return items
  }
}

export function sortMobileReviewFindings(
  items: MobileReviewSummaryFinding[]
): MobileReviewSummaryFinding[] {
  // Why: stable by index so findings of equal rank keep the host's order.
  return items
    .map((item, index) => ({ item, index }))
    .sort(
      (a, b) =>
        SEVERITY_RANK[a.item.severity] - SEVERITY_RANK[b.item.severity] ||
        ORIGIN_RANK[a.item.origin] - ORIGIN_RANK[b.item.origin] ||
        a.index - b.index
    )
    .map(({ item }) => item)
}

export function formatMobileReviewAge(iso: string | undefined, now: number): string | null {
  if (!iso) {
    return null
  }
  const then = Date.parse(iso)
  if (!Number.isFinite(then)) {
    return null
  }
  const minutes = Math.max(0, Math.floor((now - then) / 60_000))
  if (minutes < 1) {
    return 'just now'
  }
  if (minutes < 60) {
    return `${minutes} min ago`
  }
  const hours = Math.floor(minutes / 60)
  return hours < 24 ? `${hours} h ago` : `${Math.floor(hours / 24)} d ago`
}

export function mobileReviewTruncationLabel(summary: MobileReviewSummary): string | null {
  const findings = summary.findings
  if (!findings || !(findings.truncated || summary.truncated)) {
    return null
  }
  return `Showing ${findings.items.length} / ${findings.totalOpen}`
}

export function canOpenMobileReviewFindingDiff(finding: MobileReviewSummaryFinding): boolean {
  return finding.inChangedFiles && Boolean(finding.filePath)
}
