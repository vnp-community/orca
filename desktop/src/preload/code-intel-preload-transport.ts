/**
 * code-intel-preload-transport.ts — FE-CV-TASK-050-06
 *
 * Electron transport for the shared code-intel bridge. Environment calls go through the same
 * `runtimeEnvironments:call` / `:subscribe` IPC as `window.api.runtimeEnvironments` (main reads
 * `selector`, not `environmentId`). The local target has no code-intel runtime yet, so it answers
 * method_not_found and the renderer reports `unsupported`.
 *
 * @module preload/code-intel-preload-transport
 */

import type {
  CodeIntelBridgeDeps,
  CodeIntelRawEnvelope
} from '../../../frontend/src/shared/code-intel-bridge'
import { subscribeRuntimeEnvironmentFromPreload } from './runtime-environment-subscriptions'

type CodeIntelPreloadIpc = Parameters<typeof subscribeRuntimeEnvironmentFromPreload>[0]

const LOCAL_UNSUPPORTED: CodeIntelRawEnvelope = {
  ok: false,
  error: { code: 'method_not_found', message: 'code-intel: not available on the local runtime' }
}

export function createCodeIntelPreloadDeps(
  ipc: CodeIntelPreloadIpc,
  subscribe: typeof subscribeRuntimeEnvironmentFromPreload = subscribeRuntimeEnvironmentFromPreload
): CodeIntelBridgeDeps {
  return {
    callLocal: () => Promise.resolve(LOCAL_UNSUPPORTED),

    callEnvironment: async (environmentId, method, params) => {
      try {
        return (await ipc.invoke('runtimeEnvironments:call', {
          selector: environmentId,
          method,
          params
        })) as CodeIntelRawEnvelope
      } catch (error) {
        // Why: callers parse envelopes; a rejected IPC must not escape as a throw.
        return {
          ok: false,
          error: {
            code: 'connection_refused',
            message: error instanceof Error ? error.message : 'Transport error'
          }
        }
      }
    },

    subscribeEnvironment: (environmentId, method, params, callbacks) => {
      let cancelled = false
      let unsubscribe: (() => void) | null = null
      void subscribe(
        ipc,
        { selector: environmentId, method, params },
        {
          // First frame is the null ack; later frames carry the bare push object in `result`.
          onResponse: (response) => {
            const frame = response as { ok?: boolean; result?: unknown }
            if (!cancelled && frame.ok === true && frame.result != null) {
              callbacks.onEvent(frame.result)
            }
          },
          onClose: () => {
            if (!cancelled) {
              callbacks.onClose()
            }
          }
        }
      )
        .then((handle) => {
          if (cancelled) {
            handle.unsubscribe()
          } else {
            unsubscribe = handle.unsubscribe
          }
        })
        .catch(() => {
          // The environment refused the stream: the renderer falls back to polling.
          if (!cancelled) {
            callbacks.onUnsupported()
          }
        })
      return () => {
        cancelled = true
        unsubscribe?.()
        unsubscribe = null
      }
    }
  }
}
