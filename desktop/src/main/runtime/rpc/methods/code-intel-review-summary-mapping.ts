import {
  CodeIntelPortError,
  type CodeIntelSummaryPortFinding,
  type CodeIntelSummaryPortFindings,
  type CodeIntelSummaryPortOverlay,
  type CodeIntelSummaryPortStatus
} from './code-intel-summary-port'

export const REVIEW_SUMMARY_MAX_ITEMS = 50
const MAX_TITLE = 200
const MAX_SUMMARY = 400

export type MobileReviewSummaryWire = {
  available: boolean
  reason?: 'flag_off' | 'no_binding' | 'index_missing' | 'tool_unavailable'
  index?: {
    state: string
    indexedCommit?: string
    headCommit?: string
    indexedAt?: string
  }
  counts?: Record<'files' | 'symbols' | 'flows' | 'tables' | 'contracts' | 'uncovered', number>
  risk?: { level: string; reasons: string[] }
  findings?: {
    totalOpen: number
    bySeverity: { error: number; warning: number; info: number }
    items: {
      key: string
      kind: string
      severity: string
      title: string
      summary: string
      origin: string
      filePath?: string
      startLine?: number
      inChangedFiles: boolean
    }[]
    truncated: boolean
  }
  stale?: boolean
  truncated?: boolean
  headCommit?: string
}

// Why: titleKey templates live in the backend; unknown keys fall back to the rule id
// so a backend addition never breaks the mobile summary.
const TITLE_TEMPLATES: Record<string, string> = {
  'finding.layer_violation.title': '{from} depends on {to}',
  'finding.dependency_cycle.title': 'Dependency cycle: {members}',
  'finding.hotspot.title': 'Hotspot: {subject}',
  'finding.missing_tenant_id.title': 'Missing tenant id: {subject}',
  'finding.dead_code.title': 'Unused code: {subject}',
  'finding.rls_removed.title': 'Row-level security removed: {subject}'
}

const RISK_LEVELS = new Set(['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'])
const NO_BINDING_CODES = new Set([
  'CODEINTEL_NO_BINDING',
  'CODEINTEL_REPO_NOT_REGISTERED',
  'CODEINTEL_NOT_BOUND'
])

// Why: free-text from the index may carry secrets or absolute paths; the phone
// must never receive them (O-16 tracks the shared masker this stands in for).
export function maskReviewSummaryText(text: string, max: number): string {
  return text
    .replace(/\b(?:sk|ghp|gho|xox[abp])[-_A-Za-z0-9]{16,}/g, '[redacted]')
    .replace(/(?:[A-Za-z]:\\|\/(?:home|Users|root|tmp|var|opt)\/)[^\s'"`]*/g, (match) => {
      const parts = match.split(/[\\/]/).filter(Boolean)
      return parts.at(-1) ?? '[path]'
    })
    .slice(0, max)
}

function renderTemplate(template: string, params: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (_m, key: string) => params[key] ?? '')
}

function buildTitle(finding: CodeIntelSummaryPortFinding): string {
  const template = TITLE_TEMPLATES[finding.titleKey]
  const rendered = template ? renderTemplate(template, finding.params ?? {}).trim() : ''
  return maskReviewSummaryText(rendered || finding.rule, MAX_TITLE)
}

function safeRelativePath(path: string | undefined): string | undefined {
  if (!path || path.startsWith('/') || /^[A-Za-z]:[\\/]/.test(path) || path.includes('..')) {
    return undefined
  }
  return path.slice(0, 512)
}

function severityOf(value: string): 'error' | 'warning' | 'info' {
  return value === 'error' || value === 'warning' ? value : 'info'
}

function originOf(value: string): 'introduced' | 'touched' | 'preexisting' | 'unknown' {
  return value === 'introduced' || value === 'touched' || value === 'preexisting'
    ? value
    : 'unknown'
}

function indexStateOf(overall: string): string {
  switch (overall) {
    case 'READY':
    case 'OVERLAY':
      return 'ready'
    case 'STALE':
    case 'DEGRADED':
      return 'stale'
    case 'BUILDING':
      return 'building'
    default:
      return 'missing'
  }
}

