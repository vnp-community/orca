/**
 * code-intel-error-codes.ts — FE-CV-TASK-050-03
 *
 * CONTRACT-codeintel-ui-api §2.3: error codes, client error kinds and the error-message parser.
 * Re-exported from code-intel-parsers.ts.
 *
 * @module shared/code-intel-error-codes
 */

// ---------------------------------------------------------------------------
// §2.3 Error codes
// ---------------------------------------------------------------------------

export const CODE_INTEL_ERROR_CODES = {
  DISABLED: 'CODEINTEL_DISABLED',
  QUALITY_GATE_DISABLED: 'CODEINTEL_QUALITY_GATE_DISABLED',
  AI_REVIEW_DISABLED: 'CODEINTEL_AI_REVIEW_DISABLED',
  UNAVAILABLE: 'CODEINTEL_UNAVAILABLE',
  NOT_AUTHORIZED: 'CODEINTEL_NOT_AUTHORIZED',
  NOT_FOUND: 'CODEINTEL_NOT_FOUND',
  INVALID_PARAMS: 'CODEINTEL_INVALID_PARAMS',
  PATH_NOT_ALLOWED: 'CODEINTEL_PATH_NOT_ALLOWED',
  WORKTREE_NOT_FOUND: 'CODEINTEL_WORKTREE_NOT_FOUND',
  WORKTREE_REF_UNSUPPORTED: 'CODEINTEL_WORKTREE_REF_UNSUPPORTED',
  NO_DEV_SERVER: 'CODEINTEL_NO_DEV_SERVER',
  DEV_SERVER_NOT_APPROVED: 'CODEINTEL_DEV_SERVER_NOT_APPROVED',
  DEV_SERVER_MODE_UNSUPPORTED: 'CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED',
  DEV_SERVER_OFFLINE: 'CODEINTEL_DEV_SERVER_OFFLINE',
  TOOL_UNAVAILABLE: 'CODEINTEL_TOOL_UNAVAILABLE',
  INDEX_MISSING: 'CODEINTEL_INDEX_MISSING',
  REPO_NOT_REGISTERED: 'CODEINTEL_REPO_NOT_REGISTERED',
  AMBIGUOUS_SYMBOL: 'CODEINTEL_AMBIGUOUS_SYMBOL',
  SYMBOL_NOT_FOUND: 'CODEINTEL_SYMBOL_NOT_FOUND',
  TIMEOUT: 'CODEINTEL_TIMEOUT',
  REINDEX_IN_PROGRESS: 'CODEINTEL_REINDEX_IN_PROGRESS',
  REINDEX_COOLDOWN: 'CODEINTEL_REINDEX_COOLDOWN',
  OUTPUT_TOO_LARGE: 'CODEINTEL_OUTPUT_TOO_LARGE',
  RESPONSE_TOO_LARGE: 'CODEINTEL_RESPONSE_TOO_LARGE',
  TOOL_FAILED: 'CODEINTEL_TOOL_FAILED',
  RESULT_INVALID: 'CODEINTEL_RESULT_INVALID',
  AGENT_UNSUPPORTED: 'CODEINTEL_AGENT_UNSUPPORTED',
  RATE_LIMITED: 'CODEINTEL_RATE_LIMITED',
  CONCURRENCY_LIMIT: 'CODEINTEL_CONCURRENCY_LIMIT',
  VERSION_CONFLICT: 'CODEINTEL_VERSION_CONFLICT',
  PAYLOAD_TOO_LARGE: 'CODEINTEL_PAYLOAD_TOO_LARGE',
  PROFILE_INVALID: 'CODEINTEL_PROFILE_INVALID',
  PROFILE_UNKNOWN: 'CODEINTEL_PROFILE_UNKNOWN',
  ENV_NOT_READY: 'CODEINTEL_ENV_NOT_READY',
  RUN_IN_PROGRESS: 'CODEINTEL_RUN_IN_PROGRESS',
  RUN_NOT_FOUND: 'CODEINTEL_RUN_NOT_FOUND',
  RUN_CANCELLED: 'CODEINTEL_RUN_CANCELLED',
  WAIVER_EXPIRY_INVALID: 'CODEINTEL_WAIVER_EXPIRY_INVALID',
  AI_NO_RELAY: 'CODEINTEL_AI_NO_RELAY',
  AI_BAD_OUTPUT: 'CODEINTEL_AI_BAD_OUTPUT',
  SECRET_LEAK_BLOCKED: 'CODEINTEL_SECRET_LEAK_BLOCKED',
  AUTHZ_UNAVAILABLE: 'CODEINTEL_AUTHZ_UNAVAILABLE',
  INTERNAL: 'CODEINTEL_INTERNAL'
} as const

