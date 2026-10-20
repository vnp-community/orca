/**
 * Request RPC Error Classification — v6 (CR-REQ-018)
 *
 * Normalises the raw error thrown by callRuntimeRpc into a typed
 * RequestRpcError. Stays in shared/ and uses duck-typing on the error
 * object so this module never imports from renderer/src/runtime (which
 * would create a renderer→shared cross-boundary dep).
 *
 * Error codes sourced from CR-REQ-016 §2.8.
 *
 * @module shared/request-errors
 */

// ---------------------------------------------------------------------------
// Error kinds
// ---------------------------------------------------------------------------

export type RequestRpcErrorKind =
  | 'forbidden'
  | 'not_found'
  | 'conflict'
  | 'invalid_state'
  | 'blocking_questions'
  | 'unconfirmed_assumptions'
  | 'validation'
  | 'unsupported'
  | 'rate_limited'
  | 'unavailable'
  | 'expired'
  | 'pending'
  | 'no_dev_server'
  | 'no_ai'
  | 'feature_off'
  | 'network'
  | 'unknown'

export type RequestRpcError = {
  kind: RequestRpcErrorKind
  /** Transport code (e.g. `runtime_error`); the domain code is `backendCode`. */
  code: string
  message: string
  /** Domain code quoted in the message (`REQUEST_RATE_LIMITED: ...`); absent for uncoded errors. */
  backendCode?: string
  /** Wait the limiter asked for (`{"retryAfterMs":N}` message trailer), when the service sent one. */
  retryAfterMs?: number
  raw?: unknown
}

// ---------------------------------------------------------------------------
// Internal: split "CODE: message" error message format
// ---------------------------------------------------------------------------

const CODE_PREFIX_RE = /^([A-Z][A-Z0-9_]+):\s*(.*)/s

/** @internal */
export function splitErrorCode(message: string): { code: string; rest: string } {
  const m = CODE_PREFIX_RE.exec(message)
  if (m) {
    return { code: m[1], rest: m[2] }
  }
  return { code: '', rest: message }
}

// ---------------------------------------------------------------------------
// Code → kind mapping (CR-REQ-016 §2.8)
// ---------------------------------------------------------------------------

