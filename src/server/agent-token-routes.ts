/**
 * Agent Token HTTP Routes
 *
 * Exposes a REST endpoint so deployment scripts can register a direct-websocket
 * agent token AND wire it into DevServerManager so the agent appears in the UI.
 *
 * Flow (daemon-initiated):
 *   1. Script calls POST /api/agent-token { devServerId, name, ttl }
 *   2. We call DevServerManager.connectDaemonAgent() which:
 *      a. Finds or creates the DevServer record (persisted)
 *      b. Creates a DevServerRelayBridge with the generated token
 *      c. Emits 'devServer:added' + 'devServer:statusChanged' → UI updates
 *   3. When the daemon agent connects, bridge captures session → status=connected
 *
 * Endpoints:
 *   POST /api/agent-token   — Generate token + wire DevServer record
 *   GET  /api/agent-token   — List currently pending tokens (for debugging)
 *
 * Auth: requires ORCA_AGENT_API_SECRET env var as Bearer token.
 *       If not set, falls back to requiring X-Orca-Admin: 1 header (dev-only).
 */

import type { IncomingMessage, ServerResponse } from 'node:http'
import { generateAgentToken } from '../shared/agent-wire-protocol'
import type { AgentWebSocketServer } from '../main/dev-server/agent-ws-server'
import type { DevServerManager } from '../main/dev-server/dev-server-manager'
import { createTracer } from '../shared/trace'

const tokenTracer = createTracer('agentToken:register')

// ─── In-memory registry for debug listing ────────────────────────────────────
// Tracks tokens we registered via this API; actual auth state is in agentWsServer.
const pendingMeta = new Map<string, { devServerId: string; createdAt: number; expiresAt: number }>()

// ─── Auth helper ──────────────────────────────────────────────────────────────
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']
  if (apiSecret) {
    const auth = req.headers['authorization'] ?? ''
    return auth === `Bearer ${apiSecret}`
  }
  // Dev fallback: X-Orca-Admin: 1 header (no secret configured)
  return req.headers['x-orca-admin'] === '1'
}

function sendJson(res: ServerResponse, status: number, body: unknown): void {
  const json = JSON.stringify(body)
  res.writeHead(status, {
    'Content-Type':   'application/json',
    'Content-Length': Buffer.byteLength(json),
    'Cache-Control':  'no-store',
  })
  res.end(json)
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = ''
    req.on('data', (chunk: Buffer) => { data += chunk.toString() })
    req.on('end', () => resolve(data))
    req.on('error', reject)
  })
}

// ─── Route handler ────────────────────────────────────────────────────────────

