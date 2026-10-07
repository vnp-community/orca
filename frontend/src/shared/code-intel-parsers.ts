/**
 * code-intel-parsers.ts — FE-CV-TASK-050-03
 *
 * Error codes, error kind classification, envelope parsing, and
 * push-event parsing for the code-intelligence protocol.
 *
 * Invariants:
 *  - Never throws (all parse paths have safe defaults)
 *  - JSON suffix >2 KiB or malformed → data: null
 *  - Unknown CODEINTEL_* codes kept as-is with kind 'unknown'
 *  - IndexStatus.overall values are uppercase (PQ-32)
 *
 * @module shared/code-intel-parsers
 */

import type {
  CodeIntelEnvelope,
  IndexStatus,
  IndexOverall,
  CodeIntelPushEvent
} from './code-intel-types'
import { CODE_INTEL_PUSH_EVENTS } from './code-intel-rpc-methods'

// ---------------------------------------------------------------------------
// §2.3 Error codes
// ---------------------------------------------------------------------------

export const CODE_INTEL_ERROR_CODES = {
  DISABLED: 'CODEINTEL_DISABLED',
  UNAVAILABLE: 'CODEINTEL_UNAVAILABLE',
  RATE_LIMITED: 'CODEINTEL_RATE_LIMITED',
  NOT_AUTHORIZED: 'CODEINTEL_NOT_AUTHORIZED',
  WORKTREE_NOT_FOUND: 'CODEINTEL_WORKTREE_NOT_FOUND',
  INDEX_STALE: 'CODEINTEL_INDEX_STALE',
  INDEX_NOT_READY: 'CODEINTEL_INDEX_NOT_READY',
  INDEX_ERROR: 'CODEINTEL_INDEX_ERROR',
  VALIDATION: 'CODEINTEL_VALIDATION',
  PAYLOAD_TOO_LARGE: 'CODEINTEL_PAYLOAD_TOO_LARGE',
  CONFLICT: 'CODEINTEL_CONFLICT',
  NOT_FOUND: 'CODEINTEL_NOT_FOUND',
  FINDING_ALREADY_WAIVED: 'CODEINTEL_FINDING_ALREADY_WAIVED',
  QUALITY_RUN_ALREADY_RUNNING: 'CODEINTEL_QUALITY_RUN_ALREADY_RUNNING',
  QUALITY_PROFILE_INVALID: 'CODEINTEL_QUALITY_PROFILE_INVALID',
  COVERAGE_NOT_AVAILABLE: 'CODEINTEL_COVERAGE_NOT_AVAILABLE',
  TREND_NOT_AVAILABLE: 'CODEINTEL_TREND_NOT_AVAILABLE',
  SECURITY_SCAN_ALREADY_RUNNING: 'CODEINTEL_SECURITY_SCAN_ALREADY_RUNNING',
  CONTRACT_DIFF_NOT_AVAILABLE: 'CODEINTEL_CONTRACT_DIFF_NOT_AVAILABLE',
  C4_INVALID: 'CODEINTEL_C4_INVALID',
  CHECKLIST_INVALID: 'CODEINTEL_CHECKLIST_INVALID',
  COMMENT_NOT_FOUND: 'CODEINTEL_COMMENT_NOT_FOUND',
  APPROVE_NOT_ALLOWED: 'CODEINTEL_APPROVE_NOT_ALLOWED',
  BIND_REPO_CONFLICT: 'CODEINTEL_BIND_REPO_CONFLICT',
  IMPACT_QUERY_TIMEOUT: 'CODEINTEL_IMPACT_QUERY_TIMEOUT',
  HOTSPOT_NOT_AVAILABLE: 'CODEINTEL_HOTSPOT_NOT_AVAILABLE',
} as const

export type CodeIntelErrorCode = (typeof CODE_INTEL_ERROR_CODES)[keyof typeof CODE_INTEL_ERROR_CODES]

/** Broad kind used for routing: what do we tell the UI? */
export type CodeIntelErrorKind =
  | 'unsupported'
  | 'disabled'
  | 'forbidden'
  | 'offline'
  | 'rate_limited'
  | 'not_found'
  | 'validation'
  | 'payload_too_large'
  | 'conflict'
  | 'stale'
  | 'not_ready'
  | 'quality_running'
  | 'security_running'
  | 'unknown'

/** 26 values — all members of CodeIntelErrorKind */
const ALL_ERROR_KINDS: CodeIntelErrorKind[] = [
  'unsupported', 'disabled', 'forbidden', 'offline', 'rate_limited',
  'not_found', 'validation', 'payload_too_large', 'conflict', 'stale',
  'not_ready', 'quality_running', 'security_running', 'unknown'
]
export { ALL_ERROR_KINDS }

