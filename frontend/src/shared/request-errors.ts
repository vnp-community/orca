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
  | 'validation'
  | 'unsupported'
  | 'rate_limited'
  | 'unavailable'
  | 'expired'
  | 'pending'
  | 'no_dev_server'
  | 'network'
  | 'unknown'

export type RequestRpcError = {
  kind: RequestRpcErrorKind
  code: string
  message: string
  raw?: unknown
}

// ---------------------------------------------------------------------------
// Internal: split "CODE: message" error message format
// ---------------------------------------------------------------------------

const CODE_PREFIX_RE = /^([A-Z][A-Z0-9_]+):\s*(.*)/s

/** @internal */
export function splitErrorCode(message: string): { code: string; rest: string } {
  const m = CODE_PREFIX_RE.exec(message)
  if (m) return { code: m[1], rest: m[2] }
  return { code: '', rest: message }
}

// ---------------------------------------------------------------------------
// Code → kind mapping (CR-REQ-016 §2.8)
// ---------------------------------------------------------------------------

const CODE_TO_KIND: Record<string, RequestRpcErrorKind> = {
  // Authorisation
  REQUEST_FORBIDDEN: 'forbidden',
  APPROVAL_FORBIDDEN: 'forbidden',

  // Not found
  REQUEST_NOT_FOUND: 'not_found',
  SOLUTION_NOT_FOUND: 'not_found',
  APPROVAL_NOT_FOUND: 'not_found',

  // Conflict / stale
  REQUEST_STATE_STALE: 'conflict',
  APPROVAL_VERSION_CONFLICT: 'conflict',
  REQUEST_VERSION_CONFLICT: 'conflict',

  // Invalid state transitions
  REQUEST_TRANSITION_NOT_ALLOWED: 'invalid_state',
  REQUEST_TYPE_CHANGE_NOT_ALLOWED: 'invalid_state',
  REQUEST_TYPE_CHANGE_USE_CHILD: 'invalid_state',
  SOLUTION_ALREADY_CHOSEN: 'invalid_state',
  APPROVAL_ALREADY_DECIDED: 'invalid_state',

  // Validation
  REQUEST_REASON_REQUIRED: 'validation',
  REQUEST_TYPE_REQUIRED: 'validation',
  APPROVAL_COMMENT_REQUIRED: 'validation',
  REQUEST_SOURCE_FORBIDDEN: 'validation',
  REQUEST_PENDING_LIMIT: 'validation',

  // Classification limits
  REQUEST_CLASSIFICATION_LIMIT: 'rate_limited',

  // Clarification / decision / risk (CR-REQ-028, 029, 030; codes pending CONTRACT)
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

  // Not supported by runtime
  METHOD_NOT_FOUND: 'unsupported',
  CAPABILITY_UNSUPPORTED: 'unsupported',

  // Service unavailable
  SERVICE_UNAVAILABLE: 'unavailable',
  GATEWAY_TIMEOUT: 'unavailable'
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
    return { kind, code, message, raw: err }
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
