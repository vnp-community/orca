// ─── agent-token-routes-tracing.test.ts ───────────────────────────────────────
// Tests for GET /api/agent-token tracing (TASK-BE-013.3/013.4).
//
// The GET branch (list pending tokens, debug) previously had no tracer at
// all. It now reuses the existing `tokenTracer` (`agentToken:register`) —
// same tracer as the POST branch — instead of creating a new one, per
// CR-TRACE-000 §4 ("1 tracer = 1 sub-flow" in the forward direction, but no
// duplicate tracers for the same sub-domain).

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import * as http from 'node:http'
import { createAgentTokenApiHandler } from '../agent-token-routes'
import type { AgentWebSocketServer } from '../../main/dev-server/agent-ws-server'
import { registerTraceSink, type TraceEvent } from '../../shared/trace'

// ── HTTP test helper (same pattern as push-api-routes.test.ts) ────────────────

function doRequest(
  server: http.Server,
  method: string,
  path: string,
  headers?: Record<string, string>
): Promise<{ statusCode: number; body: string }> {
  return new Promise((resolve, reject) => {
    const addr = server.address() as { port: number }
    const req = http.request(
      { host: '127.0.0.1', port: addr.port, path, method, headers },
      (res) => {
        let data = ''
        res.on('data', (c) => (data += c))
        res.on('end', () => resolve({ statusCode: res.statusCode!, body: data }))
      }
    )
    req.on('error', reject)
    req.end()
  })
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

// ── Fake AgentWebSocketServer (Path B registration target) ────────────────────

function makeAgentWsServer(): AgentWebSocketServer {
  return { registerSlot: vi.fn(() => () => {}) } as unknown as AgentWebSocketServer
}

describe('GET /api/agent-token — tokenTracer (agentToken:register) tracing', () => {
  let server: http.Server
  const originalSecret = process.env['ORCA_AGENT_API_SECRET']

  beforeEach(
    () =>
      new Promise<void>((resolve) => {
        process.env['ORCA_AGENT_API_SECRET'] = 'test-secret'
        const apiHandler = createAgentTokenApiHandler(makeAgentWsServer(), null)
        server = http.createServer((req, res) => {
          if (!apiHandler(req, res)) {
            res.writeHead(404)
            res.end()
          }
        })
        server.listen(0, '127.0.0.1', resolve)
      })
  )

  afterEach(
    () =>
      new Promise<void>((resolve, reject) => {
        if (originalSecret === undefined) delete process.env['ORCA_AGENT_API_SECRET']
        else process.env['ORCA_AGENT_API_SECRET'] = originalSecret
        server.close((err) => (err ? reject(err) : resolve()))
      })
  )

  it('emits at least 1 trace event: tokenTracer.start({op:"list"}) then .ok({count}) matching the response body length', async () => {
    // Register 2 tokens via POST (Path B — no DevServerManager) so pendingMeta
    // has known, non-expired entries to list.
    const post = (devServerId: string) =>
      new Promise<void>((resolve, reject) => {
        const addr = server.address() as { port: number }
        const body = JSON.stringify({ devServerId, ttl: 600 })
        const req = http.request(
          {
            host: '127.0.0.1',
            port: addr.port,
            path: '/api/agent-token',
            method: 'POST',
            headers: {
              Authorization: 'Bearer test-secret',
              'Content-Type': 'application/json',
              'Content-Length': Buffer.byteLength(body).toString(),
            },
          },
          (res) => {
            res.on('data', () => {})
            res.on('end', resolve)
          }
        )
        req.on('error', reject)
        req.write(body)
        req.end()
      })

    await post('dev-list-1')
    await post('dev-list-2')

    const { events, stop } = captureTraceEvents()
    const res = await doRequest(server, 'GET', '/api/agent-token', {
      Authorization: 'Bearer test-secret',
    })
    stop()

    expect(res.statusCode).toBe(200)
    const parsed = JSON.parse(res.body) as { tokens: unknown[] }
    expect(parsed.tokens.length).toBeGreaterThanOrEqual(2)

    const startEvent = events.find(
      (e) => e.flow === 'agentToken:register' && e.level === 'start' && e.fields.op === 'list'
    )
    expect(startEvent).toBeDefined()

    const okEvent = events.find(
      (e) => e.flow === 'agentToken:register' && e.level === 'ok' && e.id === startEvent!.id
    )
    expect(okEvent).toBeDefined()
    expect(okEvent?.fields.count).toBe(parsed.tokens.length)
  })

  it('does not create a new tracer: every trace event from this GET request uses the existing "agentToken:register" flow', async () => {
    const { events, stop } = captureTraceEvents()

    const res = await doRequest(server, 'GET', '/api/agent-token', {
      Authorization: 'Bearer test-secret',
    })
    stop()

    expect(res.statusCode).toBe(200)
    expect(events.length).toBeGreaterThan(0)
    const flowNames = new Set(events.map((e) => e.flow))
    expect(flowNames.size).toBe(1)
    expect(flowNames.has('agentToken:register')).toBe(true)
  })
})
