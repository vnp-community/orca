/**
 * web-code-intel-api.ts — FE-CV-TASK-050-05
 *
 * Web (browser) implementation of CodeIntelBridgeApi.
 * Routes calls through callEnvironmentEnvelope (same transport as MCP).
 *
 * Rules:
 * - callLocal always returns ok:false with method_not_found (web has no runtime-local)
 * - No environment active → envelope error (never throws)
 * - Does NOT detect feature via typeof window.api.codeIntel (Proxy withFallback)
 *
 * @module web/web-code-intel-api
 */

import { createCodeIntelBridge } from '../../../../shared/code-intel-bridge'
import type { CodeIntelBridgeApi, CodeIntelRawEnvelope } from '../../../../shared/code-intel-bridge'

type WebCodeIntelTransport = {
  callEnvironmentEnvelope: (
    selector: unknown,
    method: string,
    params: unknown,
    timeoutMs?: number
  ) => Promise<CodeIntelRawEnvelope>
  subscribe?: (
    method: string,
    params: unknown,
    callbacks: { onResponse?: (r: unknown) => void; onClose?: () => void }
  ) => (() => void) | null
}

const METHOD_NOT_FOUND_ENVELOPE: CodeIntelRawEnvelope = {
  ok: false,
  result: null,
  error: {
    code: 'method_not_found',
    message: 'code-intel: local runtime not available in web context'
  }
}

/**
 * Create the code-intel API for the web preload layer.
 * Transport functions are injected to avoid circular dependencies.
 */
export function createCodeIntelApi(transport: WebCodeIntelTransport): CodeIntelBridgeApi {
  return createCodeIntelBridge({
    /**
     * Local runtime call — web never has a local runtime.
     * Always returns method_not_found envelope.
     */
    async callLocal(_method, _params) {
      return METHOD_NOT_FOUND_ENVELOPE
    },

    /**
     * Environment call — routes through callEnvironmentEnvelope.
     */
    async callEnvironment(environmentId, method, params) {
      try {
        const result = await transport.callEnvironmentEnvelope(
          environmentId,
          method,
          params,
          /* timeoutMs */ undefined
        )
        return result
      } catch (err) {
        // Transport errors → ok:false envelope
        const message = err instanceof Error ? err.message : 'Transport error'
        return {
          ok: false,
          result: null,
          error: { code: 'connection_refused', message }
        }
      }
    },

    /**
     * Subscribe to environment push events.
     */
    subscribeEnvironment(environmentId, method, params, callbacks) {
      if (!transport.subscribe) return null
      return transport.subscribe(method, params, callbacks)
    }
  })
}
