// src/relay/agent-session.ts
// Manages a single WebSocket connection session for the Orca Dev Agent.
//
// Responsibilities:
//   1. Send handshake frame immediately on connect
//   2. Send keepalive frames on AGENT_KEEPALIVE_INTERVAL_MS cadence
//   3. Respond to incoming KeepAlive frames with a keepalive pong
//   4. Gate RPC dispatch behind successful handshake
//   5. Close ws with code 1008 on handshake auth failure
//
// Design:
//   - createSession() returns an AgentSession factory — one per WS connection
//   - WireState is created inside start() — NOT at module level
//   - stop() must be called when ws closes to clear the keepalive interval

import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentLogger } from './agent-logger'
import {
  createWireState,
  decodeFrame,
  encodeDataFrame,
  encodeKeepaliveFrame,
  parseJsonPayload,
} from './agent-wire'
import { createRpcDispatcher } from './agent-rpc-dispatch'
import type { JsonRpcRequest } from './agent-rpc-dispatch'
import {
  AGENT_HANDSHAKE_METHOD,
  AGENT_KEEPALIVE_INTERVAL_MS,
} from '../shared/agent-wire-protocol'
import { MessageType } from '../main/ssh/relay-protocol'
import { createTracer } from '../shared/trace'

const sessionTracer = createTracer('agent:session')

export interface AgentSession {
  /** Attach session logic to an already-constructed WebSocket (may or may not be open yet). */
  start(ws: WebSocket): void
  /** Clear keepalive interval. Call from ws 'close' handler. */
  stop(): void
  /** Register a callback that fires once after a successful handshake. */
  onHandshakeOk(callback: () => void): void
}

export function createSession(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): AgentSession {
  let keepaliveTimer: ReturnType<typeof setInterval> | null = null
  let handshakeDone = false
  const handshakeOkCallbacks: Array<() => void> = []
  const dispatcher = createRpcDispatcher(tools, config, log)

  function sendHandshake(ws: WebSocket, wireState: ReturnType<typeof createWireState>): void {
    const rpc = {
      jsonrpc: '2.0' as const,
      id: 1,
      method: AGENT_HANDSHAKE_METHOD,
      params: {
        agentVersion:  '5.0.0',
        platform:      process.platform,
        arch:          process.arch,
        nodeVersion:   process.version,
        capabilities:  ['fs', 'git', 'preflight', 'ai.providers', 'agent.spawn', 'worktrees'] as const,
        // agentToken is only sent in direct-websocket mode; empty string = omit
        ...(config.agentToken ? { agentToken: config.agentToken } : {}),
        devServerId:   config.devServerId,
        tools:         tools.map(t => t.name),
      },
    }
    ws.send(encodeDataFrame(wireState, JSON.stringify(rpc)))
    log.info(`Handshake sent: devServerId=${config.devServerId} tools=[${tools.map(t => t.name).join(',')}]`)
  }

  function startKeepalive(ws: WebSocket, wireState: ReturnType<typeof createWireState>): void {
    keepaliveTimer = setInterval(() => {
      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        ws.send(encodeKeepaliveFrame(wireState))
      }
    }, AGENT_KEEPALIVE_INTERVAL_MS)
  }

  return {
    start(ws: WebSocket): void {
      // wireState is scoped to this connection — not shared
      const wireState = createWireState()
      const span = sessionTracer.start({ devServerId: config.devServerId })

      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        sendHandshake(ws, wireState)
        span.step('handshake-sent')
        startKeepalive(ws, wireState)
      } else {
        ws.once('open', () => {
          log.info('WebSocket opened')
          sendHandshake(ws, wireState)
          span.step('handshake-sent')
          startKeepalive(ws, wireState)
        })
      }

      ws.on('message', (data: Buffer | string) => {
        // Agent protocol uses binary frames only
        if (!Buffer.isBuffer(data)) return

        const frame = decodeFrame(wireState, data)
        if (!frame) {
          log.warn('Received malformed frame (too short) — ignoring')
          return
        }

        // Respond to KeepAlive frames immediately to maintain ACK progress
        if (frame.type === MessageType.KeepAlive) {
          if (ws.readyState === 1) ws.send(encodeKeepaliveFrame(wireState))
          return
        }

        // Empty data frame — ignore
        if (frame.payload.length === 0) return

        const rpc = parseJsonPayload<{
          id: string | number | null
          result?: { ok?: boolean; orcaVersion?: string; sessionId?: string }
          error?: { code: number; message: string }
          method?: string
          params?: Record<string, unknown>
        }>(frame.payload)

        if (!rpc) {
          log.warn('Received non-JSON frame payload — ignoring')
          return
        }

        if (!handshakeDone) {
          // Only process handshake result (id=1) before handshake completes
          if (rpc.result?.ok === true) {
            handshakeDone = true
            const sessionId = rpc.result.sessionId ?? 'unknown'
            const orcaVersion = rpc.result.orcaVersion ?? 'unknown'
            log.info(`Handshake OK: sessionId=${sessionId} orcaVersion=${orcaVersion}`)
            span.step('handshake-ok', { sessionId, orcaVersion })
            handshakeOkCallbacks.forEach(cb => cb())
          } else if (rpc.error) {
            log.error(`Handshake failed: code=${rpc.error.code} message=${rpc.error.message}`)
            span.fail(`handshake: ${rpc.error.message}`, { code: rpc.error.code })
            ws.close(1008, 'Handshake failed')
          }
          return
        }

        // Post-handshake: dispatch JSON-RPC request
        if (typeof rpc.method === 'string') {
          void dispatcher.dispatch(ws, wireState, rpc as JsonRpcRequest)
        }
      })

      ws.on('close', (code: number, reason: Buffer) => {
        this.stop()
        const reasonStr = reason.toString()
        if (code === 1000) {
          span.ok({ code, reason: reasonStr })
        } else {
          span.fail(`ws close code=${code}`, { code, reason: reasonStr })
        }
        log.info(`Session closed code=${code} reason=${reasonStr}`)
      })

      ws.on('error', (err: Error) => {
        span.fail(err, { phase: 'ws-error' })
        log.error(`WebSocket error: ${err.message}`)
      })
    },

    stop(): void {
      if (keepaliveTimer !== null) {
        clearInterval(keepaliveTimer)
        keepaliveTimer = null
      }
    },

    onHandshakeOk(callback: () => void): void {
      handshakeOkCallbacks.push(callback)
    },
  }
}
