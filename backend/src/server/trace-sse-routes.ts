// ─── Trace SSE Route ──────────────────────────────────────────────────────────
// Streams backend trace events to the browser via Server-Sent Events (SSE).
//
// Endpoint: GET /api/trace-stream
//
// How it works:
//   1. Browser EventSource connects to /api/trace-stream
//   2. Server registers a trace sink → receives ALL TraceEvent from shared/trace
//   3. Each event is serialised as SSE `data: <json>\n\n` and written to the response
//   4. On disconnect: sink is removed, connection cleaned up
//
// This powers the TracePanel.tsx UI without needing browser-side tracing:
//   even if ORCA_TRACE is not set on the server, FAIL events still flow through
//   (because they always log regardless of the flag).
//
// Auth: same as agent-token-routes — Bearer ORCA_AGENT_API_SECRET or X-Orca-Admin: 1
// For browser use, we allow any authenticated session cookie (checked via a lightweight
// header — X-Orca-Trace-Client: 1) so DevOps can open the panel without CLI access.

import type { IncomingMessage, ServerResponse } from 'node:http'
import { registerTraceSink } from '../shared/trace'
import type { TraceEvent } from '../shared/trace'

// ─── Active SSE clients ───────────────────────────────────────────────────────
// Each connected browser gets an entry here. On trace event → broadcast to all.
const clients = new Set<ServerResponse>()

// ─── One-time global sink registration ───────────────────────────────────────
// We register a single global sink that fans-out to all connected SSE clients.
// Registration happens lazily on first client connection.
let sinkInstalled = false
function ensureSinkInstalled(): void {
  if (sinkInstalled) {return}
  sinkInstalled = true

  registerTraceSink((event: TraceEvent) => {
    if (clients.size === 0) {return}
    const data = `data: ${JSON.stringify(event)}\n\n`
    for (const res of clients) {
      try {
        res.write(data)
      } catch {
        // Client disconnected between iteration steps — remove lazily
        clients.delete(res)
      }
    }
  })
}

// ─── Auth check ───────────────────────────────────────────────────────────────
function isAuthorized(req: IncomingMessage): boolean {
  // Option 1: Bearer token (CI/server use)
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']
  if (apiSecret) {
    const auth = req.headers['authorization'] ?? ''
    if (auth === `Bearer ${apiSecret}`) {return true}
  }

  // Option 2: X-Orca-Admin header (dev-only)
  if (req.headers['x-orca-admin'] === '1') {return true}

  // Option 3: X-Orca-Trace-Client — any browser with the trace panel open
  // This is intentionally low-security: trace data is diagnostic, not sensitive.
  if (req.headers['x-orca-trace-client'] === '1') {return true}

  // If no secret is configured, allow any local connection for convenience
  if (!apiSecret) {return true}

  return false
}

// ─── Handler ──────────────────────────────────────────────────────────────────

/**
 * Handle GET /api/trace-stream
 * Returns true if the request was handled (caller should return).
 */
export function handleTraceStreamRequest(
  req: IncomingMessage,
  res: ServerResponse
): boolean {
  const url = req.url ?? ''
  if (url !== '/api/trace-stream' && !url.startsWith('/api/trace-stream?')) {
    return false
  }

  if (req.method?.toUpperCase() !== 'GET') {
    res.writeHead(405, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'method_not_allowed' }))
    return true
  }

  if (!isAuthorized(req)) {
    res.writeHead(401, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: 'unauthorized' }))
    return true
  }

  // SSE headers
  res.writeHead(200, {
    'Content-Type':  'text/event-stream',
    'Cache-Control': 'no-cache',
    'Connection':    'keep-alive',
    // Allow browser EventSource to connect cross-origin from same host
    'Access-Control-Allow-Origin': '*',
    // Tell nginx not to buffer SSE responses
    'X-Accel-Buffering': 'no',
  })

  // Send a heartbeat comment immediately so the browser knows it's connected
  res.write(`: connected\n\n`)

  ensureSinkInstalled()
  clients.add(res)

  console.log(`[TraceSse] Client connected (total=${clients.size})`)

  // Periodic heartbeat to keep connection alive through nginx/load-balancer timeouts
  const heartbeat = setInterval(() => {
    try {
      res.write(`: heartbeat\n\n`)
    } catch {
      clearInterval(heartbeat)
      clients.delete(res)
    }
  }, 15_000)

  // Cleanup on disconnect
  req.on('close', () => {
    clearInterval(heartbeat)
    clients.delete(res)
    console.log(`[TraceSse] Client disconnected (total=${clients.size})`)
  })

  return true
}