// Why: CONTRACT section 5 fixes the REQUEST_ prefix for every code, while CR-REQ-016 and
// older callers use the short form (APPROVAL_*, SOLUTION_*); both spellings stay mapped.
const CODE_TO_KIND: Record<string, RequestRpcErrorKind> = {
  // Authorisation
  REQUEST_FORBIDDEN: 'forbidden',
  APPROVAL_FORBIDDEN: 'forbidden',
  REQUEST_APPROVAL_FORBIDDEN: 'forbidden',
  REQUEST_SOLUTION_FORBIDDEN: 'forbidden',
  APPROVAL_NOT_APPROVER: 'forbidden',
  REQUEST_APPROVAL_NOT_APPROVER: 'forbidden',
  APPROVAL_SELF_APPROVAL_FORBIDDEN: 'forbidden',
  REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN: 'forbidden',
  APPROVAL_AGENT_FORBIDDEN: 'forbidden',
  REQUEST_APPROVAL_AGENT_FORBIDDEN: 'forbidden',
  REQUEST_EXECUTE_FORBIDDEN: 'forbidden',
  TASK_FORBIDDEN: 'forbidden',

  // Not found
  TASK_NOT_FOUND: 'not_found',
  REQUEST_NOT_FOUND: 'not_found',
  SOLUTION_NOT_FOUND: 'not_found',
  REQUEST_SOLUTION_NOT_FOUND: 'not_found',
  REQUEST_SOLUTION_REQUEST_NOT_FOUND: 'not_found',
  REQUEST_SOLUTION_OPTION_NOT_FOUND: 'not_found',
  APPROVAL_NOT_FOUND: 'not_found',
  REQUEST_APPROVAL_NOT_FOUND: 'not_found',
  REQUEST_PARENT_NOT_FOUND: 'not_found',
  REQUEST_SOURCE_NOT_FOUND: 'not_found',
  REQUEST_PROJECT_NOT_FOUND: 'not_found',

  // Conflict / stale
  REQUEST_STATE_STALE: 'conflict',
  APPROVAL_VERSION_CONFLICT: 'conflict',
  REQUEST_APPROVAL_VERSION_CONFLICT: 'conflict',
  APPROVAL_DIGEST_MISMATCH: 'conflict',
  REQUEST_APPROVAL_DIGEST_MISMATCH: 'conflict',
  APPROVAL_STAGE_MISMATCH: 'conflict',
  REQUEST_APPROVAL_STAGE_MISMATCH: 'conflict',
  REQUEST_VERSION_CONFLICT: 'conflict',
  SOLUTION_VERSION_CONFLICT: 'conflict',
  REQUEST_SOLUTION_VERSION_CONFLICT: 'conflict',

  // Invalid state transitions
  REQUEST_TRANSITION_NOT_ALLOWED: 'invalid_state',
  REQUEST_TYPE_CHANGE_NOT_ALLOWED: 'invalid_state',
  REQUEST_TYPE_CHANGE_USE_CHILD: 'invalid_state',
  REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION: 'invalid_state',
  REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION: 'invalid_state',
  REQUEST_REOPEN_NOT_ALLOWED: 'invalid_state',
  REQUEST_CANCEL_NOT_ALLOWED: 'invalid_state',
  REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION: 'invalid_state',
  REQUEST_CHILD_NOT_ALLOWED: 'invalid_state',
  REQUEST_CHILD_LIMIT: 'invalid_state',
  REQUEST_CHILD_DEPTH_EXCEEDED: 'invalid_state',
  REQUEST_PHASE_NOT_APPROVED: 'invalid_state',
  REQUEST_PHASE_PREDECESSOR_NOT_DONE: 'invalid_state',
  REQUEST_PHASE_NOT_IN_PLAN: 'invalid_state',
  REQUEST_NOT_EXECUTING: 'invalid_state',
  REQUEST_PLAN_NOT_APPLICABLE: 'invalid_state',
  REQUEST_PLAN_SOLUTION_NOT_APPROVED: 'invalid_state',
  // Plan versions (CR-REQ-045): a second reviewer is already revising; hotfix and unchanged-document refusals; reads of a version.
  REQUEST_PLAN_REVISION_IN_PROGRESS: 'conflict',
  REQUEST_PLAN_REVISION_NOT_APPLICABLE: 'invalid_state',
  REQUEST_PLAN_REVISION_NO_CHANGE: 'invalid_state',
  REQUEST_PLAN_REVISION_TEXT_PRUNED: 'invalid_state',
  REQUEST_PLAN_REVISION_NOT_FOUND: 'not_found',
  REQUEST_PLAN_BUNDLE_NOT_FOUND: 'not_found',
  REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND: 'not_found',
  SOLUTION_ALREADY_CHOSEN: 'invalid_state',
  REQUEST_SOLUTION_WRONG_STATE: 'invalid_state',
  REQUEST_SOLUTION_NOT_PROPOSED: 'invalid_state',
  REQUEST_SOLUTION_OPTION_NOT_CHOSEN: 'invalid_state',
  REQUEST_SOLUTION_KIND_NOT_ALLOWED: 'invalid_state',
  APPROVAL_ALREADY_DECIDED: 'invalid_state',
  REQUEST_APPROVAL_ALREADY_DECIDED: 'invalid_state',
  REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER: 'invalid_state',

  // Approval refused until the AI's blocking open questions are answered; not an "already decided" state.
  REQUEST_SOLUTION_BLOCKING_QUESTIONS: 'blocking_questions',
  // Same idea for a Plan: approval is refused until its `needs_confirmation` assumptions have an answer.
  REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS: 'unconfirmed_assumptions',

  // Validation
  REQUEST_REASON_REQUIRED: 'validation',
  REQUEST_TYPE_REQUIRED: 'validation',
  REQUEST_SIZE_REQUIRED: 'validation',
  REQUEST_INVALID_TYPE: 'validation',
  REQUEST_TYPE_UNCHANGED: 'validation',
  REQUEST_HOTFIX_REQUIRES_URGENT: 'validation',
  REQUEST_TITLE_REQUIRED: 'validation',
  REQUEST_BODY_TOO_LARGE: 'validation',
  REQUEST_PROJECT_REQUIRED: 'validation',
  REQUEST_SOURCE_PROVIDER_INVALID: 'validation',
  REQUEST_SOURCE_REF_REQUIRED: 'validation',
  REQUEST_RETURN_STAGE_INVALID: 'validation',
  REQUEST_RETURN_CATEGORY_INVALID: 'validation',
  REQUEST_LINK_SELF: 'validation',
  REQUEST_BACKLOG_INVALID_VIEW: 'validation',
  REQUEST_BACKLOG_BAD_PAGE_TOKEN: 'validation',
  APPROVAL_COMMENT_REQUIRED: 'validation',
  REQUEST_APPROVAL_COMMENT_REQUIRED: 'validation',
  REQUEST_SOURCE_FORBIDDEN: 'validation',
  REQUEST_PENDING_LIMIT: 'validation',
  INVALID_ARGUMENT: 'validation',

  // Rate / quota limits. Only REQUEST_RATE_LIMITED is a short, retryable throttle;
  // callers key the one automatic retry on that code, not on the kind.
  REQUEST_RATE_LIMITED: 'rate_limited',
  REQUEST_CLASSIFICATION_LIMIT: 'rate_limited',
  REQUEST_AI_QUOTA_EXHAUSTED: 'rate_limited',

  // Feature switched off for the tenant
  REQUEST_FLOW_DISABLED: 'feature_off',

  // Clarification / decision / risk (CR-REQ-028, 029, 030)
  REQUEST_CLARIFICATION_NOT_FOUND: 'not_found',
  REQUEST_CLARIFICATION_NOT_OPEN: 'invalid_state',
  REQUEST_CLARIFICATION_ALREADY_ANSWERED: 'invalid_state',
  REQUEST_CLARIFICATION_STATE_NOT_ALLOWED: 'invalid_state',
  REQUEST_CLARIFICATION_EXPIRED: 'expired',
  REQUEST_CLARIFICATION_INCOMPLETE: 'validation',
  REQUEST_CLARIFICATION_INVALID_ANSWER: 'validation',
  REQUEST_CLARIFICATION_NOT_ASSIGNEE: 'forbidden',
  REQUEST_CLARIFICATION_VERSION_CONFLICT: 'conflict',
  REQUEST_DECISION_RATIONALE_REQUIRED: 'validation',
  REQUEST_DECISION_CONFIRMATION_MISMATCH: 'validation',
  REQUEST_DECISION_NOT_EFFECTIVE: 'invalid_state',
  REQUEST_DECISION_SELF_CHOICE_FORBIDDEN: 'forbidden',
  REQUEST_DECISION_AGENT_FORBIDDEN: 'forbidden',
  REQUEST_RISK_ASSESSMENT_PENDING: 'pending',
  REQUEST_RISK_ACCEPTANCE_REQUIRED: 'validation',
  REQUEST_RISK_ASSESSMENT_STALE: 'conflict',
  REQUEST_RISK_APPROVER_NOT_ALLOWED: 'forbidden',
  REQUEST_RISK_OVERRIDE_REASON_REQUIRED: 'validation',
  REQUEST_IMPACT_NO_CONNECTION: 'no_dev_server',
  REQUEST_READINESS_WAIVE_FORBIDDEN: 'forbidden',
  APPROVAL_EXPIRED: 'expired',
  REQUEST_APPROVAL_EXPIRED: 'expired',

  // AI provider on the dev server (CONTRACT section 5)
  REQUEST_AI_NOT_CONFIGURED: 'no_ai',
  REQUEST_AI_CREDENTIAL_UNAVAILABLE: 'no_ai',
  REQUEST_AI_SESSION_EXPIRED: 'no_ai',
  REQUEST_AI_NO_PROVIDER: 'no_ai',
  REQUEST_AI_AGENT_UNAVAILABLE: 'no_dev_server',
  REQUEST_SOLUTION_NO_CONNECTION: 'no_dev_server',
  REQUEST_ANALYSIS_NO_CONNECTION: 'no_dev_server',
  REQUEST_AI_BUSY: 'unavailable',
  REQUEST_ANALYSIS_BUSY: 'unavailable',
  REQUEST_AI_COMPLETE_TIMEOUT: 'unavailable',
  REQUEST_PLAN_AI_UNAVAILABLE: 'unavailable',

  // Dev server attached to a project
  PROJECT_DEV_SERVER_NOT_FOUND: 'no_dev_server',
  PROJECT_DEV_SERVER_UNREACHABLE: 'no_dev_server',
  PROJECT_DEV_SERVER_CONNECTION_FAILED: 'no_dev_server',
  PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED: 'no_dev_server',
  PROJECT_DEV_SERVER_LOOKUP_FAILED: 'unavailable',

  // Not supported by runtime
  METHOD_NOT_FOUND: 'unsupported',
  CAPABILITY_UNSUPPORTED: 'unsupported',
  REQUEST_NOT_IMPLEMENTED: 'unsupported',

  // Service unavailable
  SERVICE_UNAVAILABLE: 'unavailable',
  GATEWAY_TIMEOUT: 'unavailable',
  REQUEST_UNAVAILABLE: 'unavailable',
  TASK_UNAVAILABLE: 'unavailable',
  REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE: 'unavailable',
  REQUEST_SOURCE_FETCH_FAILED: 'unavailable'
}

