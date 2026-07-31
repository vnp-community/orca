# TASK-004: Tạo `src/main/dev-server/ws-handshake.ts`

> **Status:** ✅ DONE (2026-07-26)
> **File created:** `src/main/dev-server/ws-handshake.ts`
> **TypeScript:** 0 errors

**Status:** ✅ DONE  
**Phase:** 2 — Transport Layer  
**Solution:** [SOL-AG-002](../solutions/SOL-AG-002-ws-transport-adapter.md) §3.2  
**Depends on:** TASK-001, TASK-003  
**Blocks:** TASK-005, TASK-006, TASK-008  

---

## Mục tiêu

Tạo handshake logic cho cả 2 modes:
- **`runOrcaInitiatorHandshake()`** — relay-websocket: Orca là WS client, gửi `agent.handshake` trước
- **`runOrcaReceiverHandshake()`** — direct-websocket: Orca là WS server, chờ agent gửi handshake

Handshake phải hoàn thành **trước** khi `createWebSocketTransport()` được gọi để
tránh race condition giữa handshake frame và regular RPC frames.

---

## File cần tạo

**Path:** `src/main/dev-server/ws-handshake.ts`

---

## Nội dung

```typescript
// src/main/dev-server/ws-handshake.ts
// Agent protocol handshake over a raw WebSocket connection.
// Must complete BEFORE wiring SshChannelMultiplexer via createWebSocketTransport().
//
// Two functions:
//   runOrcaInitiatorHandshake()  — Orca is WS CLIENT (relay-websocket)
//     Orca sends agent.handshake, waits for agent's result response.
//
//   runOrcaReceiverHandshake()   — Orca is WS SERVER (direct-websocket)
//     Orca waits for agent's agent.handshake, validates token, sends ok.

import type { WsLike } from './ws-transport'
import {
  encodeJsonRpcFrame,
  FrameDecoder,
  parseJsonRpcMessage,
  MessageType,
} from '../ssh/relay-protocol'
import type { AgentHandshakeParams } from '../../shared/agent-wire-protocol'
import {
  AGENT_HANDSHAKE_METHOD,
  AGENT_TIMEOUT_MS,
  AgentErrorCode,
} from '../../shared/agent-wire-protocol'

export type WsHandshakeInfo = {
  platform: string
  arch: string
  nodeVersion: string
  agentVersion: string
  sessionId: string
  agentToken?: string  // set by receiver handshake; used by AgentWebSocketServer to find slot
}

// ─── Initiator Handshake (relay-websocket) ────────────────────────────────────

/**
 * relay-websocket mode: Orca connected to agent's WS server.
 * Orca sends agent.handshake request and waits for agent's response.
 *
 * Flow:
 *   Orca → { method: 'agent.handshake', params: { orcaVersion } }  [seq=1, ack=0]
 *   Agent → { result: { ok: true, platform, arch, nodeVersion, agentVersion, sessionId } }
 *
 * Resolves: WsHandshakeInfo
 * Rejects: on error response | timeout | WS close before response
 */
export function runOrcaInitiatorHandshake(
  ws: WsLike,
  orcaVersion: string
): Promise<WsHandshakeInfo> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      ws.close()
      reject(new Error(
        `Agent handshake timed out after ${AGENT_TIMEOUT_MS}ms — ` +
        `agent did not respond to agent.handshake request`
      ))
    }, AGENT_TIMEOUT_MS)

    const decoder = new FrameDecoder((frame) => {
      if (frame.type !== MessageType.Regular) return  // ignore keepalives

      let msg: ReturnType<typeof parseJsonRpcMessage>
      try {
        msg = parseJsonRpcMessage(frame.payload)
      } catch {
        return  // invalid JSON — wait for next frame
      }

      // Only process response to our handshake (must have id field)
      if (!('id' in msg)) return
      clearTimeout(timer)

      if ('error' in msg && msg.error) {
        ws.close()
        reject(new Error(
          `Agent rejected handshake: ${msg.error.message} (code: ${msg.error.code})`
        ))
        return
      }

      const result = (msg as { result?: Record<string, unknown> }).result ?? {}
      resolve({
        platform:     (result['platform'] as string)     ?? 'linux',
        arch:         (result['arch'] as string)          ?? 'x64',
        nodeVersion:  (result['nodeVersion'] as string)   ?? 'unknown',
        agentVersion: (result['agentVersion'] as string)  ?? 'unknown',
        sessionId:    (result['sessionId'] as string)     ?? `sess-${Date.now()}`,
      })
    })

    ws.on('message', (data: Buffer) => decoder.feed(data))

    // Send handshake request: seq=1, ack=0
    const frame = encodeJsonRpcFrame(
      {
        jsonrpc: '2.0',
        id: 1,
        method: AGENT_HANDSHAKE_METHOD,
        params: { orcaVersion },
      },
      1,
      0
    )
    ws.send(frame)
  })
}

// ─── Receiver Handshake (direct-websocket) ────────────────────────────────────

/**
 * direct-websocket mode: Orca is the WS server, agent just connected.
 * Orca waits for agent's handshake request, validates token, sends ok.
 *
 * Flow:
 *   Agent → { method: 'agent.handshake', params: { agentToken, platform, ... } }
 *   Orca  → { result: { ok: true, orcaVersion, sessionId } }
 *         OR { error: { code: -33101, message: 'Authentication failed...' } }
 *
 * @param validateToken — returns true if the agentToken is registered (has a slot)
 *
 * Resolves: WsHandshakeInfo (includes agentToken so caller can find the slot)
 * Rejects: on invalid token | wrong first message | timeout
 */
export function runOrcaReceiverHandshake(
  ws: WsLike,
  validateToken: (token: string) => boolean,
  orcaVersion: string
): Promise<WsHandshakeInfo> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      ws.close()
      reject(new Error(
        `Agent did not send handshake within ${AGENT_TIMEOUT_MS}ms — ` +
        `ensure agent sends agent.handshake as first message after connecting`
      ))
    }, AGENT_TIMEOUT_MS)

    let outSeq = 0

    const decoder = new FrameDecoder((frame) => {
      if (frame.type !== MessageType.Regular) return  // ignore keepalives

      let msg: ReturnType<typeof parseJsonRpcMessage>
      try {
        msg = parseJsonRpcMessage(frame.payload)
      } catch {
        clearTimeout(timer)
        ws.close()
        reject(new Error('Protocol violation: received invalid JSON as first message'))
        return
      }

      // First message MUST be agent.handshake (a request with method, not a response)
      if (!('method' in msg) || msg.method !== AGENT_HANDSHAKE_METHOD) {
        clearTimeout(timer)
        ws.close()
        reject(new Error(
          `Protocol violation: first message must be '${AGENT_HANDSHAKE_METHOD}', ` +
          `got '${('method' in msg ? msg.method : '[response]')}'`
        ))
        return
      }

      clearTimeout(timer)
      const requestId = (msg as { id?: number }).id ?? 1
      const params = (msg as { params?: AgentHandshakeParams }).params

      // Validate auth token
      const agentToken = params?.agentToken ?? ''
      if (!validateToken(agentToken)) {
        outSeq++
        const errFrame = encodeJsonRpcFrame(
          {
            jsonrpc: '2.0',
            id: requestId,
            error: {
              code: AgentErrorCode.AuthFailed,
              message: 'Authentication failed: invalid or unregistered agent token',
            },
          },
          outSeq,
          0
        )
        ws.send(errFrame)
        ws.close()
        reject(new Error('Agent authentication failed: token not registered'))
        return
      }

      // Send handshake-ok
      const sessionId = `sess-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
      outSeq++
      const okFrame = encodeJsonRpcFrame(
        {
          jsonrpc: '2.0',
          id: requestId,
          result: { ok: true, orcaVersion, sessionId },
        },
        outSeq,
        0
      )
      ws.send(okFrame)

      resolve({
        platform:     params?.platform     ?? 'linux',
        arch:         params?.arch         ?? 'x64',
        nodeVersion:  params?.nodeVersion  ?? 'unknown',
        agentVersion: params?.agentVersion ?? 'unknown',
        sessionId,
        agentToken,
      })
    })

    ws.on('message', (data: Buffer) => decoder.feed(data))
  })
}
```

---

## Acceptance Criteria

- [x] File `src/main/dev-server/ws-handshake.ts` tồn tại
- [x] `WsHandshakeInfo` type export được
- [x] `runOrcaInitiatorHandshake()`: gửi `agent.handshake` request qua `ws.send()`
- [x] `runOrcaInitiatorHandshake()`: reject sau `AGENT_TIMEOUT_MS` (20s)
- [x] `runOrcaInitiatorHandshake()`: reject khi agent trả error response
- [x] `runOrcaReceiverHandshake()`: gửi handshake-ok khi token hợp lệ
- [x] `runOrcaReceiverHandshake()`: reject + close ws khi token sai (gửi error trước)
- [x] `runOrcaReceiverHandshake()`: reject khi first message không phải `agent.handshake`
- [x] TypeScript compile không lỗi
