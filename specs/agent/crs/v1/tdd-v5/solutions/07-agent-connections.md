# SOL-07: agent-connection-direct.ts + agent-connection-relay.ts

**TDD Ref:** TDD-AG-03  
**Files:**
- `src/relay/agent-connection-direct.ts` [NEW]
- `src/relay/agent-connection-relay.ts` [NEW]  
**Mức độ:** 🟡 Trung bình  
**Thời gian ước tính:** 2h

---

## agent-connection-direct.ts

```typescript
// src/relay/agent-connection-direct.ts

import WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'

/**
 * Mode 1: direct-websocket
 * Agent connects to Orca Server at ORCA_URL.
 * Token is one-time use — exit on disconnect so systemd can restart with fresh token.
 */
export async function connectDirect(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never> {
  if (!config.agentToken) {
    log.error('AGENT_TOKEN required for direct-websocket mode')
    log.error('Get a token: bash deploy/dev/scripts/connect-agent.sh')
    process.exit(1)
  }

  let lastHandshakeOk = false

  log.info(`Connecting to ${config.orcaUrl}...`)

  const ws = new WebSocket(config.orcaUrl, {
    headers: { 'User-Agent': 'orca-dev-agent/2.1.0' },
    rejectUnauthorized: config.tlsRejectUnauthorized,
  })

  const session = createSession(config, tools, log)
  session.onHandshakeOk(() => { lastHandshakeOk = true })
  session.start(ws)

  ws.once('close', (code: number) => {
    session.stop()
    if (code === 1000) {
      log.info('Connection closed cleanly (code=1000). Shutting down.')
      process.exit(0)
    }
    if (lastHandshakeOk) {
      log.warn(`Connection dropped after handshake (code=${code}) — exit(2) for systemd restart`)
    } else {
      log.error(`Connection closed before handshake (code=${code}) — token rejected/expired. exit(2)`)
    }
    // NEVER retry internally: token is one-time use, systemd Restart=always + start.sh gets fresh token
    setTimeout(() => process.exit(2), 200)
  })

  // Never resolves — process runs until exit()
  return new Promise<never>(() => {})
}
```

---

## agent-connection-relay.ts

```typescript
// src/relay/agent-connection-relay.ts

import { WebSocketServer, type WebSocket } from 'ws'
import type { IncomingMessage } from 'node:http'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'

/**
 * Mode 2: relay-websocket
 * Agent listens on :AGENT_PORT/orca-relay; Orca Server connects to agent.
 * Token is a long-lived shared secret (not one-time use).
 * Server stays up; new session per incoming connection.
 */
export async function listenRelay(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never> {
  const token = config.agentToken || 'relay-secret'

  return new Promise<never>((_, reject) => {
    const wss = new WebSocketServer({ port: config.agentPort, path: '/orca-relay' })

    wss.once('listening', () => {
      log.info(`✅ Relay server ready: ws://0.0.0.0:${config.agentPort}/orca-relay`)
      log.info(`Orca UI config: Type=relay-websocket  URL=ws://${config.devServerId}:${config.agentPort}/orca-relay?token=${token}`)
    })

    wss.on('connection', (ws: WebSocket, req: IncomingMessage) => {
      if (!authenticate(ws, req, token, log)) return

      const remoteAddr = req.socket.remoteAddress ?? 'unknown'
      log.info(`Orca connected from ${remoteAddr}`)

      const session = createSession(config, tools, log)
      session.start(ws)

      ws.once('close', (code: number) => {
        session.stop()
        log.info(`Orca disconnected from ${remoteAddr} (code=${code})`)
      })
    })

    wss.once('error', (err: Error) => {
      log.error(`Relay server error: ${err.message}`)
      reject(err)
    })
  })
}

/**
 * Authenticate incoming connection.
 * Token accepted from:
 *   1. URL query: ?token=<token>
 *   2. Header: Authorization: Bearer <token>
 */
function authenticate(
  ws: WebSocket,
  req: IncomingMessage,
  expectedToken: string,
  log: AgentLogger
): boolean {
  const rawUrl = req.url ?? ''
  let qToken = ''
  try {
    qToken = new URL(`ws://localhost${rawUrl}`).searchParams.get('token') ?? ''
  } catch { /* ignore URL parse errors */ }

  const authHeader = (req.headers['authorization'] ?? '') as string
  const bToken = authHeader.replace(/^Bearer\s+/i, '')
  const incoming = qToken || bToken

  if (incoming !== expectedToken) {
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress}`)
    ws.close(1008, 'Unauthorized')
    return false
  }
  return true
}
```

---

## Definition of Done

- [x] `agent-connection-direct.ts` created + `tsc` passes
- [x] `agent-connection-relay.ts` created + `tsc` passes
- [x] Tests for `connectDirect`:
  - exit(1) when AGENT_TOKEN empty
  - session.start() called on connection
  - exit(0) on code=1000
  - exit(2) on unexpected close post-handshake
  - exit(2) on close pre-handshake
- [x] Tests for `listenRelay`:
  - Rejects wrong token → ws.close(1008)
  - Accepts token from querystring
  - Accepts token from Authorization header
  - Each connection gets independent session