export type CodeIntelErrorCode = (typeof CODE_INTEL_ERROR_CODES)[keyof typeof CODE_INTEL_ERROR_CODES]

// ---------------------------------------------------------------------------
// §2.3 Client error kinds
// ---------------------------------------------------------------------------

/** Every client `kind` in the §2.3 table, plus 'unknown' (infrastructure / unmapped). */
export const ALL_ERROR_KINDS = [
  'disabled',
  'quality-disabled',
  'ai-disabled',
  'unsupported',
  'offline',
  'forbidden',
  'not-found',
  'validation',
  'path-not-allowed',
  'no-binding',
  'tool-unavailable',
  'index-missing',
  'repo-not-registered',
  'ambiguous',
  'timeout',
  'reindex-in-progress',
  'rate-limited',
  'too-large',
  'tool-failed',
  'conflict',
  'profile-unknown',
  'env-not-ready',
  'run-in-progress',
  'run-cancelled',
  'ai-error',
  'unknown'
] as const

export type CodeIntelErrorKind = (typeof ALL_ERROR_KINDS)[number]

const CODE_TO_KIND: Record<string, CodeIntelErrorKind> = {
  [CODE_INTEL_ERROR_CODES.DISABLED]: 'disabled',
  [CODE_INTEL_ERROR_CODES.QUALITY_GATE_DISABLED]: 'quality-disabled',
  [CODE_INTEL_ERROR_CODES.AI_REVIEW_DISABLED]: 'ai-disabled',
  [CODE_INTEL_ERROR_CODES.UNAVAILABLE]: 'unsupported',
  [CODE_INTEL_ERROR_CODES.NOT_AUTHORIZED]: 'forbidden',
  [CODE_INTEL_ERROR_CODES.NOT_FOUND]: 'not-found',
  [CODE_INTEL_ERROR_CODES.INVALID_PARAMS]: 'validation',
  [CODE_INTEL_ERROR_CODES.PATH_NOT_ALLOWED]: 'path-not-allowed',
  [CODE_INTEL_ERROR_CODES.WORKTREE_NOT_FOUND]: 'no-binding',
  [CODE_INTEL_ERROR_CODES.WORKTREE_REF_UNSUPPORTED]: 'no-binding',
  [CODE_INTEL_ERROR_CODES.NO_DEV_SERVER]: 'no-binding',
  [CODE_INTEL_ERROR_CODES.DEV_SERVER_NOT_APPROVED]: 'no-binding',
  [CODE_INTEL_ERROR_CODES.DEV_SERVER_MODE_UNSUPPORTED]: 'no-binding',
  [CODE_INTEL_ERROR_CODES.DEV_SERVER_OFFLINE]: 'offline',
  [CODE_INTEL_ERROR_CODES.TOOL_UNAVAILABLE]: 'tool-unavailable',
  [CODE_INTEL_ERROR_CODES.INDEX_MISSING]: 'index-missing',
  [CODE_INTEL_ERROR_CODES.REPO_NOT_REGISTERED]: 'repo-not-registered',
  [CODE_INTEL_ERROR_CODES.AMBIGUOUS_SYMBOL]: 'ambiguous',
  [CODE_INTEL_ERROR_CODES.SYMBOL_NOT_FOUND]: 'not-found',
  [CODE_INTEL_ERROR_CODES.TIMEOUT]: 'timeout',
  [CODE_INTEL_ERROR_CODES.REINDEX_IN_PROGRESS]: 'reindex-in-progress',
  [CODE_INTEL_ERROR_CODES.REINDEX_COOLDOWN]: 'rate-limited',
  [CODE_INTEL_ERROR_CODES.OUTPUT_TOO_LARGE]: 'too-large',
  [CODE_INTEL_ERROR_CODES.RESPONSE_TOO_LARGE]: 'too-large',
  [CODE_INTEL_ERROR_CODES.TOOL_FAILED]: 'tool-failed',
  [CODE_INTEL_ERROR_CODES.RESULT_INVALID]: 'tool-failed',
  [CODE_INTEL_ERROR_CODES.AGENT_UNSUPPORTED]: 'unsupported',
  [CODE_INTEL_ERROR_CODES.RATE_LIMITED]: 'rate-limited',
  [CODE_INTEL_ERROR_CODES.CONCURRENCY_LIMIT]: 'rate-limited',
  [CODE_INTEL_ERROR_CODES.VERSION_CONFLICT]: 'conflict',
  [CODE_INTEL_ERROR_CODES.PAYLOAD_TOO_LARGE]: 'validation',
  [CODE_INTEL_ERROR_CODES.PROFILE_INVALID]: 'validation',
  [CODE_INTEL_ERROR_CODES.PROFILE_UNKNOWN]: 'profile-unknown',
  [CODE_INTEL_ERROR_CODES.ENV_NOT_READY]: 'env-not-ready',
  [CODE_INTEL_ERROR_CODES.RUN_IN_PROGRESS]: 'run-in-progress',
  [CODE_INTEL_ERROR_CODES.RUN_NOT_FOUND]: 'not-found',
  [CODE_INTEL_ERROR_CODES.RUN_CANCELLED]: 'run-cancelled',
  [CODE_INTEL_ERROR_CODES.WAIVER_EXPIRY_INVALID]: 'validation',
  [CODE_INTEL_ERROR_CODES.AI_NO_RELAY]: 'ai-error',
  [CODE_INTEL_ERROR_CODES.AI_BAD_OUTPUT]: 'ai-error',
  // Never rendered raw: the view was blocked by the backend secret scan.
  [CODE_INTEL_ERROR_CODES.SECRET_LEAK_BLOCKED]: 'tool-failed',
  [CODE_INTEL_ERROR_CODES.AUTHZ_UNAVAILABLE]: 'unknown',
  [CODE_INTEL_ERROR_CODES.INTERNAL]: 'unknown',
  // RPC-level codes (no CODEINTEL_ prefix)
  method_not_found: 'unsupported',
  forbidden: 'forbidden',
  connection_refused: 'offline',
  timeout: 'offline',
  network_error: 'offline'
}

