# T12 — Write profile-rpc.test.ts

**Phase:** 2C  
**Effort:** ~45 min  
**Depends on:** T01 (RPC wiring — optional, can run independently)  
**Solution ref:** [01-tdd14-profile-hierarchy.md §2.1](../solutions/01-tdd14-profile-hierarchy.md)  
**TDD ref:** TDD-14 (profile-rpc-handler.ts)

---

## Mục tiêu

Viết RPC handler tests cho `profile-rpc-handler.ts` — access control, locked sections, invalidate.

**Target: ≥ 12 tests**

---

## Files Cần Đọc Trước

1. `src/main/profile/profile-rpc-handler.ts` — đọc toàn bộ (method names, RpcContext, createProfileMethods)
2. `src/main/project/__tests__/project-rpc.test.ts` — **pattern tái sử dụng** (mock pattern + findHandler)
3. `src/main/profile/OrcaProfile.ts` — OrcaProfile, ResolvedProfile types
4. `src/main/auth/auth-manager.ts` — AuthManager interface (admin check)

---

## File Cần Tạo

### `src/main/profile/__tests__/profile-rpc.test.ts`

```typescript
/**
 * Tests for profile RPC handlers (TDD-14) — T12
 *
 * Mock-based tests following project-rpc.test.ts pattern.
 * ≥ 12 tests covering admin access control + profile merge delegation.
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { createProfileMethods } from '../profile-rpc-handler'
import type { RpcContext } from '../../runtime/rpc/core'
import type { ResolvedProfile, OrcaProfile } from '../OrcaProfile'

// ── Helpers ────────────────────────────────────────────────────────────────────

const FAKE_RESOLVED: ResolvedProfile = {
  _sources: { 'agent.model': 'company' },
  _resolvedAt: Date.now(),
  agent: { model: 'claude-opus-4-5' },
}

const FAKE_COMPANY_PROFILE: OrcaProfile = {
  agent: { model: 'claude-opus-4-5' },
  security: { allowShellEscape: false, allowNetworkAccess: true },
}

function makeCtx(userId: string, role: 'admin' | 'developer' | 'viewer' = 'developer'): RpcContext {
  return { userId, user: { id: userId, role } } as unknown as RpcContext
}

function makeProfileService(overrides = {}) {
  return {
    getUserProfile: vi.fn().mockResolvedValue({}),
    setUserProfile: vi.fn().mockResolvedValue(undefined),
    getCompanyProfile: vi.fn().mockResolvedValue(FAKE_COMPANY_PROFILE),
    setCompanyProfile: vi.fn().mockResolvedValue(undefined),
    getDeptProfile: vi.fn().mockResolvedValue({}),
    setDeptProfile: vi.fn().mockResolvedValue(undefined),
    getCompanyProfileForUser: vi.fn().mockResolvedValue(FAKE_COMPANY_PROFILE),
    getDeptProfileForUser: vi.fn().mockResolvedValue({}),
    listDepartments: vi.fn().mockResolvedValue([]),
    setUserDepartment: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function makeProfileResolver(resolved = FAKE_RESOLVED) {
  return {
    resolve: vi.fn().mockResolvedValue(resolved),
    invalidate: vi.fn(),
  }
}

function makeAuthManager() {
  return {
    userStore: {
      isAdmin: vi.fn().mockResolvedValue(false),
    },
  }
}

function makeAdminAuthManager() {
  return {
    userStore: {
      isAdmin: vi.fn().mockResolvedValue(true),
    },
  }
}

function findHandler(methods: ReturnType<typeof createProfileMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) throw new Error(`Method not found: ${name}`)
  return method.handler
}

// ── Tests ──────────────────────────────────────────────────────────────────────

describe('profile RPC handlers', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── profile.getResolved ────────────────────────────────────────────────────
  describe('profile.getResolved', () => {
    it('returns resolved profile for authenticated user', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.getResolved')
      const result = await handler({}, makeCtx('user-001'))
      expect(result._resolvedAt).toBeDefined()
      expect(resolver.resolve).toHaveBeenCalledWith('user-001')
    })

    it('returns empty-source profile when no company configured', async () => {
      const emptyResolver = makeProfileResolver({ _sources: {}, _resolvedAt: Date.now() })
      const methods = createProfileMethods(makeProfileService() as any, emptyResolver as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.getResolved')
      const result = await handler({}, makeCtx('user-001'))
      expect(Object.keys(result._sources)).toHaveLength(0)
    })
  })

  // ── profile.updateCompany ──────────────────────────────────────────────────
  describe('profile.updateCompany', () => {
    it('admin can update company profile', async () => {
      const svc = makeProfileService()
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any, makeAdminAuthManager() as any)
      const handler = findHandler(methods, 'profile.updateCompany')
      await handler({ profile: { agent: { model: 'gemini-2.5' } } }, makeCtx('admin-001', 'admin'))
      expect(svc.setCompanyProfile).toHaveBeenCalled()
    })

    it('non-admin receives 403 PROFILE_UNAUTHORIZED', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.updateCompany')
      await expect(handler({ profile: {} }, makeCtx('user-001', 'developer'))).rejects.toThrow('PROFILE_UNAUTHORIZED')
    })
  })

  // ── profile.updateUser ─────────────────────────────────────────────────────
  describe('profile.updateUser', () => {
    it('user can update their own profile', async () => {
      const svc = makeProfileService()
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.updateUser')
      await handler({ userId: 'user-001', profile: { editor: { fontSize: 14 } } }, makeCtx('user-001'))
      expect(svc.setUserProfile).toHaveBeenCalledWith('user-001', { editor: { fontSize: 14 } })
    })

    it('rejects update of locked security section — PROFILE_FIELD_LOCKED', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.updateUser')
      await expect(
        handler({ userId: 'user-001', profile: { security: { allowShellEscape: true } } }, makeCtx('user-001'))
      ).rejects.toThrow('PROFILE_FIELD_LOCKED')
    })

    it('user cannot update another user profile — 403', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.updateUser')
      await expect(
        handler({ userId: 'user-002', profile: { editor: { fontSize: 14 } } }, makeCtx('user-001'))
      ).rejects.toThrow('PROFILE_UNAUTHORIZED')
    })
  })

  // ── profile.invalidate ─────────────────────────────────────────────────────
  describe('profile.invalidate', () => {
    it('admin can invalidate specific user cache', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any, makeAdminAuthManager() as any)
      const handler = findHandler(methods, 'profile.invalidate')
      await handler({ userId: 'user-001' }, makeCtx('admin-001', 'admin'))
      expect(resolver.invalidate).toHaveBeenCalledWith('user-001')
    })

    it('admin can invalidate entire cache (no userId)', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any, makeAdminAuthManager() as any)
      const handler = findHandler(methods, 'profile.invalidate')
      await handler({}, makeCtx('admin-001', 'admin'))
      expect(resolver.invalidate).toHaveBeenCalledWith(undefined)
    })

    it('non-admin receives 403', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.invalidate')
      await expect(handler({ userId: 'user-001' }, makeCtx('user-001', 'developer'))).rejects.toThrow('PROFILE_UNAUTHORIZED')
    })
  })

  // ── profile.setUserDept ────────────────────────────────────────────────────
  describe('profile.setUserDept', () => {
    it('admin can set user department', async () => {
      const svc = makeProfileService()
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any, makeAdminAuthManager() as any)
      const handler = findHandler(methods, 'profile.setUserDept')
      await handler({ userId: 'user-001', deptId: 'dept-engineering' }, makeCtx('admin-001', 'admin'))
      expect(svc.setUserDepartment).toHaveBeenCalledWith('user-001', 'dept-engineering')
    })

    it('non-admin receives 403', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any, makeAuthManager() as any)
      const handler = findHandler(methods, 'profile.setUserDept')
      await expect(
        handler({ userId: 'user-001', deptId: 'dept-X' }, makeCtx('user-001', 'developer'))
      ).rejects.toThrow('PROFILE_UNAUTHORIZED')
    })

    it('unknown deptId returns DEPT_NOT_FOUND', async () => {
      const svc = makeProfileService({
        setUserDepartment: vi.fn().mockRejectedValue(new Error('DEPT_NOT_FOUND')),
      })
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any, makeAdminAuthManager() as any)
      const handler = findHandler(methods, 'profile.setUserDept')
      await expect(
        handler({ userId: 'user-001', deptId: 'nonexistent' }, makeCtx('admin-001', 'admin'))
      ).rejects.toThrow('DEPT_NOT_FOUND')
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/main/profile/__tests__/profile-rpc.test.ts` ✅
- [x] `pnpm vitest run src/main/profile/__tests__/profile-rpc.test.ts` → ≥12 tests passing ✅ (19 tests pass)
- [x] 0 TypeScript errors ✅