/** @internal Exposed for the contract table test; callers use classifyRequestRpcError. */
export function requestErrorKindForCode(code: string): RequestRpcErrorKind {
  return CODE_TO_KIND[code] ?? 'unknown'
}

/**
 * Drop the REQUEST_ prefix from APPROVAL_* and SOLUTION_* codes so callers can match one
 * spelling regardless of which form the server sent.
 */
export function canonicalApprovalSolutionCode(code: string): string {
  return code.toUpperCase().replace(/^REQUEST_(?=(APPROVAL|SOLUTION)_)/, '')
}

const BLOCKING_IDS_RE = /unanswered:\s*([^\n]+?)\s*$/i
const QUESTION_ID_RE = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/

/**
 * Ids from `REQUEST_SOLUTION_BLOCKING_QUESTIONS: ... unanswered: Q-1, Q-2` (also the Plan
 * variant, `A-1, A-2`). The gateway only sends text, so anything that does not look like an id is dropped.
 */
export function parseBlockingQuestionIds(message: string): string[] {
  const m = BLOCKING_IDS_RE.exec(splitErrorCode(message).rest)
  if (!m) {
    return []
  }
  return m[1]
    .split(',')
    .map((part) => part.trim())
    .filter((part) => QUESTION_ID_RE.test(part))
}

