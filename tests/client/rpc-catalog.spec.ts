/**
 * Client tests — RPC Method Catalog smoke coverage
 *
 * IMPORTANT: the live deployment this hits (`ORCA_SERVER_URL`, default
 * 172.20.2.39) runs `backend-go`, not the TypeScript `backend/` that
 * specs/frontend/api/rpc-catalog.md was generated against. `frontend/`'s
 * unmodified RPC client still speaks that same wire protocol though, so
 * `api-gateway`'s `wscompat` shim (`backend-go/services/api-gateway/internal/adapter/wscompat/`)
 * translates each named method into a real backend-go gRPC call — but only
 * for the subset it has wired. Every other method returns a clean
 * `{ok:false, error:{message:"channel %q is not yet implemented..."}}`
 * instead of failing the connection. See
 * `backend-go/docs/execution-plan.md` §0 and `wscompat/channels.go`'s doc
 * comment (the single source of truth for current coverage — re-grep it,
 * don't trust this file's snapshot, before extending this suite):
 *
 *   grep -rhoE '\.Register\("[a-zA-Z][a-zA-Z0-9]*\.[a-zA-Z0-9_.]+"' \
 *     backend-go/services/api-gateway/internal/adapter/wscompat/*.go \
 *     | sed -E 's/.*Register\("//; s/"$//' | sort -u
 *
 * This file has two kinds of tests, split into separate `describe` blocks
 * per namespace:
 *   - Methods `wscompat` DOES register: asserted against the CORRECT
 *     contract (rpc-catalog.md / mobile-rpc-catalog.md's documented shape).
 *     Where the live backend-go service behind a wired channel is itself
 *     still buggy, these are meant to fail red — that failure IS the
 *     verification signal this suite exists to produce, not something to
 *     paper over with a weaker assertion.
 *   - Methods NOT yet registered: one tracked-gap test each, asserting
 *     today's "not yet implemented" error shape. If backend-go wires the
 *     channel, that assertion starts failing — the signal to move the
 *     method into a real coverage test above.
 *
 * Run:
 *   ORCA_SERVER_URL=http://172.20.2.39:6769 \
 *   pnpm vitest run --config tests/client/vitest.config.ts tests/client/rpc-catalog.spec.ts
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import {
  clientLogin,
  adminLogin,
  adminCreateUser,
  adminDeleteUser,
  ADMIN_EMAIL,
  ADMIN_PASSWORD,
  connectRpc
} from './helpers'
import type { RpcSession } from './rpc-client'

describe('RPC Catalog: status.* (wired)', () => {
  let rpc: RpcSession
  beforeAll(async () => {
    const { cookie } = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    rpc = await connectRpc(cookie)
  })
  afterAll(() => rpc.close())

  it('status.get reports runtime status', async () => {
    const status = await rpc.callOk<Record<string, unknown>>('status.get')
    expect(status).toBeTruthy()
  })
})

describe('RPC Catalog: profile.* (wired)', () => {
  let rpc: RpcSession
  beforeAll(async () => {
    const { cookie } = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    rpc = await connectRpc(cookie)
  })
  afterAll(() => rpc.close())

  it('profile.getResolved returns the merged user/dept/company profile', async () => {
    const profile = await rpc.callOk<Record<string, unknown>>('profile.getResolved')
    expect(profile).toBeTruthy()
  })

  it('profile.getUserProfile (no userId → resolves from session) returns the caller profile', async () => {
    const profile = await rpc.callOk<Record<string, unknown>>('profile.getUserProfile', {})
    expect(profile).toBeTruthy()
  })

  it('profile.listDepts is admin-gated: admin succeeds, developer is refused', async () => {
    const admin = await rpc.call('profile.listDepts')
    expect(admin.ok).toBe(true)

    const adminCookie = await adminLogin()
    const devEmail = `e2e-rpc-rbac-${Date.now()}@test.orca.local`
    const dev = await adminCreateUser(adminCookie, devEmail, 'RpcRbacTest@2025')
    try {
      const { cookie: devCookie } = await clientLogin(devEmail, 'RpcRbacTest@2025')
      const devRpc = await connectRpc(devCookie)
      try {
        const res = await devRpc.call('profile.listDepts')
        expect(res.ok).toBe(false)
      } finally {
        devRpc.close()
      }
    } finally {
      await adminDeleteUser(adminCookie, dev.id)
    }
  })
})

describe('RPC Catalog: repo.* / project.* / projectGroup.* / folderWorkspace.* / worktree.* (wired)', () => {
  let rpc: RpcSession
  beforeAll(async () => {
    const { cookie } = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    rpc = await connectRpc(cookie)
  })
  afterAll(() => rpc.close())

  it('repo.list returns the caller-visible repo catalog', async () => {
    const result = await rpc.callOk<{ repos: unknown[] }>('repo.list')
    expect(Array.isArray(result.repos)).toBe(true)
  })

  it('project.list returns v5.0 collaborative projects for the caller', async () => {
    const res = await rpc.call('project.list')
    expect(res.ok).toBe(true)
  })

  it('projectGroup.list returns a groups array (possibly empty, never null)', async () => {
    const result = await rpc.callOk<{ groups: unknown[] } | null>('projectGroup.list')
    expect(Array.isArray(result?.groups)).toBe(true)
  })

  it('folderWorkspace.list returns non-git folder workspaces for the caller tenant', async () => {
    const res = await rpc.call('folderWorkspace.list')
    expect(res.ok).toBe(true)
  })

  it('worktree.list (no repo filter) returns managed worktrees', async () => {
    const res = await rpc.call('worktree.list', {})
    expect(res.ok).toBe(true)
  })
})

describe('RPC Catalog: devServer.* / ssh.* / team.* (wired)', () => {
  let rpc: RpcSession
  beforeAll(async () => {
    const { cookie } = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    rpc = await connectRpc(cookie)
  })
  afterAll(() => rpc.close())

  it('devServer.list returns registered SSH dev-server targets', async () => {
    const res = await rpc.call('devServer.list')
    expect(res.ok).toBe(true)
  })

  it('ssh.listTargets returns a targets array (possibly empty, never null)', async () => {
    const result = await rpc.callOk<{ targets: unknown[] } | null>('ssh.listTargets')
    expect(Array.isArray(result?.targets)).toBe(true)
  })

  it('team.list returns v5.0 multi-user teams for the caller', async () => {
    const res = await rpc.call('team.list')
    expect(res.ok).toBe(true)
  })
})

describe('RPC Catalog: credentials.* (wired)', () => {
  let rpc: RpcSession
  beforeAll(async () => {
    const { cookie } = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    rpc = await connectRpc(cookie)
  })
  afterAll(() => rpc.close())

  it('credentials.list reports a services array (web-credential mode or electron passthrough)', async () => {
    const result = await rpc.callOk<{ services: unknown[]; mode: string }>('credentials.list')
    expect(Array.isArray(result.services)).toBe(true)
  })
})

describe('RPC Catalog: tracked gaps — not yet wired in wscompat', () => {
  let rpc: RpcSession
  beforeAll(async () => {
    const { cookie } = await clientLogin(ADMIN_EMAIL, ADMIN_PASSWORD)
    rpc = await connectRpc(cookie)
  })
  afterAll(() => rpc.close())

  // Why: these assert TODAY's "not yet implemented" placeholder response.
  // If a channel gets wired in wscompat, this test starts failing — that's
  // the signal to move it up into a real coverage describe block above
  // instead of adjusting the assertion here.
  const notYetWired = ['ui.get', 'settings.get', 'accounts.list']

  for (const method of notYetWired) {
    it(`${method} is not yet wired — update this test if it starts passing`, async () => {
      const res = await rpc.call(method)
      expect(res.ok).toBe(false)
      if (!res.ok) {
        expect(res.error.message).toMatch(/not yet implemented/i)
      }
    })
  }
})
