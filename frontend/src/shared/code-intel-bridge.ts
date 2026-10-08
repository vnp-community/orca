/**
 * code-intel-bridge.ts — FE-CV-TASK-050-04
 *
 * Bridge factory for the code-intel API, mirroring how McpBridgeApi works.
 * Abstracts over local (not supported — returns method_not_found envelope)
 * vs environment (callEnvironment) transport.
 *
 * createCodeIntelBridge is the single factory; it is instantiated:
 *  - In web-preload-api.ts via createCodeIntelApi (050-05)
 *  - In desktop preload if wiring is possible (050-06)
 *
 * @module shared/code-intel-bridge
 */

import type { CodeIntelPushEvent } from './code-intel-types'
import { parseCodeIntelPushEvent } from './code-intel-parsers'

// ---------------------------------------------------------------------------
// Bridge API types
// ---------------------------------------------------------------------------

export type CodeIntelCallArgs = {
  /** environmentId === null ⇒ local path */
  environmentId: string | null
  method: string
  params: unknown
}

export type CodeIntelRawEnvelope = {
  ok: boolean
  result?: unknown
  error?: { code: string; message: string; data?: unknown }
}

/** Subscription callbacks */
export type CodeIntelSubscribeCallbacks = {
  /** Called for each push event frame (filtered to known event names) */
  onEvent: (event: CodeIntelPushEvent) => void
  /** Called when the stream ends cleanly or the socket closes */
  onClose: () => void
  /** Called when the local target doesn't support streaming */
  onUnsupported: () => void
}

/** Transport-level callbacks: frames are raw (unparsed) and normalized by the bridge. */
export type CodeIntelRawSubscribeCallbacks = Omit<CodeIntelSubscribeCallbacks, 'onEvent'> & {
  onEvent: (frame: unknown) => void
}

export type CodeIntelBridgeDeps = {
  /** Call a local runtime method (desktop only); returns raw envelope */
  callLocal: (method: string, params: unknown) => Promise<CodeIntelRawEnvelope>
  /** Call a method on an environment runtime; returns raw envelope */
  callEnvironment: (environmentId: string, method: string, params: unknown) => Promise<CodeIntelRawEnvelope>
  /** Subscribe to push events on an environment */
  subscribeEnvironment: (
    environmentId: string,
    method: string,
    params: unknown,
    callbacks: CodeIntelRawSubscribeCallbacks
  ) => () => void
}

export type CodeIntelBridgeApi = {
  /**
   * Call a method. environmentId === null routes to local.
   * Returns the raw server envelope — callers parse it.
   */
  call: (args: CodeIntelCallArgs) => Promise<CodeIntelRawEnvelope>

  /**
   * Subscribe to codeIntel push events.
   * Local target: fires onUnsupported() once and returns a no-op cleanup.
   * Returns a cleanup / unsubscribe function; safe to call before ACK arrives.
   */
  subscribeEvents: (
    environmentId: string | null,
    callbacks: CodeIntelSubscribeCallbacks
  ) => () => void
}

// ---------------------------------------------------------------------------
// Push frames are normalized by parseCodeIntelPushEvent (unknown events dropped)
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

/**
 * Create a CodeIntelBridgeApi from transport dependencies.
 * This factory is the single point of contact for both web and Electron.
 */
export function createCodeIntelBridge(deps: CodeIntelBridgeDeps): CodeIntelBridgeApi {
  const { callLocal, callEnvironment, subscribeEnvironment } = deps

  const call: CodeIntelBridgeApi['call'] = ({ environmentId, method, params }) => {
    if (environmentId === null) {
      return callLocal(method, params ?? {})
    }
    return callEnvironment(environmentId, method, params ?? {})
  }

  const subscribeEvents: CodeIntelBridgeApi['subscribeEvents'] = (environmentId, callbacks) => {
    if (environmentId === null) {
      // Local target has no push stream — caller should fall back to polling
      callbacks.onUnsupported()
      return () => {}
    }

    let cancelled = false
    let unsubscribeHandle: (() => void) | null = null

    const handle = subscribeEnvironment(
      environmentId,
      'codeIntel.subscribe',
      {},
      {
        onEvent: (frame) => {
          if (cancelled) {return}
          const event = parseCodeIntelPushEvent(frame)
          if (event) {callbacks.onEvent(event)}
        },
        onClose: () => {
          if (!cancelled) {callbacks.onClose()}
        },
        onUnsupported: () => {
          if (!cancelled) {callbacks.onUnsupported()}
        }
      }
    )

    unsubscribeHandle = handle

    return () => {
      cancelled = true
      unsubscribeHandle?.()
    }
  }

  return { call, subscribeEvents }
}