const CODE_TO_KIND: Partial<Record<string, CodeIntelErrorKind>> = {
  [CODE_INTEL_ERROR_CODES.DISABLED]: 'disabled',
  [CODE_INTEL_ERROR_CODES.UNAVAILABLE]: 'offline',
  [CODE_INTEL_ERROR_CODES.RATE_LIMITED]: 'rate_limited',
  [CODE_INTEL_ERROR_CODES.NOT_AUTHORIZED]: 'forbidden',
  [CODE_INTEL_ERROR_CODES.WORKTREE_NOT_FOUND]: 'not_found',
  [CODE_INTEL_ERROR_CODES.INDEX_STALE]: 'stale',
  [CODE_INTEL_ERROR_CODES.INDEX_NOT_READY]: 'not_ready',
  [CODE_INTEL_ERROR_CODES.INDEX_ERROR]: 'unknown',
  [CODE_INTEL_ERROR_CODES.VALIDATION]: 'validation',
  [CODE_INTEL_ERROR_CODES.PAYLOAD_TOO_LARGE]: 'payload_too_large',
  [CODE_INTEL_ERROR_CODES.CONFLICT]: 'conflict',
  [CODE_INTEL_ERROR_CODES.NOT_FOUND]: 'not_found',
  [CODE_INTEL_ERROR_CODES.FINDING_ALREADY_WAIVED]: 'conflict',
  [CODE_INTEL_ERROR_CODES.QUALITY_RUN_ALREADY_RUNNING]: 'quality_running',
  [CODE_INTEL_ERROR_CODES.QUALITY_PROFILE_INVALID]: 'validation',
  [CODE_INTEL_ERROR_CODES.COVERAGE_NOT_AVAILABLE]: 'unknown',
  [CODE_INTEL_ERROR_CODES.TREND_NOT_AVAILABLE]: 'unknown',
  [CODE_INTEL_ERROR_CODES.SECURITY_SCAN_ALREADY_RUNNING]: 'security_running',
  [CODE_INTEL_ERROR_CODES.CONTRACT_DIFF_NOT_AVAILABLE]: 'unknown',
  [CODE_INTEL_ERROR_CODES.C4_INVALID]: 'validation',
  [CODE_INTEL_ERROR_CODES.CHECKLIST_INVALID]: 'validation',
  [CODE_INTEL_ERROR_CODES.COMMENT_NOT_FOUND]: 'not_found',
  [CODE_INTEL_ERROR_CODES.APPROVE_NOT_ALLOWED]: 'forbidden',
  [CODE_INTEL_ERROR_CODES.BIND_REPO_CONFLICT]: 'conflict',
  [CODE_INTEL_ERROR_CODES.IMPACT_QUERY_TIMEOUT]: 'unknown',
  [CODE_INTEL_ERROR_CODES.HOTSPOT_NOT_AVAILABLE]: 'unknown',
  // RPC-level codes
  method_not_found: 'unsupported',
  forbidden: 'forbidden',
  // connectivity
  connection_refused: 'offline',
  timeout: 'offline',
  network_error: 'offline',
}

export const CODE_INTEL_ERROR_KIND_BY_CODE: Readonly<Record<string, CodeIntelErrorKind>> =
  CODE_TO_KIND as Record<string, CodeIntelErrorKind>

// ---------------------------------------------------------------------------
// §2.3 Error message parser
// ---------------------------------------------------------------------------

const MAX_DATA_BYTES = 2 * 1024 // 2 KiB cap

export type ParsedCodeIntelError = {
  code: string | null
  text: string
  data: Record<string, unknown> | null
}

/**
 * Parse a code-intel error message that may embed a JSON suffix.
 * Format: "human text {\"json\":\"payload\"}"
 * Never throws.
 */
export function parseCodeIntelErrorMessage(message: string): ParsedCodeIntelError {
  const jsonStart = message.lastIndexOf('{')
  if (jsonStart === -1) {
    return { code: null, text: message.trim(), data: null }
  }

  const jsonPart = message.slice(jsonStart)
  const textPart = message.slice(0, jsonStart).trim()

  // Guard: skip parsing oversized payloads
  if (new TextEncoder().encode(jsonPart).length > MAX_DATA_BYTES) {
    return { code: null, text: textPart || message.trim(), data: null }
  }

  try {
    const parsed = JSON.parse(jsonPart)
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      return { code: null, text: textPart || message.trim(), data: null }
    }
    const code = typeof parsed.code === 'string' ? parsed.code : null
    return { code, text: textPart || message.trim(), data: parsed as Record<string, unknown> }
  } catch {
    return { code: null, text: textPart || message.trim(), data: null }
  }
}

// ---------------------------------------------------------------------------
// §4.2 Envelope parser
// ---------------------------------------------------------------------------

