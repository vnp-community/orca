/**
 * review-shell-data.ts
 *
 * Wire access for the Review shell: tolerant normalizers (backend may omit arrays), error
 * classification from CODEINTEL_* codes, and the ReviewDataApi seam.
 * Why a seam: the SOL-050 client/method constants are still settling; tests and the
 * reading-progress slice inject a fake instead of depending on them.
 */

import type {
  ChangeOverlayView,
  IndexOverall,
  IndexStatusView,
  ReadingProgress,
  ReviewError,
  ReviewErrorKind,
  ReviewStateView
} from './review-wire-types'
import { emptyReadingProgress } from './review-wire-types'

// ---------------------------------------------------------------------------
// Normalizers
// ---------------------------------------------------------------------------

const OVERALLS: readonly IndexOverall[] = [
  'OFFLINE',
  'UNKNOWN',
  'NOT_INSTALLED',
  'BUILDING',
  'MISSING',
  'DEGRADED',
  'OVERLAY',
  'STALE',
  'READY'
]

function rec(v: unknown): Record<string, unknown> {
  return typeof v === 'object' && v !== null ? (v as Record<string, unknown>) : {}
}
function arr<T = unknown>(v: unknown): T[] {
  return Array.isArray(v) ? (v as T[]) : []
}

/** Unknown `overall` never pretends to be READY. */
export function normalizeIndexStatus(raw: unknown): IndexStatusView {
  const r = rec(raw)
  const overall = OVERALLS.includes(r.overall as IndexOverall)
    ? (r.overall as IndexOverall)
    : 'UNKNOWN'
  const job = rec(r.activeJob)
  return {
    overall,
    tools: arr(r.tools),
    scopeMismatch: r.scopeMismatch === true,
    activeJob:
      typeof job.id === 'string'
        ? {
            id: job.id,
            stage: typeof job.stage === 'string' ? job.stage : '',
            percent: typeof job.percent === 'number' ? job.percent : null
          }
        : undefined,
    indexBasis: arr(r.indexBasis),
    lastStatusAt: typeof r.lastStatusAt === 'string' ? r.lastStatusAt : undefined,
    errorCode: typeof r.errorCode === 'string' ? r.errorCode : undefined
  }
}

/** Accepts either the envelope ({data}) or the bare overlay. */
export function normalizeChangeOverlay(raw: unknown): ChangeOverlayView {
  const envelope = rec(raw)
  const r = 'data' in envelope && envelope.data ? rec(envelope.data) : envelope
  const limits = rec(r.limits)
  const risk = rec(r.risk)
  return {
    scope: r.scope ? (rec(r.scope) as unknown as ChangeOverlayView['scope']) : null,
    emptyReason: typeof r.emptyReason === 'string' ? r.emptyReason : undefined,
    changedFiles: arr(r.changedFiles),
    changedSymbols: arr(r.changedSymbols),
    affectedFlows: arr(r.affectedFlows),
    touchedTables: arr(r.touchedTables),
    touchedContracts: arr(r.touchedContracts),
    uncoveredSymbols: arr(r.uncoveredSymbols),
    violations: arr(r.violations),
    readingOrder: arr(r.readingOrder),
    components: arr(r.components),
    risk:
      typeof risk.level === 'string'
        ? {
            level: risk.level as 'LOW',
            score: typeof risk.score === 'number' ? risk.score : undefined,
            incomplete: risk.incomplete === true,
            confidence: typeof risk.confidence === 'string' ? risk.confidence : undefined,
            reasons: arr(risk.reasons)
          }
        : null,
    indexFreshness: r.indexFreshness ? (rec(r.indexFreshness) as never) : null,
    limits: {
      truncated: rec(limits.truncated) as Record<string, boolean>,
      totalCounts: rec(limits.totalCounts) as Record<string, number>
    }
  }
}

export function normalizeReviewState(raw: unknown, base: string, head: string): ReviewStateView {
  const r = rec(raw)
  const rp = rec(r.readingProgress)
  const entries: ReadingProgress['entries'] = {}
  for (const [k, v] of Object.entries(rec(rp.entries))) {
    const e = rec(v)
    if ((e.state === 'seen' || e.state === 'unseen') && typeof e.at === 'number') {
      entries[k] = { state: e.state, at: e.at }
    }
  }
  return {
    ...r,
    baseCommit: typeof r.baseCommit === 'string' ? r.baseCommit : base,
    headCommit: typeof r.headCommit === 'string' ? r.headCommit : head,
    readingProgress: {
      ...emptyReadingProgress(),
      entries,
      lastFocusedKey: typeof rp.lastFocusedKey === 'string' ? rp.lastFocusedKey : null
    },
    version: typeof r.version === 'number' ? r.version : 0
  }
}

