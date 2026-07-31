// src/relay/__tests__/agent-connection-relay.test.ts
// Tests for the authenticate() logic in agent-connection-relay.ts.
// We test via duck-typed MockWs and MockReq without starting a real server.

import { describe, it, expect, vi, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'
import type { Socket } from 'node:net'

// ─── Minimal authenticate() extracted for isolated testing ──────────────────
// Replicated inline so we can test without spinning up a real WebSocket server.
function authenticate(
  ws: { close: (code: number, reason: string) => void },
  req: { url?: string; headers: Record<string, string>; socket: { remoteAddress?: string } },
  expectedToken: string,
  log: { warn: (msg: string) => void }
): boolean {
  const rawUrl = req.url ?? ''
  let queryToken = ''
  try {
    queryToken = new URL(`ws://localhost${rawUrl}`).searchParams.get('token') ?? ''
  } catch {}

  const authHeader  = (req.headers['authorization'] ?? '')
  const bearerToken = authHeader.replace(/^Bearer\s+/i, '').trim()
  const incoming    = queryToken || bearerToken

  if (incoming !== expectedToken) {
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress ?? 'unknown'}`)
    ws.close(1008, 'Unauthorized')
    return false
  }
  return true
}

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