/**
 * Parse a raw code-intel envelope safely.
 * Unknown/missing fields use safe defaults (stale:false, truncated:false, etc.)
 */
export function parseCodeIntelEnvelope<T>(
  raw: unknown,
  parseData: (raw: unknown) => T
): CodeIntelEnvelope<T> {
  if (typeof raw !== 'object' || raw === null) {
    throw new Error('[code-intel] unexpected non-object envelope')
  }
  const r = raw as Record<string, unknown>

  return {
    worktreeId: typeof r.worktreeId === 'string' ? r.worktreeId : '',
    view: typeof r.view === 'string' ? r.view : '',
    sources: Array.isArray(r.sources) ? (r.sources as string[]) : [],
    headCommit: typeof r.headCommit === 'string' ? r.headCommit : null,
    stale: r.stale === true,
    truncated: r.truncated === true,
    totalCount: typeof r.totalCount === 'number' ? r.totalCount : 0,
    etag: typeof r.etag === 'string' ? r.etag : null,
    notModified: r.notModified === true,
    data: r.notModified ? undefined : parseData(r.data)
  }
}

// ---------------------------------------------------------------------------
// §4.3 IndexStatus parser
// ---------------------------------------------------------------------------

const VALID_OVERALL: Set<string> = new Set(['READY', 'INDEXING', 'PARTIAL', 'ERROR', 'UNKNOWN'])

export function parseIndexStatus(raw: unknown): IndexStatus {
  if (typeof raw !== 'object' || raw === null) {
    return {
      worktreeId: '',
      overall: 'UNKNOWN',
      lastIndexedAt: null,
      fileCoverage: 0,
      linesIndexed: 0,
      running: false,
      percent: null,
      error: null
    }
  }
  const r = raw as Record<string, unknown>
  const rawOverall = typeof r.overall === 'string' ? r.overall.toUpperCase() : ''
  const overall: IndexOverall = VALID_OVERALL.has(rawOverall)
    ? (rawOverall as IndexOverall)
    : 'UNKNOWN'

  return {
    worktreeId: typeof r.worktreeId === 'string' ? r.worktreeId : '',
    overall,
    lastIndexedAt: typeof r.lastIndexedAt === 'string' ? r.lastIndexedAt : null,
    fileCoverage: typeof r.fileCoverage === 'number' ? r.fileCoverage : 0,
    linesIndexed: typeof r.linesIndexed === 'number' ? r.linesIndexed : 0,
    running: r.running === true,
    percent: typeof r.percent === 'number' ? r.percent : null,
    error: typeof r.error === 'string' ? r.error : null
  }
}

// ---------------------------------------------------------------------------
// §5 Push event parser
// ---------------------------------------------------------------------------

const PUSH_EVENT_NAMES = new Set<string>(CODE_INTEL_PUSH_EVENTS)

/**
 * Parse a raw push frame.
 * Returns null for unknown/malformed events (caller should discard).
 */
export function parseCodeIntelPushEvent(raw: unknown): CodeIntelPushEvent | null {
  if (typeof raw !== 'object' || raw === null) return null
  const r = raw as Record<string, unknown>
  const eventName = r.event
  if (typeof eventName !== 'string' || !PUSH_EVENT_NAMES.has(eventName)) return null

  switch (eventName) {
    case 'changed':
      return {
        event: 'changed',
        worktreeId: String(r.worktreeId ?? ''),
        reason: typeof r.reason === 'string' ? (r.reason as 'commit' | 'file_save' | 'branch_switch' | 'reindex' | 'manual') : 'unknown',
        resync: r.resync === true
      }
    case 'reindexProgress':
      return {
        event: 'reindexProgress',
        worktreeId: String(r.worktreeId ?? ''),
        percent: typeof r.percent === 'number' ? r.percent : null,
        running: r.running === true
      }
    case 'qualityProgress':
      return {
        event: 'qualityProgress',
        worktreeId: String(r.worktreeId ?? ''),
        runId: String(r.runId ?? ''),
        percent: typeof r.percent === 'number' ? r.percent : null,
        phase: typeof r.phase === 'string' ? (r.phase as 'collect' | 'analyze' | 'report') : 'unknown'
      }
    case 'qualityFinished':
      return {
        event: 'qualityFinished',
        worktreeId: String(r.worktreeId ?? ''),
        runId: String(r.runId ?? ''),
        success: r.success === true,
        error: typeof r.error === 'string' ? r.error : null
      }
    case 'gateChanged':
      return {
        event: 'gateChanged',
        worktreeId: String(r.worktreeId ?? ''),
        gate: typeof r.gate === 'string' ? (r.gate as 'pass' | 'warn' | 'fail' | 'unknown') : 'unknown'
      }
    default:
      return null
  }
}
