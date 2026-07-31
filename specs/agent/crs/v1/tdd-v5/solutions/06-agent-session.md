# SOL-06: agent-session.ts — Session Handler

**TDD Ref:** TDD-AG-03, TDD-AG-04  
**File:** `src/relay/agent-session.ts` [NEW]  
**Mức độ:** 🟡 Trung bình  
**Thời gian ước tính:** 2h

---

## Full Implementation

```typescript
// src/relay/agent-session.ts

import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentLogger } from './agent-logger'
import { createWireState, decodeFrame, encodeDataFrame, encodeKeepaliveFrame, parseJsonPayload } from './agent-wire'
import { createRpcDispatcher } from './agent-rpc-dispatch'
import {
  AGENT_HANDSHAKE_METHOD,
  AGENT_KEEPALIVE_INTERVAL_MS,
} from '../shared/agent-wire-protocol'
import { MessageType } from '../main/ssh/relay-protocol'

export interface AgentSession {
  /** Start session on an already-open WebSocket. */
  start(ws: WebSocket): void
  /** Clean up keepalive timer. */
  stop(): void
  /** Register callback fired when handshake-ok received from server. */
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
        agentVersion:  '2.1.0',
        platform:      process.platform,
        arch:          process.arch,
        nodeVersion:   process.version,
        capabilities:  ['fs', 'git', 'preflight'] as string[],
        // agentToken sent only in direct-websocket mode
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
      const wireState = createWireState()

      // Handshake: send when WS opens (direct mode) or immediately (relay mode)
      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        sendHandshake(ws, wireState)
        startKeepalive(ws, wireState)
      } else {
        ws.once('open', () => {
          log.info('WebSocket opened')
          sendHandshake(ws, wireState)
          startKeepalive(ws, wireState)
        })
      }

      ws.on('message', (data: Buffer | string) => {
        if (!Buffer.isBuffer(data)) return  // Agent only accepts binary frames

        const frame = decodeFrame(wireState, data)
        if (!frame) return  // Malformed frame — ignore

        // Respond to keepalives immediately (maintains ACK progress)
        if (frame.type === MessageType.KeepAlive) {
          ws.send(encodeKeepaliveFrame(wireState))
          return
        }

        if (frame.payload.length === 0) return  // Empty data frame — ignore

        const rpc = parseJsonPayload<{
          id: string | number | null
          result?: { ok?: boolean; sessionId?: string }
          error?: { code: number; message: string }
          method?: string
          params?: Record<string, unknown>
        }>(frame.payload)

        if (!rpc) {
          log.warn('Received malformed JSON frame — ignoring')
          return
        }

        // Pre-handshake: only process handshake result (id=1)
        if (!handshakeDone) {
          if (rpc.result?.ok === true) {
            handshakeDone = true
            log.info(`Handshake OK: sessionId=${rpc.result.sessionId ?? 'unknown'}`)
            handshakeOkCallbacks.forEach(cb => cb())
          } else if (rpc.error) {
            log.error(`Handshake failed: code=${rpc.error.code} message=${rpc.error.message}`)
            ws.close(1008, 'Handshake failed')
          }
          return
        }

        // Post-handshake: dispatch JSON-RPC
        if (typeof rpc.method === 'string') {
          void dispatcher.dispatch(ws, wireState, rpc as Parameters<typeof dispatcher.dispatch>[2])
        }
      })

      ws.on('close', (code, reason) => {
        this.stop()
        log.info(`Session closed code=${code} reason=${reason.toString()}`)
      })

      ws.on('error', (err) => {
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
```

---

## Vitest Test File Pattern

```typescript
// src/relay/__tests__/agent-session.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { EventEmitter } from 'node:events'

// Mock WebSocket
class MockWs extends EventEmitter {
  readyState = 1  // OPEN
  send = vi.fn()
  close = vi.fn()
}

describe('createSession', () => {
  it('sends handshake immediately when ws is already OPEN', () => {
    const ws = new MockWs() as any
    const session = createSession(mockConfig, mockTools, mockLog)
    session.start(ws)
    expect(ws.send).toHaveBeenCalledOnce()  // handshake frame
    const call = ws.send.mock.calls[0][0] as Buffer
    const json = JSON.parse(call.subarray(13).toString())  // skip 13-byte header
    expect(json.method).toBe('agent.handshake')
  })

  it('sets handshakeDone on result.ok=true', () => { ... })
  it('closes ws on handshake error', () => { ... })
  it('dispatches RPC after handshake', () => { ... })
  it('responds to KeepAlive frames with keepalive', () => { ... })
  it('stop() clears keepalive interval', () => { ... })
  it('onHandshakeOk callback fires after successful handshake', () => { ... })
})
```

---

## Definition of Done

- [x] `src/relay/agent-session.ts` created
- [x] `tsc` passes
- [x] ≥ 15 tests pass (handshake, keepalive, dispatch, stop)
