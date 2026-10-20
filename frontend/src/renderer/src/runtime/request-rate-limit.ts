/**
 * Request rate-limit policy — TASK-FE-E2E-006
 *
 * The gateway throttles per class (read / write / ai) with REQUEST_RATE_LIMITED and, when
 * it knows, `{"retryAfterMs":N}`. A throttled call never ran, so a retry is only unsafe
 * when repeating it could do something twice; the safe set is named here. After the
 * automatic retry the UI keeps that class' buttons off until the wait ends, so people do
 * not hammer a limiter that is already saying no.
 *
 * @module runtime/request-rate-limit
 */

import { REQUEST_RPC_METHODS, type RequestRpcMethod } from '../../../shared/request-rpc-methods'
import type { RequestRpcError } from '../../../shared/request-errors'

export type RateLimitClass = 'read' | 'write' | 'ai'

export const RATE_LIMITED_CODE = 'REQUEST_RATE_LIMITED'
export const DEFAULT_RETRY_AFTER_MS = 1000
/** Longer waits are shown to the user instead of silently holding the call. */
export const MAX_AUTO_RETRY_WAIT_MS = 5000
/** Upper bound for a button lock, so a bad value can never disable a button for long. */
export const MAX_COOLDOWN_MS = 30_000

const AI_METHODS: ReadonlySet<string> = new Set([
  REQUEST_RPC_METHODS.SOLUTION_GENERATE,
  REQUEST_RPC_METHODS.GENERATE_PLAN,
  REQUEST_RPC_METHODS.RETRY_PLAN,
  REQUEST_RPC_METHODS.CLASSIFY
])

const WRITE_METHODS: ReadonlySet<string> = new Set([
  REQUEST_RPC_METHODS.CREATE,
  REQUEST_RPC_METHODS.CANCEL,
  REQUEST_RPC_METHODS.REOPEN,
  REQUEST_RPC_METHODS.RETURN_TO_BACKLOG,
  REQUEST_RPC_METHODS.CONFIRM_TYPE,
  REQUEST_RPC_METHODS.CHANGE_TYPE,
  REQUEST_RPC_METHODS.SET_BASE_REF,
  REQUEST_RPC_METHODS.SPAWN_CHILD,
  REQUEST_RPC_METHODS.START_PHASE,
  REQUEST_RPC_METHODS.SOLUTION_CHOOSE,
  REQUEST_RPC_METHODS.APPROVAL_APPROVE,
  REQUEST_RPC_METHODS.APPROVAL_REJECT,
  REQUEST_RPC_METHODS.BACKLOG_EXECUTE,
  REQUEST_RPC_METHODS.CLARIFICATION_ANSWER,
  REQUEST_RPC_METHODS.CLARIFICATION_CANCEL,
  REQUEST_RPC_METHODS.CLARIFICATION_REQUEST,
  REQUEST_RPC_METHODS.DECISION_CONFIRM,
  REQUEST_RPC_METHODS.IMPACT_ACCEPT,
  REQUEST_RPC_METHODS.RISK_OVERRIDE,
  REQUEST_RPC_METHODS.READINESS_CHECK,
  REQUEST_RPC_METHODS.ENGINE_SETTINGS_SET
])

/**
 * Writes that are safe to repeat: solution.generate carries an idempotency key, and the
 * others are guarded by expectedVersion/digest, so a second run fails closed.
 */
const IDEMPOTENT_WRITES: ReadonlySet<string> = new Set([
  REQUEST_RPC_METHODS.SOLUTION_GENERATE,
  REQUEST_RPC_METHODS.CLARIFICATION_ANSWER,
  // Server returns the same Clarification for the same source, sourceRef and question keys.
  REQUEST_RPC_METHODS.CLARIFICATION_REQUEST,
  REQUEST_RPC_METHODS.APPROVAL_APPROVE,
  REQUEST_RPC_METHODS.APPROVAL_REJECT,
  // Carries expectedVersion: a repeat after a throttled call that did run fails with REQUEST_VERSION_CONFLICT.
  REQUEST_RPC_METHODS.SET_BASE_REF
])

export function rateLimitClassOf(method: RequestRpcMethod): RateLimitClass {
  if (AI_METHODS.has(method)) {
    return 'ai'
  }
  return WRITE_METHODS.has(method) ? 'write' : 'read'
}

/** Reads and the guarded writes above retry on their own; everything else is shown to the user. */
export function isRetrySafe(method: RequestRpcMethod): boolean {
  return rateLimitClassOf(method) === 'read' || IDEMPOTENT_WRITES.has(method)
}

/** How long the limiter asked us to wait (default when it did not say). */
export function cooldownMsFor(error: Pick<RequestRpcError, 'retryAfterMs'>): number {
  return Math.min(Math.max(error.retryAfterMs ?? DEFAULT_RETRY_AFTER_MS, 0), MAX_COOLDOWN_MS)
}

export function isShortThrottle(error: Pick<RequestRpcError, 'backendCode'>): boolean {
  return error.backendCode === RATE_LIMITED_CODE
}

/** Wait before the single automatic retry, or null when the call must not be retried. */
export function autoRetryDelayMs(
  method: RequestRpcMethod,
  error: Pick<RequestRpcError, 'backendCode' | 'retryAfterMs'>,
  idempotentOverride?: boolean
): number | null {
  if (!isShortThrottle(error)) {
    return null
  }
  if (!(idempotentOverride ?? isRetrySafe(method))) {
    return null
  }
  const wait = error.retryAfterMs ?? DEFAULT_RETRY_AFTER_MS
  return wait <= MAX_AUTO_RETRY_WAIT_MS ? wait : null
}

// ---------------------------------------------------------------------------
// Cooldown store (module state, read through useRequestCooldown)
// ---------------------------------------------------------------------------

const until: Record<RateLimitClass, number> = { read: 0, write: 0, ai: 0 }
const listeners = new Set<() => void>()
let version = 0

function emit(): void {
  version += 1
  for (const l of listeners) {
    l()
  }
}

export function startRequestCooldown(
  cls: RateLimitClass,
  ms: number,
  now: number = Date.now()
): void {
  const end = now + Math.min(Math.max(ms, 0), MAX_COOLDOWN_MS)
  if (end <= until[cls]) {
    return
  }
  until[cls] = end
  emit()
}

/** Milliseconds left on the class' cooldown (0 when free). */
export function cooldownRemainingMs(cls: RateLimitClass, now: number = Date.now()): number {
  return Math.max(until[cls] - now, 0)
}

export function subscribeRequestCooldown(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function requestCooldownVersion(): number {
  return version
}

/** Test seam: clear every cooldown. */
export function resetRequestCooldowns(): void {
  until.read = 0
  until.write = 0
  until.ai = 0
  emit()
}
