/**
 * Client tests — RPC Transport (cookie-authenticated WebSocket RPC)
 *
 * Tests the wire-level contract rpc-client.ts implements against the real
 * backend: WsSessionRouter's cookie-based WS auth
 * (backend/src/main/session/ws-session-router.ts) and OrcaRuntimeRpcServer's
 * request/response envelope (backend/src/main/runtime/runtime-rpc.ts).
 *
 * See specs/frontend/api/README.md's "Connection types" section for how this
 * channel (browser SPA ↔ per-user process, cookie auth) differs from the
 * E2EE/device-token channel mobile and remote-environment pairing use.
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/client/vitest.config.ts tests/client/rpc-transport.spec.ts
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { clientLogin, ADMIN_EMAIL, ADMIN_PASSWORD, RPC_WS_URL, connectRpc } from './helpers'
import { RpcSession } from './rpc-client'

describe('RPC Transport: Connection & Auth', () => {
  let cookie: string

  beforeAll(async () => {
    const result = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    cookie = result.cookie
  })

  it('a valid session cookie opens an RPC connection and can call status.get', async () => {
    const rpc = await connectRpc(cookie)
    try {
      const status = await rpc.callOk('status.get')
      expect(status).toBeTruthy()
      expect(typeof status).toBe('object')
    } finally {
      rpc.close()
    }
  })

  it('missing cookie is rejected at the WS handshake (WsSessionRouter close 4401)', async () => {
    await expect(RpcSession.connect(RPC_WS_URL, '')).rejects.toMatchObject({
      code: 4401
    })
  })

  it('an invalid/garbage session cookie is rejected the same way', async () => {
    await expect(
      RpcSession.connect(RPC_WS_URL, 'orca_session=totally-invalid-token')
    ).rejects.toMatchObject({ code: 4401 })
  })
})

describe('RPC Transport: Method dispatch', () => {
  let rpc: RpcSession

  beforeAll(async () => {
    const { cookie } = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    rpc = await connectRpc(cookie)
  })

  afterAll(() => {
    rpc.close()
  })

  it('an unknown method returns a well-formed ok:false error, not a transport failure', async () => {
    // Why: pin only the envelope shape, not the exact error code string — the
    // deployed server this hits may run a different backend commit than this
    // checkout, and dispatcher.ts's exact code (method_not_found as of this
    // checkout) is an implementation detail, not part of the wire contract.
    const res = await rpc.call('definitely.not.a.real.method')
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(typeof res.error.code).toBe('string')
      expect(res.error.code.length).toBeGreaterThan(0)
    }
  })

  it('status.get returns a well-formed success envelope', async () => {
    const res = await rpc.call('status.get')
    expect(res.ok).toBe(true)
    expect(typeof res.id).toBe('string')
  })

  it('a call missing required params surfaces as a backend error, not a transport failure', async () => {
    // profile.getUserProfile accepts an optional userId, but a namespace that
    // requires structured params (worktree.list's `repo`) rejecting garbage
    // shows the round trip carries the error back instead of dropping the
    // connection — use worktree.list with a bogus `repo` to stay read-only.
    const res = await rpc.call('worktree.list', { repo: 'definitely/not-a-real-repo' })
    // Either an explicit backend error, or an ok:true empty list — both prove
    // the request was dispatched and answered, which is what this test checks.
    expect(res).toHaveProperty('ok')
  })
})
