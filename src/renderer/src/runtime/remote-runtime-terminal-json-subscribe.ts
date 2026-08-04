// JSON fallback for terminal output streaming, used when the underlying RPC
// transport cannot carry binary WebSocket frames — the standalone multi-user
// web server's browser-session client (see pty-connection.ts's
// isWebClient / 'session-auth' checks). terminal.multiplex (the normal path,
// remote-runtime-terminal-multiplexer.ts) requires sendBinary /
// registerBinaryStreamHandler, which only exist on the direct-WebSocket/E2EE
// transport (Electron desktop, paired mobile) — the web session's browser ->
// WsSessionRouter -> Unix-socket-per-user-process path is JSON-lines only,
// with no binary-frame capability at any layer. terminal.subscribe is a
// second, plain-JSON streaming RPC method that already works over that path
// (orca-runtime.ts's dispatcher never requires sendBinary for a non-mobile
// client), so this module drives that instead.
//
// Deliberately narrower than the binary multiplexer:
//   - No shared connection/streamId multiplexing — each call opens its own
//     terminal.subscribe stream (the binary path shares one WS connection
//     across many terminals specifically to avoid per-tab WS overhead, which
//     doesn't apply here since each RPC call is already an independent
//     Unix-socket round trip regardless).
//   - No input/resize/viewport-claim support over this stream — sendInput/
//     resize/claimViewport all return false so remote-runtime-pty-transport.ts's
//     existing plain-RPC fallback (terminal.send / terminal.updateViewport)
//     fires instead, exactly as it already does today when there's no
//     multiplexed stream yet.
//   - No on-demand snapshot (serializeBuffer resolves null) and no
//     driver-changed push notifications (desktop/mobile viewport-ownership
//     events aren't part of this method) — both are accepted degradations
//     for this fallback, not bugs.
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'
import { unwrapRuntimeRpcResult } from './runtime-rpc-client'
import type {
  RemoteRuntimeMultiplexedTerminal,
  RemoteRuntimeMultiplexedTerminalCallbacks
} from './remote-runtime-terminal-multiplexer'

type TerminalSubscribeEvent =
  | { type: 'subscribed'; lines?: string[] }
  | { type: 'scrollback'; lines?: string[] }
  | { type: 'data'; chunk?: string }
  | {
      type: 'fit-override-changed'
      mode?: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit'
      cols?: number
      rows?: number
    }
  | { type: 'end' }
  | { type: string; [key: string]: unknown }

export async function subscribeTerminalViaJson(args: {
  environmentId: string
  terminal: string
  client: { id: string; type: 'desktop' | 'mobile' }
  viewport?: { cols: number; rows: number }
  callbacks: RemoteRuntimeMultiplexedTerminalCallbacks
}): Promise<RemoteRuntimeMultiplexedTerminal> {
  let closed = false
  let announcedSubscribed = false

  const announceSubscribedOnce = (): void => {
    if (announcedSubscribed) return
    announcedSubscribed = true
    args.callbacks.onSubscribed?.()
  }

  const handleResponse = (response: RuntimeRpcResponse<unknown>): void => {
    if (closed) return
    let event: TerminalSubscribeEvent
    try {
      event = unwrapRuntimeRpcResult(response) as TerminalSubscribeEvent
    } catch (error) {
      args.callbacks.onError?.(error instanceof Error ? error.message : String(error))
      return
    }

    if (event.type === 'scrollback' || event.type === 'subscribed') {
      const lines = Array.isArray(event.lines) ? event.lines : []
      if (lines.length > 0) {
        args.callbacks.onSnapshot(lines.join('\n'))
      }
      announceSubscribedOnce()
    } else if (event.type === 'data') {
      if (typeof event.chunk === 'string') {
        args.callbacks.onData(event.chunk)
      }
    } else if (event.type === 'fit-override-changed') {
      if (
        (event.mode === 'mobile-fit' ||
          event.mode === 'remote-desktop-fit' ||
          event.mode === 'desktop-fit') &&
        typeof event.cols === 'number' &&
        typeof event.rows === 'number'
      ) {
        args.callbacks.onFitOverrideChanged?.({ mode: event.mode, cols: event.cols, rows: event.rows })
      }
    } else if (event.type === 'end') {
      closed = true
      args.callbacks.onEnd?.()
    }
  }

  const subscription = await window.api.runtimeEnvironments.subscribe(
    {
      selector: args.environmentId,
      method: 'terminal.subscribe',
      params: { terminal: args.terminal, client: args.client, viewport: args.viewport }
    },
    {
      onResponse: handleResponse,
      onError: (error) => {
        if (!closed) {
          args.callbacks.onError?.(error.message)
        }
      },
      onClose: () => {
        if (!closed) {
          closed = true
          args.callbacks.onTransportClose?.()
        }
      }
    }
  )

  return {
    streamId: -1,
    sendInput: () => false,
    resize: () => false,
    claimViewport: () => false,
    serializeBuffer: async () => null,
    close: () => {
      if (closed) return
      closed = true
      subscription.unsubscribe()
      void window.api.runtimeEnvironments
        .call({
          selector: args.environmentId,
          method: 'terminal.unsubscribe',
          params: {
            subscriptionId: `${args.terminal}:${args.client.id}`,
            client: { id: args.client.id }
          }
        })
        .catch(() => {})
    }
  }
}