/** Assumption ids from `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS: ... unanswered: A-1`. */
export const parseUnconfirmedAssumptionIds = parseBlockingQuestionIds

const RETRY_AFTER_TRAILER_RE = /\{[^{}]*"retryAfterMs"\s*:\s*(\d+)[^{}]*\}\s*$/

/** Read the `{"retryAfterMs":N}` trailer the gateway appends to throttling errors. */
export function parseRetryAfterMs(message: string): number | undefined {
  const m = RETRY_AFTER_TRAILER_RE.exec(message)
  if (!m) {
    return undefined
  }
  const ms = Number(m[1])
  return Number.isFinite(ms) && ms >= 0 ? ms : undefined
}

// ---------------------------------------------------------------------------
// Classifier
// ---------------------------------------------------------------------------

/**
 * Classify any raw error value (from callRuntimeRpc / subscribeRuntimeStreamChannel)
 * into a typed RequestRpcError. Never throws.
 *
 * Duck-types the error: if it has .code and .message it is treated as a
 * RuntimeRpcCallError-shaped object to avoid importing renderer types here.
 */
export function classifyRequestRpcError(err: unknown): RequestRpcError {
  // Duck-typed RuntimeRpcCallError
  if (
    err !== null &&
    typeof err === 'object' &&
    'code' in err &&
    'message' in err &&
    typeof (err as Record<string, unknown>).code === 'string' &&
    typeof (err as Record<string, unknown>).message === 'string'
  ) {
    const code = (err as { code: string }).code
    const message = (err as { message: string }).message

    // method_not_found is a JSON-RPC level code
    if (code === 'method_not_found' || code === '-32601') {
      return { kind: 'unsupported', code, message, raw: err }
    }

    // Try to extract uppercase code from message body (CODE: text pattern)
    const { code: extractedCode } = splitErrorCode(message)
    const lookupCode = extractedCode || code.toUpperCase()
    const kind: RequestRpcErrorKind = CODE_TO_KIND[lookupCode] ?? 'unknown'
    const retryAfterMs = parseRetryAfterMs(message)
    return {
      kind,
      code,
      message,
      ...(extractedCode ? { backendCode: extractedCode } : {}),
      ...(retryAfterMs !== undefined ? { retryAfterMs } : {}),
      raw: err
    }
  }

  // Network / offline errors (no code property)
  if (err instanceof Error) {
    const msg = err.message.toLowerCase()
    if (
      msg.includes('network') ||
      msg.includes('fetch') ||
      msg.includes('timeout') ||
      msg.includes('econnrefused') ||
      msg.includes('offline')
    ) {
      return { kind: 'network', code: 'NETWORK_ERROR', message: err.message, raw: err }
    }
    return { kind: 'unknown', code: 'UNKNOWN_ERROR', message: err.message, raw: err }
  }

  return {
    kind: 'unknown',
    code: 'UNKNOWN_ERROR',
    message: typeof err === 'string' ? err : 'Unknown error',
    raw: err
  }
}
