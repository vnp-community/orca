// src/relay/emulator-session.ts
// Session/handshake orchestration for the Orca Mobile Emulator Agent.
//
// Deliberately NOT a copy of agent/src/relay/agent-session.ts and does not
// import it: that file hardcodes createRpcDispatcher (~150 git/fs/pty cases)
// and git/pty capability detection. Routing a mis-routed git.exec through
// Mobile Emulator Agent code running on someone's personal laptop would
// recreate exactly the security hole CR-DS-009 exists to close — so this
// session only ever dispatches through the device.*-only
// EmulatorRpcDispatcher, has no capability detection beyond the fixed
// ['device'] list, and does no PTY/fs-watch cleanup on stop() (this agent
// never creates any).
//
// Wire framing (13-byte header, seq/ack, keepalive) is reused as-is from
// orca-dev-agent-transport (see specs/emulator/tdd/v1/03-transport-reuse-analysis.md)
// so this agent and the Dev Server Agent never drift on the wire protocol.
import type WebSocket from 'ws'
import {
  createWireState,
  decodeFrame,
  encodeDataFrame,
  encodeKeepaliveFrame,
  parseJsonPayload,
  MessageType,
  type WireState
} from 'orca-dev-agent-transport'
import type { EmulatorConfig } from './emulator-config'
import type { EmulatorLogger } from './emulator-logger'
import type { EmulatorRpcDispatcher, JsonRpcRequest } from './emulator-rpc-dispatch'

const HANDSHAKE_METHOD = 'agent.handshake'
const AGENT_VERSION = '0.1.0'
const KEEPALIVE_INTERVAL_MS = 5_000

type HandshakeResultFrame = {
  id: string | number | null
  result?: { ok?: boolean; orcaVersion?: string; sessionId?: string }
  error?: { code: number; message: string }
  method?: string
  params?: Record<string, unknown>
}

export type EmulatorSession = {
  /** Attach session logic to an already-constructed WebSocket (may or may not be open yet). */
  start(ws: WebSocket): void
  /** Clear keepalive interval. Call from ws 'close' handler. */
  stop(): void
  /** Register a callback that fires once after a successful handshake. */
  onHandshakeOk(callback: () => void): void
}

export function createEmulatorSession(
  config: EmulatorConfig,
  log: EmulatorLogger,
  dispatcher: EmulatorRpcDispatcher
): EmulatorSession {
  let keepaliveTimer: ReturnType<typeof setInterval> | null = null
  let handshakeDone = false
  const handshakeOkCallbacks: (() => void)[] = []

  function sendHandshake(ws: WebSocket, wireState: WireState): void {
    const rpc = {
      jsonrpc: '2.0' as const,
      id: 1,
      method: HANDSHAKE_METHOD,
      params: {
        agentVersion: AGENT_VERSION,
        platform: process.platform,
        arch: process.arch,
        // Fixed — no tools/git/pty capabilities. This is the security
        // boundary: a backend that mis-routes a git.* or pty.* request here
        // has no dispatcher case to run it against.
        capabilities: ['device'],
        // agentToken is only meaningful in direct-websocket mode; empty/unset = omit.
        ...(config.agentToken ? { agentToken: config.agentToken } : {})
      }
    }
    ws.send(encodeDataFrame(wireState, JSON.stringify(rpc)))
    log.info('Handshake sent (capabilities=[device])')
  }

  function startKeepalive(ws: WebSocket, wireState: WireState): void {
    keepaliveTimer = setInterval(() => {
      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        ws.send(encodeKeepaliveFrame(wireState))
      }
    }, KEEPALIVE_INTERVAL_MS)
  }

  function stop(): void {
    if (keepaliveTimer !== null) {
      clearInterval(keepaliveTimer)
      keepaliveTimer = null
    }
  }

  return {
    start(ws: WebSocket): void {
      // wireState is scoped to this connection — not shared across reconnects.
      const wireState = createWireState()

      const doHandshake = (): void => {
        sendHandshake(ws, wireState)
        startKeepalive(ws, wireState)
      }

      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        doHandshake()
      } else {
        ws.once('open', () => {
          log.info('WebSocket opened')
          doHandshake()
        })
      }

      ws.on('message', (data: Buffer | string) => {
        // Wire protocol uses binary frames only.
        if (!Buffer.isBuffer(data)) {
          return
        }

        const frame = decodeFrame(wireState, data)
        if (!frame) {
          log.warn('Received malformed frame (too short) — ignoring')
          return
        }

        if (frame.type === MessageType.KeepAlive) {
          if (ws.readyState === 1) {
            ws.send(encodeKeepaliveFrame(wireState))
          }
          return
        }

        if (frame.payload.length === 0) {
          return
        }

        const rpc = parseJsonPayload<HandshakeResultFrame>(frame.payload)
        if (!rpc) {
          log.warn('Received non-JSON frame payload — ignoring')
          return
        }

        if (!handshakeDone) {
          // Only process the handshake response (id=1) before handshake completes.
          if (rpc.result?.ok === true) {
            handshakeDone = true
            const sessionId = rpc.result.sessionId ?? 'unknown'
            const orcaVersion = rpc.result.orcaVersion ?? 'unknown'
            log.info(`Handshake OK: sessionId=${sessionId} orcaVersion=${orcaVersion}`)
            handshakeOkCallbacks.forEach((cb) => cb())
          } else if (rpc.error) {
            log.error(`Handshake failed: code=${rpc.error.code} message=${rpc.error.message}`)
            ws.close(1008, 'Handshake failed')
          }
          return
        }

        // Post-handshake: dispatch device.* JSON-RPC requests only.
        if (typeof rpc.method === 'string') {
          const request: JsonRpcRequest = {
            jsonrpc: '2.0',
            id: rpc.id,
            method: rpc.method,
            params: rpc.params
          }
          dispatcher
            .dispatch(request)
            .then((response) => {
              if (ws.readyState === 1) {
                ws.send(encodeDataFrame(wireState, JSON.stringify(response)))
              }
            })
            .catch((err: unknown) => {
              log.error(
                `dispatch threw for ${request.method}: ${err instanceof Error ? err.stack : String(err)}`
              )
            })
        }
      })

      ws.on('close', (code: number, reason: Buffer) => {
        stop()
        log.info(`Session closed code=${code} reason=${reason.toString()}`)
      })

      ws.on('error', (err: Error) => {
        log.error(`WebSocket error: ${err.message}`)
      })
    },

    stop,

    onHandshakeOk(callback: () => void): void {
      handshakeOkCallbacks.push(callback)
    }
  }
}
