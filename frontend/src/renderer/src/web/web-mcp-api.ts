import type { McpBridgeApi } from '../../../preload/api-types'
import { MCP_EVENT_TYPES, type McpEvent } from '../../../shared/mcp-types'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'

export type McpTransport = {
  callRuntimeResult: <T>(method: string, params?: unknown, timeoutMs?: number) => Promise<T>
  /** null = no runtime environment yet (caller gets an empty teardown, no error). */
  openStream: (
    method: string,
    params: unknown,
    handlers: {
      onResponse: (r: RuntimeRpcResponse<unknown>) => void
      onClose: () => void
    }
  ) => Promise<{ unsubscribe: () => void }> | null
}

export function isMcpEvent(v: unknown): v is McpEvent {
  return (
    typeof v === 'object' &&
    v !== null &&
    (MCP_EVENT_TYPES as readonly unknown[]).includes((v as { type?: unknown }).type)
  )
}

export function createMcpApi(t: McpTransport): McpBridgeApi {
  return {
    // C10: session dialect forwards params as exactly one args[0] object.
    call: (method, ...params) => t.callRuntimeResult(method, params[0] ?? {}) as never,
    subscribeEvents: (onEvent, onClose) => {
      let cancelled = false
      let handle: { unsubscribe: () => void } | null = null
      const p = t.openStream(
        'mcp.events.subscribe',
        {},
        {
          // The first frame is a null ack; later frames are bare McpEvents.
          onResponse: (r) => {
            if (cancelled || !r.ok || r.result == null) {
              return
            }
            if (isMcpEvent(r.result)) {
              onEvent(r.result)
            }
          },
          onClose: () => {
            if (!cancelled) {
              onClose?.()
            }
          }
        }
      )
      if (!p) {
        return () => {}
      }
      void p
        .then((h) => {
          if (cancelled) {
            h.unsubscribe()
          } else {
            handle = h
          }
        })
        .catch(() => {
          if (!cancelled) {
            onClose?.()
          }
        })
      return () => {
        cancelled = true
        handle?.unsubscribe()
        handle = null
      }
    }
  }
}

/** Bridge for hosts without MCP transport: every call rejects, stream is a no-op. */
export function createUnavailableMcpApi(): McpBridgeApi {
  return {
    call: () => Promise.reject(new Error('MCP is not available in this build.')),
    subscribeEvents: () => () => {}
  }
}
