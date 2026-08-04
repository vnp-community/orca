// src/relay/__tests__/agent-connection-relay.test.ts
// Tests for the authenticate() logic in agent-connection-relay.ts.
// We test via duck-typed MockWs and MockReq without starting a real server.

import { describe, it, expect, vi } from 'vitest'
import { registerTraceSink, createTracer } from '../../shared/trace'
import type { TraceEvent, TraceSpan } from '../../shared/trace'

// ─── Minimal authenticate() extracted for isolated testing ──────────────────
// Replicated inline so we can test without spinning up a real WebSocket server.
// Kept in sync with the real implementation's signature in agent-connection-relay.ts
// (TASK-AG-013.1 added the trailing optional `span` param).
function authenticate(
  ws: { close: (code: number, reason: string) => void },
  req: { url?: string; headers: Record<string, string>; socket: { remoteAddress?: string } },
  expectedToken: string,
  log: { warn: (msg: string) => void },
  span?: TraceSpan
): boolean {
  const rawUrl = req.url ?? ''
  let queryToken = ''
  try {
    queryToken = new URL(`ws://localhost${rawUrl}`).searchParams.get('token') ?? ''
  } catch {}

  const authHeader  = (req.headers['authorization'] ?? '')
  const bearerToken = authHeader.replace(/^Bearer\s+/i, '').trim()
  const incoming    = queryToken || bearerToken
  const source: 'query' | 'header' | 'none' = queryToken ? 'query' : bearerToken ? 'header' : 'none'

  if (incoming !== expectedToken) {
    span?.fail('unauthorized', { source })
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress ?? 'unknown'}`)
    ws.close(1008, 'Unauthorized')
    return false
  }
  span?.step('tokenAccepted', { source })
  return true
}

// Local tracer replicate — same flow name as `relayConnTracer` in
// agent-connection-relay.ts, so emitted events' `flow` field matches.
const relayConnTracerForTest = createTracer('agent:connectionRelay')

// ─── Helpers ─────────────────────────────────────────────────────────────────
const TOKEN = 'my-relay-secret'

function makeWs()  { return { close: vi.fn() } }
function makeReq(url = '', authHeader = '') {
  return {
    url,
    headers: authHeader ? { authorization: authHeader } : {},
    socket: { remoteAddress: '127.0.0.1' },
  }
}
const mockLog = { warn: vi.fn() }

// ─── Tests ───────────────────────────────────────────────────────────────────
describe('authenticate() — URL query string', () => {
  it('accepts correct ?token= in URL', () => {
    const ws  = makeWs()
    const req = makeReq(`/orca-relay?token=${TOKEN}`)
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(true)
    expect(ws.close).not.toHaveBeenCalled()
  })

  it('rejects wrong ?token= in URL', () => {
    const ws  = makeWs()
    const req = makeReq(`/orca-relay?token=wrong`)
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(false)
    expect(ws.close).toHaveBeenCalledWith(1008, 'Unauthorized')
  })

  it('rejects empty ?token= in URL', () => {
    const ws  = makeWs()
    const req = makeReq(`/orca-relay?token=`)
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(false)
    expect(ws.close).toHaveBeenCalledWith(1008, 'Unauthorized')
  })

  it('rejects missing query param entirely', () => {
    const ws  = makeWs()
    const req = makeReq(`/orca-relay`)
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(false)
  })
})

describe('authenticate() — Authorization header', () => {
  it('accepts correct "Bearer <token>" header', () => {
    const ws  = makeWs()
    const req = makeReq('', `Bearer ${TOKEN}`)
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(true)
    expect(ws.close).not.toHaveBeenCalled()
  })

  it('accepts case-insensitive "bearer <token>" header', () => {
    const ws  = makeWs()
    const req = makeReq('', `bearer ${TOKEN}`)
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(true)
  })

  it('rejects wrong bearer token', () => {
    const ws  = makeWs()
    const req = makeReq('', `Bearer wrong-token`)
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(false)
    expect(ws.close).toHaveBeenCalledWith(1008, 'Unauthorized')
  })

  it('rejects empty Authorization header', () => {
    const ws  = makeWs()
    const req = makeReq('', '')
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(false)
  })
})

describe('authenticate() — priority and edge cases', () => {
  it('URL token takes priority over header when both present', () => {
    // URL has correct token, header has wrong token
    const ws  = makeWs()
    const req = { url: `/orca-relay?token=${TOKEN}`, headers: { authorization: 'Bearer wrong' }, socket: { remoteAddress: '127.0.0.1' } }
    expect(authenticate(ws, req, TOKEN, mockLog)).toBe(true)
  })

  it('calls ws.close(1008) on rejection', () => {
    const ws  = makeWs()
    const req = makeReq()
    authenticate(ws, req, TOKEN, mockLog)
    expect(ws.close).toHaveBeenCalledWith(1008, 'Unauthorized')
  })

  it('calls log.warn on rejection', () => {
    const ws  = makeWs()
    const req = makeReq()
    const log = { warn: vi.fn() }
    authenticate(ws, req, TOKEN, log)
    expect(log.warn).toHaveBeenCalledOnce()
  })

  it('does not call ws.close on successful auth', () => {
    const ws  = makeWs()
    const req = makeReq(`/orca-relay?token=${TOKEN}`)
    authenticate(ws, req, TOKEN, mockLog)
    expect(ws.close).not.toHaveBeenCalled()
  })
})

describe('authenticate() — agent:connectionRelay tracing', () => {
  it('span.step("tokenAccepted", {source:"query"}) khi token hợp lệ qua query string', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const span = relayConnTracerForTest.start({ remoteAddr: '127.0.0.1' })

    const ok = authenticate(makeWs(), makeReq(`/orca-relay?token=${TOKEN}`), TOKEN, mockLog, span)

    unregister()
    expect(ok).toBe(true)
    const step = events.find(e => e.level === 'step' && e.label === 'tokenAccepted')
    expect(step?.fields.source).toBe('query')
  })

  it('span.fail("unauthorized", {source:"none"}) khi thiếu token, KHÔNG có field nào chứa token thật', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const span = relayConnTracerForTest.start({ remoteAddr: '127.0.0.1' })

    const ok = authenticate(makeWs(), makeReq(''), TOKEN, mockLog, span)

    unregister()
    expect(ok).toBe(false)
    const fail = events.find(e => e.level === 'fail')
    expect(fail?.fields.source).toBe('none')
    expect(JSON.stringify(events)).not.toContain(TOKEN)
  })

  it('authenticate() vẫn hoạt động khi span không được truyền (backward-compat)', () => {
    expect(authenticate(makeWs(), makeReq(`/orca-relay?token=${TOKEN}`), TOKEN, mockLog)).toBe(true)
  })
})