function riskReasonText(reason: { messageKey: string; params?: Record<string, string> }): string {
  const params = reason.params ?? {}
  const detail = Object.values(params).join(', ')
  return maskReviewSummaryText(detail ? `${reason.messageKey}: ${detail}` : reason.messageKey, 300)
}

export function buildMobileReviewSummary(input: {
  overlay: CodeIntelSummaryPortOverlay
  findings: CodeIntelSummaryPortFindings
  status: CodeIntelSummaryPortStatus
}): MobileReviewSummaryWire {
  const { overlay, findings, status } = input
  const changed = new Set(overlay.changedFiles.map((file) => file.path))
  const totals = overlay.limits?.totalCounts ?? {}
  const count = (key: string, fallback: number) => totals[key] ?? fallback
  const all = findings.findings
  const items = all.slice(0, REVIEW_SUMMARY_MAX_ITEMS).map((finding) => {
    const first = finding.evidence?.[0]
    const filePath = safeRelativePath(first?.path)
    return {
      key: finding.findingKey,
      kind: finding.kind,
      severity: severityOf(finding.severity),
      title: buildTitle(finding),
      summary: maskReviewSummaryText(finding.rule, MAX_SUMMARY),
      origin: originOf(finding.origin),
      filePath,
      startLine: filePath ? first?.line : undefined,
      inChangedFiles: filePath !== undefined && changed.has(filePath)
    }
  })
  const bySeverity = { error: 0, warning: 0, info: 0 }
  for (const finding of all) {
    bySeverity[severityOf(finding.severity)] += 1
  }
  const totalOpen = findings.totalCount ?? all.length
  const truncated = all.length > REVIEW_SUMMARY_MAX_ITEMS || totalOpen > items.length
  const freshness = overlay.indexFreshness
  const index = {
    state: indexStateOf(status.overall),
    indexedCommit: freshness?.indexedCommit,
    headCommit: freshness?.headOid,
    indexedAt: freshness?.generatedAt
  }
  return {
    available: true,
    index,
    counts: {
      files: count('files', overlay.changedFiles.length),
      symbols: count('symbols', overlay.changedSymbols?.length ?? 0),
      flows: count('flows', overlay.affectedFlows?.length ?? 0),
      tables: count('tables', overlay.touchedTables?.length ?? 0),
      contracts: count('contracts', overlay.touchedContracts?.length ?? 0),
      uncovered: count('uncovered', overlay.uncoveredSymbols?.length ?? 0)
    },
    risk: {
      level: RISK_LEVELS.has(overlay.risk.level) ? overlay.risk.level : 'UNKNOWN',
      reasons: overlay.risk.reasons.slice(0, 10).map(riskReasonText)
    },
    findings: { totalOpen, bySeverity, items, truncated },
    stale: index.state === 'stale' || freshness?.state === 'behind' ? true : undefined,
    truncated: truncated || undefined,
    headCommit: freshness?.headOid
  }
}

// Returns an unavailable payload for known "feature not usable" codes; rethrows others.
export function mapCodeIntelErrorToSummary(error: unknown): MobileReviewSummaryWire {
  const code = error instanceof CodeIntelPortError ? error.code : undefined
  if (code === 'CODEINTEL_DISABLED') {
    return { available: false, reason: 'flag_off' }
  }
  if (code && NO_BINDING_CODES.has(code)) {
    return { available: false, reason: 'no_binding' }
  }
  if (code === 'CODEINTEL_INDEX_MISSING') {
    return { available: false, reason: 'index_missing' }
  }
  if (code === 'CODEINTEL_TOOL_UNAVAILABLE') {
    return { available: false, reason: 'tool_unavailable' }
  }
  // Why: the default no-gateway port (O-4 open) is "unavailable", not an error.
  if (code === 'CODEINTEL_UNAVAILABLE') {
    return { available: false }
  }
  throw error
}
