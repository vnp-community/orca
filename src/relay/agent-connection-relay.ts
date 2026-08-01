// src/relay/agent-connection-relay.ts
// Relay-websocket connection mode: Orca Server connects inbound to Agent.
//
// In this mode:
//   - Agent listens as a WebSocketServer on :AGENT_PORT/orca-relay
//   - Orca Server initiates the connection
//   - agentToken is a long-lived shared secret (not one-time use)
//   - Multiple sequential sessions are supported (server reconnects after drop)
//
// Token validation:
//   Accept from ?token=<token> query string OR Authorization: Bearer <token> header.

import { WebSocketServer } from 'ws'
import type WebSocket from 'ws'
import type { IncomingMessage } from 'node:http'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'

export async function listenRelay(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never> {
  // ORCH-013: Token is mandatory — no fallback to insecure 'relay-secret'.
  // If ORCA_AGENT_TOKEN is not set, the agent cannot operate in relay mode.
  const token = config.agentToken?.trim()
  if (!token) {
    log.error('FATAL: agentToken (ORCA_AGENT_TOKEN) is not set or empty.')
    log.error('In relay-websocket mode, a shared secret is required for authentication.')
    log.error('On the Dev Server, run:')
    log.error('  export ORCA_AGENT_TOKEN=$(openssl rand -hex 32)')
    log.error('  node ~/orca-agent/agent.js')
    process.exit(1)
  }

  return new Promise<never>((_, reject) => {
    const wss = new WebSocketServer({ port: config.agentPort, path: '/orca-relay' })

    wss.once('listening', () => {
      log.info(`✅ Relay server ready: ws://0.0.0.0:${config.agentPort}/orca-relay`)
      // ORCH-013: Do NOT log the token — security risk.
      log.info(`Orca UI config → Type: relay-websocket  URL: ws://<devServerHost>:${config.agentPort}/orca-relay`)
      log.info(`Set the token in Orca UI → Dev Server settings (matches ORCA_AGENT_TOKEN on this machine)`)
    })

    wss.on('connection', (ws: WebSocket, req: IncomingMessage) => {
      if (!authenticate(ws, req, token, log)) return

      const remoteAddr = req.socket.remoteAddress ?? 'unknown'
      log.info(`Orca Server connected from ${remoteAddr}`)

      const session = createSession(config, tools, log)
      session.start(ws)

      ws.once('close', (code: number) => {
        session.stop()
        log.info(`Orca Server disconnected from ${remoteAddr} (code=${code})`)
      })
    })

    wss.once('error', (err: Error) => {
      log.error(`Relay server fatal error: ${err.message}`)
      reject(err)
    })
  })
}

/**
 * Authenticate an incoming connection.
 * Accepts token from:
 *   1. URL query parameter: ?token=<token>
 *   2. HTTP header: Authorization: Bearer <token>
 *
 * Returns true if authenticated, false if rejected (ws already closed).
 */
function authenticate(
  ws: WebSocket,
  req: IncomingMessage,
  expectedToken: string,
  log: AgentLogger
): boolean {
  const rawUrl = req.url ?? ''

  // Parse ?token= from URL
  let queryToken = ''
  try {
    queryToken = new URL(`ws://localhost${rawUrl}`).searchParams.get('token') ?? ''
  } catch {
    // Ignore URL parse errors — treat as empty token
  }

  // Parse Authorization: Bearer <token>
  const authHeader = (req.headers['authorization'] ?? '') as string
  const bearerToken = authHeader.replace(/^Bearer\s+/i, '').trim()

  const incoming = queryToken || bearerToken

  if (incoming !== expectedToken) {
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress ?? 'unknown'}`)
    ws.close(1008, 'Unauthorized')
    return false
  }

  return true
}
