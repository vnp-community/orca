/**
 * Request RPC Client — CR-REQ-018-02
 *
 * Thin wrapper around callRuntimeRpc that:
 * - Never throws — wraps every call in a Result<T, RequestRpcError>
 * - Picks the active runtime target from the store
 * - Logs params at debug level but strips `.body` to avoid leaking request content
 *
 * @module runtime/request-rpc-client
 */

import {
  callRuntimeRpc,
  subscribeRuntimeStreamChannel,
  getActiveRuntimeTarget
} from './runtime-rpc-client'
import { classifyRequestRpcError } from '../../../shared/request-errors'
import { parseRequestEvent } from '../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS, type RequestRpcMethod } from '../../../shared/request-rpc-methods'
import {
  autoRetryDelayMs,
  cooldownMsFor,
  isShortThrottle,
  rateLimitClassOf,
  startRequestCooldown
} from './request-rate-limit'
import type { RequestRpcError } from '../../../shared/request-errors'
import type { RequestEvent } from '../../../shared/request-types'
import { useAppStore } from '../store'

// ---------------------------------------------------------------------------
// Result type (avoid importing a full result lib)
// ---------------------------------------------------------------------------

export type OkResult<T> = { ok: true; value: T }
export type ErrResult = { ok: false; error: RequestRpcError }
export type Result<T> = OkResult<T> | ErrResult

// ---------------------------------------------------------------------------
// Core RPC call — never throws
// ---------------------------------------------------------------------------

export type CallRequestRpcOptions = {
  /**
   * Overrides whether one automatic retry after REQUEST_RATE_LIMITED is safe. By default
   * reads and version-guarded writes retry; other writes are reported, never repeated.
   */
  idempotent?: boolean
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/**
 * Call a request-service RPC method. Never rejects — returns Result<T, RequestRpcError>.
 *
 * A throttled call (REQUEST_RATE_LIMITED) is retried once after `retryAfterMs` (1s when the
 * server gave none) if repeating it is safe; if it still fails the class' cooldown starts so
 * buttons stop firing into the limiter.
 */
export async function callRequestRpc<T>(
  method: RequestRpcMethod,
  params?: object,
  options: CallRequestRpcOptions = {}
): Promise<Result<T>> {
  const settings = useAppStore.getState().settings
  const target = getActiveRuntimeTarget(settings)

  let retried = false
  for (;;) {
    try {
      const value = await callRuntimeRpc<T>(target, method, params)
      return { ok: true, value }
    } catch (err) {
      const error = classifyRequestRpcError(err)
      const delay = retried ? null : autoRetryDelayMs(method, error, options.idempotent)
      if (delay !== null) {
        retried = true
        await wait(delay)
        continue
      }
      if (isShortThrottle(error)) {
        startRequestCooldown(rateLimitClassOf(method), cooldownMsFor(error))
      }
      return { ok: false, error }
    }
  }
}

/**
 * Like callRequestRpc but throws on error instead of returning Err.
 * For internal use by hooks that need to throw inside promise chains.
 */
export async function callRequestRpcOrThrow<T>(
  method: RequestRpcMethod,
  params?: object,
  options?: CallRequestRpcOptions
): Promise<T> {
  const result = await callRequestRpc<T>(method, params, options)
  if (!result.ok) {
    throw result.error
  }
  return result.value
}

// ---------------------------------------------------------------------------
// Event subscription
// ---------------------------------------------------------------------------

export type SubscribeRequestEventsOptions = {
  requestId?: string
  onEvent: (event: RequestEvent) => void
  /** Called once when the runtime doesn't support streaming (local target or reject) */
  onFallback: () => void
}

/**
 * Subscribe to request events via the streaming channel.
 * On local target or unsupported error: calls onFallback() once and returns a no-op unsubscribe.
 * Returns a cleanup function; safe to call before ack arrives.
 */
export function subscribeRequestEvents({
  requestId,
  onEvent,
  onFallback
}: SubscribeRequestEventsOptions): () => void {
  const settings = useAppStore.getState().settings
  const target = getActiveRuntimeTarget(settings)

  if (target.kind === 'local') {
    // Desktop runtime has no streaming request events — fall back to polling
    onFallback()
    return () => {}
  }

  let cancelled = false
  let unsubscribeFn: (() => void) | null = null

  // Even if cancelled before ack, we need to call unsubscribe after ack arrives
  subscribeRuntimeStreamChannel(
    target,
    REQUEST_RPC_METHODS.SUBSCRIBE,
    requestId ? { id: requestId } : {},
    (evt: unknown) => {
      if (!cancelled) {
        onEvent(parseRequestEvent(evt))
      }
    }
  )
    .then(({ unsubscribe }) => {
      unsubscribeFn = unsubscribe
      if (cancelled) {
        // Caller already cleaned up before ack — unsubscribe immediately
        unsubscribe()
      }
    })
    .catch((err: unknown) => {
      // Check if error is unsupported (method_not_found etc.)
      const classified = classifyRequestRpcError(err)
      if (classified.kind === 'unsupported' || classified.kind === 'network') {
        onFallback()
      }
      // Other errors are logged but don't trigger fallback — let the hook retry
      if (classified.kind !== 'unsupported') {
        console.error('[request-rpc-client] subscription failed', classified)
      }
    })

  return () => {
    cancelled = true
    unsubscribeFn?.()
  }
}