export const CODE_INTEL_ERROR_KIND_BY_CODE: Readonly<Record<string, CodeIntelErrorKind>> = CODE_TO_KIND

// ---------------------------------------------------------------------------
// §2.3 Error message parser
// ---------------------------------------------------------------------------

const MAX_DATA_BYTES = 2 * 1024
// Contract §2.3: ^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$
const ERROR_MESSAGE_PATTERN = /^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$/s

export type ParsedCodeIntelError = {
  code: string | null
  text: string
  data: Record<string, unknown> | null
}

function parseDataSuffix(json: string | undefined): Record<string, unknown> | null {
  if (!json || new TextEncoder().encode(json).length > MAX_DATA_BYTES) {
    return null
  }
  try {
    const parsed: unknown = JSON.parse(json)
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null
  } catch {
    return null
  }
}

/**
 * Parse `CODEINTEL_X: text | {json}`. Messages without the prefix keep code: null.
 * Never throws; malformed or oversized JSON yields data: null.
 */
export function parseCodeIntelErrorMessage(message: string): ParsedCodeIntelError {
  const trimmed = message.trim()
  const match = ERROR_MESSAGE_PATTERN.exec(trimmed)
  if (!match) {
    return { code: null, text: trimmed, data: null }
  }
  return { code: match[1], text: match[2].trim(), data: parseDataSuffix(match[3]) }
}

/** Kind for a (possibly unknown) code; unknown CODEINTEL_* codes are 'unknown'. */
export function codeIntelErrorKindForCode(code: string | null | undefined): CodeIntelErrorKind {
  return (code ? CODE_TO_KIND[code] : undefined) ?? 'unknown'
}