async function handleAgentTokenRequest(
  req: IncomingMessage,
  res: ServerResponse,
  agentWsServer: AgentWebSocketServer,
  devServerManager: DevServerManager | null
): Promise<void> {
  // Auth check
  if (!isAuthorized(req)) {
    sendJson(res, 401, {
      error: 'unauthorized',
      message: 'Missing or invalid auth. Set ORCA_AGENT_API_SECRET or pass X-Orca-Admin: 1 header.',
    })
    return
  }

  const method = req.method?.toUpperCase() ?? 'GET'

  // ── GET /api/agent-token — list pending tokens (debug) ─────────────────────
  if (method === 'GET') {
    const now = Date.now()
    const tokens = Array.from(pendingMeta.entries())
      .filter(([, meta]) => meta.expiresAt > now)
      .map(([token, meta]) => ({
        token,
        devServerId: meta.devServerId,
        expiresIn: Math.round((meta.expiresAt - now) / 1000),
      }))
    sendJson(res, 200, { tokens })
    return
  }

  // ── POST /api/agent-token — generate token + wire DevServer ────────────────
  if (method === 'POST') {
    let body: Record<string, unknown> = {}
    try {
      const raw = await readBody(req)
      if (raw.trim()) body = JSON.parse(raw) as Record<string, unknown>
    } catch {
      sendJson(res, 400, { error: 'bad_request', message: 'Invalid JSON body' })
      return
    }

    const devServerId = (body['devServerId'] as string | undefined) ?? 'dev-local'
    const name        = (body['name']        as string | undefined) ?? `Dev Server (${devServerId})`
    const ttlSec      = Math.min(Number(body['ttl'] ?? 300), 600)   // max 10 min
    const expiresAt   = Date.now() + ttlSec * 1000
    const token       = generateAgentToken(devServerId)

    const span = tokenTracer.start({ devServerId, name, ttl: ttlSec })

    if (devServerManager) {
      // ── Path A: Full wiring via DevServerManager ──────────────────────────
      // This registers the token in AgentWebSocketServer AND creates/updates
      // the DevServer record so it appears in the UI.
      pendingMeta.set(token, { devServerId, createdAt: Date.now(), expiresAt })

      // connectDaemonAgent is non-blocking: returns immediately after wiring,
      // fires 'devServer:statusChanged' → 'connected' when agent actually connects.
      const { created } = await devServerManager.connectDaemonAgent({
        devServerId,
        name,
        token,
        ttlMs: ttlSec * 1000,
      })

      span.ok({ token: token.slice(0, 16) + '...', created })
      console.log(
        `[AgentTokenAPI] Token registered via DevServerManager: ${token} ` +
        `(devServerId=${devServerId}, name="${name}", ttl=${ttlSec}s, created=${created})`
      )

      // Clean up pending meta when token expires (rough TTL)
      setTimeout(() => pendingMeta.delete(token), ttlSec * 1000)

      sendJson(res, 200, {
        token,
        devServerId,
        name,
        expiresIn: ttlSec,
        created,
        agentCommand: `ORCA_URL=wss://<orca-host>/agent AGENT_TOKEN=${token} node agent.js`,
      })
    } else {
      // ── Path B: Fallback — raw slot registration (no UI wiring) ───────────
      // DevServerManager not available (unlikely in production but possible
      // in tests or edge-case startup order). Agent can connect but won't
      // appear in the Dev Servers list.
      agentWsServer.registerSlot(
        token,
        (_mux, info) => {
          pendingMeta.delete(token)
          span.step('agent-connected', { platform: String(info.platform), version: String(info.agentVersion ?? '?') })
        },
        (reason) => {
          pendingMeta.delete(token)
          span.fail(`token expired: ${reason}`, { devServerId })
        }
      )

      pendingMeta.set(token, { devServerId, createdAt: Date.now(), expiresAt })
      setTimeout(() => pendingMeta.delete(token), ttlSec * 1000)

      span.step('registered-fallback', { devServerId, ttl: ttlSec })

      sendJson(res, 200, {
        token,
        devServerId,
        name,
        expiresIn: ttlSec,
        created: false,
        warning: 'DevServerManager not available — agent will connect but not appear in UI',
        agentCommand: `ORCA_URL=wss://<orca-host>/agent AGENT_TOKEN=${token} node agent.js`,
      })
    }
    return
  }

  sendJson(res, 405, { error: 'method_not_allowed' })
}

// ─── Factory ──────────────────────────────────────────────────────────────────

/**
 * Create an apiHandler compatible with HttpServerOptions.apiHandler.
 *
 * Accepts both AgentWebSocketServer (required for raw slot registration fallback)
 * and DevServerManager (required for full UI wiring via connectDaemonAgent).
 *
 * Returns a synchronous function that:
 *   - Returns true  (+ handles the response async) for /api/agent-token requests
 *   - Returns false for everything else (caller falls through to static handler)
 */
export function createAgentTokenApiHandler(
  agentWsServer: AgentWebSocketServer,
  devServerManager: DevServerManager | null = null
): (req: IncomingMessage, res: ServerResponse) => boolean {
  console.log(
    '[AgentTokenAPI] Route registered: POST /api/agent-token' +
    (devServerManager ? ' (DevServerManager wiring enabled)' : ' (fallback: no DevServerManager)')
  )

  return (req: IncomingMessage, res: ServerResponse): boolean => {
    const url = req.url ?? ''
    if (url !== '/api/agent-token' && !url.startsWith('/api/agent-token?')) {
      return false  // not ours
    }

    void handleAgentTokenRequest(req, res, agentWsServer, devServerManager)
      .catch((err: Error) => {
        console.error('[AgentTokenAPI] Unhandled error:', err.message)
        if (!res.headersSent) sendJson(res, 500, { error: 'internal_error' })
      })

    return true  // we handled it
  }
}
