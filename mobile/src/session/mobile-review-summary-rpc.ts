// Why: mobile declares the contract locally (no zod, no vendor-shared) so a host
// payload drift is absorbed by this one parser; see CONTRACT-codeintel-ui-api §8.
export type MobileReviewSummaryReason =
  | 'flag_off'
  | 'no_binding'
  | 'index_missing'
  | 'tool_unavailable'

export type MobileReviewSummaryRiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'UNKNOWN'
export type MobileReviewSummarySeverity = 'error' | 'warning' | 'info'
export type MobileReviewSummaryOrigin = 'introduced' | 'touched' | 'preexisting' | 'unknown'
export type MobileReviewSummaryIndexState = 'missing' | 'building' | 'ready' | 'stale'

export type MobileReviewSummaryFinding = {
  key: string
  kind: string
  severity: MobileReviewSummarySeverity
  title: string
  summary: string
  origin: MobileReviewSummaryOrigin
  filePath?: string
  startLine?: number
  inChangedFiles: boolean
}

export type MobileReviewSummary = {
  available: boolean
  reason?: MobileReviewSummaryReason
  index?: {
    state: MobileReviewSummaryIndexState
    indexedCommit?: string
    headCommit?: string
    indexedAt?: string
  }
  counts?: {
    files: number
    symbols: number
    flows: number
    tables: number
    contracts: number
    uncovered: number
  }
  risk?: { level: MobileReviewSummaryRiskLevel; reasons: string[] }
  findings?: {
    totalOpen: number
    bySeverity: { error: number; warning: number; info: number }
    items: MobileReviewSummaryFinding[]
    truncated: boolean
  }
  stale?: boolean
  truncated?: boolean
  headCommit?: string
}

export const MOBILE_REVIEW_SUMMARY_MAX_ITEMS = 50
const MAX_TITLE = 200
const MAX_SUMMARY = 400
const MAX_REASONS = 10
const MAX_REASON_LENGTH = 300
const MAX_SHORT = 200

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function readString(value: unknown, max = MAX_SHORT): string | undefined {
  return typeof value === 'string' ? value.slice(0, max) : undefined
}

function readCount(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0
}

function readReason(value: unknown): MobileReviewSummaryReason | undefined {
  return value === 'flag_off' ||
    value === 'no_binding' ||
    value === 'index_missing' ||
    value === 'tool_unavailable'
    ? value
    : undefined
}

function readRiskLevel(value: unknown): MobileReviewSummaryRiskLevel {
  return value === 'LOW' || value === 'MEDIUM' || value === 'HIGH' || value === 'CRITICAL'
    ? value
    : 'UNKNOWN'
}

function readSeverity(value: unknown): MobileReviewSummarySeverity {
  return value === 'error' || value === 'warning' ? value : 'info'
}

function readOrigin(value: unknown): MobileReviewSummaryOrigin {
  return value === 'introduced' || value === 'touched' || value === 'preexisting'
    ? value
    : 'unknown'
}

function readIndex(value: unknown): MobileReviewSummary['index'] {
  if (!isRecord(value)) {
    return undefined
  }
  const state = value.state
  if (state !== 'missing' && state !== 'building' && state !== 'ready' && state !== 'stale') {
    return undefined
  }
  return {
    state,
    indexedCommit: readString(value.indexedCommit),
    headCommit: readString(value.headCommit),
    indexedAt: readString(value.indexedAt)
  }
}

function readCounts(value: unknown): MobileReviewSummary['counts'] {
  if (!isRecord(value)) {
    return undefined
  }
  return {
    files: readCount(value.files),
    symbols: readCount(value.symbols),
    flows: readCount(value.flows),
    tables: readCount(value.tables),
    contracts: readCount(value.contracts),
    uncovered: readCount(value.uncovered)
  }
}

function readRisk(value: unknown): MobileReviewSummary['risk'] {
  if (!isRecord(value)) {
    return undefined
  }
  const reasons = Array.isArray(value.reasons)
    ? value.reasons
        .map((reason) => readString(reason, MAX_REASON_LENGTH))
        .filter((reason): reason is string => reason !== undefined)
        .slice(0, MAX_REASONS)
    : []
  return { level: readRiskLevel(value.level), reasons }
}

function readFinding(value: unknown): MobileReviewSummaryFinding | null {
  if (!isRecord(value)) {
    return null
  }
  const key = readString(value.key)
  if (!key) {
    return null
  }
  const startLine = readCount(value.startLine)
  return {
    key,
    kind: readString(value.kind) ?? '',
    severity: readSeverity(value.severity),
    title: readString(value.title, MAX_TITLE) ?? '',
    summary: readString(value.summary, MAX_SUMMARY) ?? '',
    origin: readOrigin(value.origin),
    filePath: readString(value.filePath, 512),
    startLine: startLine > 0 ? startLine : undefined,
    inChangedFiles: value.inChangedFiles === true
  }
}

function readFindings(value: unknown): MobileReviewSummary['findings'] {
  if (!isRecord(value)) {
    return undefined
  }
  const rawItems = Array.isArray(value.items) ? value.items : []
  const items = rawItems
    .slice(0, MOBILE_REVIEW_SUMMARY_MAX_ITEMS)
    .map(readFinding)
    .filter((item): item is MobileReviewSummaryFinding => item !== null)
  const by = isRecord(value.bySeverity) ? value.bySeverity : {}
  return {
    totalOpen: readCount(value.totalOpen),
    // Why: high/medium/low from the original CR are not part of the contract (PQ-06).
    bySeverity: {
      error: readCount(by.error),
      warning: readCount(by.warning),
      info: readCount(by.info)
    },
    items,
    truncated: value.truncated === true || rawItems.length > MOBILE_REVIEW_SUMMARY_MAX_ITEMS
  }
}

export function readMobileReviewSummaryResult(value: unknown): MobileReviewSummary | null {
  if (!isRecord(value) || typeof value.available !== 'boolean') {
    return null
  }
  return {
    available: value.available,
    reason: readReason(value.reason),
    index: readIndex(value.index),
    counts: readCounts(value.counts),
    risk: readRisk(value.risk),
    findings: readFindings(value.findings),
    stale: value.stale === true ? true : undefined,
    truncated: value.truncated === true ? true : undefined,
    headCommit: readString(value.headCommit)
  }
}
