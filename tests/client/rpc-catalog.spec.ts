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

  it('profile.getUserProfile (no userId → resolves from session) returns the caller profile, or a clean not-found if none was ever set', async () => {
    // Why not always callOk: tenant-service's UserProfile is a per-user
    // *override* row (ports.go's UserProfileRepository doc comment), not a
    // record every user is guaranteed to have — a fresh bootstrap admin
    // with no department/preference override legitimately gets
    // TENANT_PROFILE_NOT_FOUND (get_user_profile.go), which is a correct
    // empty-state answer, not a bug. What this test actually guards is the
    // caller-identity fix (specs/backend-go/bugs/missing-v2/ follow-up):
    // the failure mode to catch is a *lookup crash*
    // (TENANT_PROFILE_LOOKUP_FAILED, an empty-string userId bound into a
    // UUID column), not "no override exists yet".
    const res = await rpc.call('profile.getUserProfile', {})
    if (!res.ok) {
      expect(res.error.message).toContain('TENANT_PROFILE_NOT_FOUND')
    }
  })

  // Why skipped, not asserted red: proving profile.listDepts actually
  // refuses a non-admin needs a real non-admin session, but auth-service's
  // CreateUser has no password field on its wire contract by design —
  // "there is no invite/reset-link flow implemented in this scaffold... a
  // random, never-returned password is generated" (create_user.go's
  // Execute doc comment) — so an admin-created account cannot be logged
  // into by any client, this test included, until a real credential-
  // issuance flow (invite email / forced reset / SSO) exists. Re-enable
  // once one does; leaving this red in the meantime would misreport a
  // known, documented scope gap as a live regression.
  it.skip('profile.listDepts is admin-gated: admin succeeds, a non-admin user is refused', async () => {
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

  it('repo.list requires a project — reaches OPA and gets a clean policy decision, not an eval crash', async () => {
    // Why not assert ok:true: backend-go's repo.list is project-scoped and
    // authorization-gated (unlike the old TS backend's tenant-wide,
    // params:null version) — succeeding needs a real project the caller is
    // a member of, which a fresh bootstrap tenant doesn't have. What THIS
    // test actually verifies is BUG-003 (specs/backend-go/bugs/missing-v2/):
    // a syntactically valid but nonexistent projectId must resolve to a
    // clean PROJECT_NOT_AUTHORIZED policy decision — proof OPA evaluated
    // for real — not PROJECT_POLICY_EVAL_FAILED (the OPA bundle itself
    // failing to load/evaluate).
    const res = await rpc.call('repo.list', { projectId: '00000000-0000-0000-0000-000000000000' })
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.error.message).not.toContain('PROJECT_POLICY_EVAL_FAILED')
    }
  })

  it('project.list returns v5.0 collaborative projects for the caller', async () => {
    const res = await rpc.call('project.list')
    expect(res.ok).toBe(true)
  })

  it('projectGroup.list returns a groups array (possibly empty, never null)', async () => {
    // Why a bare array, not {groups:[...]}: backend-go's channel handler
    // returns resp.GetGroups() directly as the RPC result (confirmed via
    // channels_tenant_project.go) — there is no wrapper object. An earlier
    // version of this test asserted the wrong shape ({groups:[...]}) and
    // treated the resulting failure as a backend bug; it wasn't — see
    // specs/backend-go/bugs/missing-v2/BUG-005's resolution.
    const result = await rpc.callOk<unknown[] | null>('projectGroup.list')
    expect(Array.isArray(result)).toBe(true)
  })

  it('folderWorkspace.list returns non-git folder workspaces for the caller tenant', async () => {
    const res = await rpc.call('folderWorkspace.list')
    expect(res.ok).toBe(true)
  })

  it('worktree.list requires a project — reaches OPA and gets a clean policy decision, not an eval crash', async () => {
    // Same reasoning as the repo.list test above.
    const res = await rpc.call('worktree.list', {
      projectId: '00000000-0000-0000-0000-000000000000'
    })
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.error.message).not.toContain('PROJECT_POLICY_EVAL_FAILED')
    }
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
    // Bare array, not {targets:[...]} — same reasoning as projectGroup.list
    // above (BUG-005's resolution); channels_repo_ssh_status_workspace.go
    // returns resp.GetSshTargets() directly.
    const result = await rpc.callOk<unknown[] | null>('ssh.listTargets')
    expect(Array.isArray(result)).toBe(true)
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