// ---------------------------------------------------------------------------
// Error classification (§2.3)
// ---------------------------------------------------------------------------

const NO_BINDING_CODES = [
  'CODEINTEL_WORKTREE_NOT_FOUND',
  'CODEINTEL_NO_DEV_SERVER',
  'CODEINTEL_DEV_SERVER_NOT_APPROVED',
  'CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED',
  'CODEINTEL_WORKTREE_REF_UNSUPPORTED'
]

const KIND_BY_CODE: Record<string, ReviewErrorKind> = {
  CODEINTEL_DISABLED: 'disabled',
  CODEINTEL_UNAVAILABLE: 'unsupported',
  CODEINTEL_AGENT_UNSUPPORTED: 'unsupported',
  CODEINTEL_NOT_AUTHORIZED: 'forbidden',
  CODEINTEL_PATH_NOT_ALLOWED: 'path-not-allowed',
  CODEINTEL_DEV_SERVER_OFFLINE: 'offline',
  CODEINTEL_TOOL_UNAVAILABLE: 'tool-unavailable',
  CODEINTEL_INDEX_MISSING: 'index-missing',
  CODEINTEL_REPO_NOT_REGISTERED: 'repo-not-registered',
  CODEINTEL_TIMEOUT: 'timeout',
  CODEINTEL_REINDEX_COOLDOWN: 'rate-limited',
  CODEINTEL_RATE_LIMITED: 'rate-limited',
  CODEINTEL_CONCURRENCY_LIMIT: 'rate-limited',
  CODEINTEL_OUTPUT_TOO_LARGE: 'too-large',
  CODEINTEL_RESPONSE_TOO_LARGE: 'too-large',
  CODEINTEL_PAYLOAD_TOO_LARGE: 'too-large',
  CODEINTEL_TOOL_FAILED: 'tool-failed',
  CODEINTEL_RESULT_INVALID: 'tool-failed',
  CODEINTEL_VERSION_CONFLICT: 'conflict',
  CODEINTEL_REINDEX_IN_PROGRESS: 'reindex-in-progress'
}

const CODE_PATTERN = /CODEINTEL_[A-Z0-9_]+/

/** Accepts the 050 client error (kind/code/message/data) or a bare Error. */
export function classifyReviewError(err: unknown): ReviewError {
  const e = rec(err)
  const message = typeof e.message === 'string' ? e.message : 'Unknown error'
  const code =
    (typeof e.code === 'string' && CODE_PATTERN.test(e.code) ? e.code : null) ??
    message.match(CODE_PATTERN)?.[0] ??
    null
  const data =
    typeof e.data === 'object' && e.data !== null ? (e.data as Record<string, unknown>) : null
  let kind: ReviewErrorKind = 'unknown'
  if (code && NO_BINDING_CODES.includes(code)) {
    kind = 'no-binding'
  } else if (code && KIND_BY_CODE[code]) {
    kind = KIND_BY_CODE[code]
  } else if (e.kind === 'offline') {
    kind = 'offline'
  } else if (e.kind === 'forbidden') {
    kind = 'forbidden'
  } else if (e.kind === 'rate-limited') {
    kind = 'rate-limited'
  } else if (e.kind === 'conflict') {
    kind = 'conflict'
  } else if (e.kind === 'unsupported') {
    kind = 'unsupported'
  } else if (e.kind === 'disabled') {
    kind = 'disabled'
  }
  const retry =
    data && typeof data.retryAfterSeconds === 'number' ? data.retryAfterSeconds : undefined
  return { kind, message, retryAfterSeconds: retry, data }
}

// ---------------------------------------------------------------------------
// API seam
// ---------------------------------------------------------------------------

export type ReviewApiResult<T> = { ok: true; value: T } | { ok: false; error: ReviewError }

export type ReviewDataApi = {
  getStatus(
    worktreeId: string,
    opts?: { refresh?: boolean; signal?: AbortSignal }
  ): Promise<ReviewApiResult<IndexStatusView>>
  reindex(
    worktreeId: string,
    mode: 'incremental' | 'full'
  ): Promise<ReviewApiResult<{ jobId: string; status: string }>>
  bindRepo(worktreeId: string): Promise<ReviewApiResult<IndexStatusView>>
  getChangeOverlay(
    worktreeId: string,
    params: { base?: string; head?: string; mode: 'worktree' | 'committed' },
    signal?: AbortSignal
  ): Promise<ReviewApiResult<ChangeOverlayView>>
  getReviewState(
    worktreeId: string,
    key: { baseCommit: string; headCommit: string }
  ): Promise<ReviewApiResult<ReviewStateView>>
  saveReviewState(
    worktreeId: string,
    state: ReviewStateView,
    expectedVersion: number
  ): Promise<ReviewApiResult<ReviewStateView>>
}
