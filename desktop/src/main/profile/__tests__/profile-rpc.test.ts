/**
 * Tests for profile RPC handlers (TDD-14) — T12
 *
 * Mock-based tests following project-rpc.test.ts pattern.
 *
 * Actual API:
 *   createProfileMethods(profileService: ProfileService, profileResolver: ProfileResolver)
 *   — 2 args, NO authManager (admin check via requireAdmin(ctx) which only checks ctx.userId)
 *
 * Admin enforcement: requireAdmin() only checks ctx.userId presence (auth is middleware-level).
 * For tests, any user with userId passes requireAdmin — cannot test 403 via handler alone.
 *
 * Method names: profile.getResolved, profile.getUserProfile, profile.updateUser,
 *   profile.getCompany, profile.updateCompany, profile.updateDept, profile.invalidate,
 *   profile.setUserDept, profile.createCompany, profile.createDept
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { createProfileMethods } from '../profile-rpc-handler'
import type { RpcContext } from '../../runtime/rpc/core'
import type { OrcaProfile } from '../OrcaProfile'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

// ── Helpers ────────────────────────────────────────────────────────────────────

const FAKE_RESOLVED = {
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
    createCompany: vi.fn().mockResolvedValue('company-001'),
    createDepartment: vi.fn().mockResolvedValue('dept-001'),
    setUserDepartment: vi.fn().mockResolvedValue(undefined),
    listDepartments: vi.fn().mockResolvedValue([]),
    ...overrides,
  }
}

function makeProfileResolver(resolved = FAKE_RESOLVED) {
  return {
    resolve: vi.fn().mockResolvedValue(resolved),
    invalidate: vi.fn(),
  }
}

function findHandler(methods: ReturnType<typeof createProfileMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) {throw new Error(`Method not found: ${name}`)}
  return method.handler
}

// ── Tests ──────────────────────────────────────────────────────────────────────

describe('profile RPC handlers', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── profile.getResolved ────────────────────────────────────────────────────
  describe('profile.getResolved', () => {
    it('returns resolved profile for authenticated user', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any)
      const handler = findHandler(methods, 'profile.getResolved')
      const result = await handler({}, makeCtx('user-001'))
      expect(result._resolvedAt).toBeDefined()
      expect(resolver.resolve).toHaveBeenCalledWith('user-001')
    })

    it('returns empty-source profile when no company configured', async () => {
      const emptyResolver = makeProfileResolver({ _sources: {}, _resolvedAt: Date.now() })
      const methods = createProfileMethods(makeProfileService() as any, emptyResolver as any)
      const handler = findHandler(methods, 'profile.getResolved')
      const result = await handler({}, makeCtx('user-001'))
      expect(Object.keys(result._sources)).toHaveLength(0)
    })

    it('throws UNAUTHENTICATED when no userId in context', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.getResolved')
      await expect(handler({}, {} as RpcContext)).rejects.toThrow('UNAUTHENTICATED')
    })
  })

  // ── profile.getUserProfile ─────────────────────────────────────────────────
  describe('profile.getUserProfile', () => {
    it('returns user profile for the requested userId', async () => {
      const svc = makeProfileService({ getUserProfile: vi.fn().mockResolvedValue({ editor: { fontSize: 16 } }) })
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.getUserProfile')
      const result = await handler({ userId: 'user-001' }, makeCtx('user-001'))
      expect(result).toHaveProperty('editor')
    })

    it('falls back to ctx.userId when userId param not provided', async () => {
      const svc = makeProfileService()
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.getUserProfile')
      await handler({}, makeCtx('user-ctx'))
      expect(svc.getUserProfile).toHaveBeenCalledWith('user-ctx')
    })
  })

  // ── profile.updateUser ─────────────────────────────────────────────────────
  describe('profile.updateUser', () => {
    it('user can update their own profile (non-security fields)', async () => {
      const svc = makeProfileService()
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(svc as any, resolver as any)
      const handler = findHandler(methods, 'profile.updateUser')
      const result = await handler({ userId: 'user-001', profile: { editor: { fontSize: 14 } } }, makeCtx('user-001'))
      expect(svc.setUserProfile).toHaveBeenCalledWith('user-001', { editor: { fontSize: 14 } })
      expect(result).toHaveProperty('success', true)
    })

    it('rejects update of locked security section — PROFILE_FIELD_LOCKED', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateUser')
      await expect(
        handler({ userId: 'user-001', profile: { security: { allowShellEscape: true } } }, makeCtx('user-001'))
      ).rejects.toThrow('PROFILE_FIELD_LOCKED')
    })

    it('invalidates resolver cache after update', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any)
      const handler = findHandler(methods, 'profile.updateUser')
      await handler({ userId: 'user-001', profile: { editor: { fontSize: 12 } } }, makeCtx('user-001'))
      expect(resolver.invalidate).toHaveBeenCalledWith('user-001')
    })
  })

  // ── profile.updateCompany ──────────────────────────────────────────────────
  describe('profile.updateCompany', () => {
    it('authenticated user calls setCompanyProfile', async () => {
      const svc = makeProfileService()
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateCompany')
      // requireAdmin only checks userId presence, not role
      await handler({ companyId: 'co-001', profile: { agent: { model: 'gemini-2.5' } } }, makeCtx('admin-001', 'admin'))
      expect(svc.setCompanyProfile).toHaveBeenCalled()
    })

    it('invalidates all profiles after company update', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any)
      const handler = findHandler(methods, 'profile.updateCompany')
      await handler({ companyId: 'co-001', profile: {} }, makeCtx('admin-001'))
      expect(resolver.invalidate).toHaveBeenCalledWith()
    })

    it('unauthenticated call throws UNAUTHENTICATED', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateCompany')
      // No userId → requireAdmin throws UNAUTHENTICATED
      await expect(handler({ companyId: 'co-001', profile: {} }, {} as RpcContext)).rejects.toThrow('UNAUTHENTICATED')
    })
  })

  // ── profile.invalidate ─────────────────────────────────────────────────────
  describe('profile.invalidate', () => {
    it('calls resolver.invalidate with specific userId', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any)
      const handler = findHandler(methods, 'profile.invalidate')
      await handler({ userId: 'user-001' }, makeCtx('admin-001', 'admin'))
      expect(resolver.invalidate).toHaveBeenCalledWith('user-001')
    })

    it('calls resolver.invalidate with undefined when no userId', async () => {
      const resolver = makeProfileResolver()
      const methods = createProfileMethods(makeProfileService() as any, resolver as any)
      const handler = findHandler(methods, 'profile.invalidate')
      await handler({}, makeCtx('admin-001', 'admin'))
      expect(resolver.invalidate).toHaveBeenCalledWith(undefined)
    })

    it('returns { success: true, cleared } on success', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.invalidate')
      const result = await handler({ userId: 'user-001' }, makeCtx('admin-001'))
      expect(result).toHaveProperty('success', true)
      expect(result).toHaveProperty('cleared')
    })

    it('unauthenticated call throws UNAUTHENTICATED', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.invalidate')
      await expect(handler({ userId: 'u' }, {} as RpcContext)).rejects.toThrow('UNAUTHENTICATED')
    })
  })

  // ── profile.setUserDept ────────────────────────────────────────────────────
  describe('profile.setUserDept', () => {
    it('sets user department and returns { success: true }', async () => {
      const svc = makeProfileService()
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.setUserDept')
      const result = await handler({ userId: 'user-001', deptId: 'dept-engineering' }, makeCtx('admin-001', 'admin'))
      expect(svc.setUserDepartment).toHaveBeenCalledWith('user-001', 'dept-engineering')
      expect(result).toHaveProperty('success', true)
    })

    it('unknown deptId propagates DEPT_NOT_FOUND', async () => {
      const svc = makeProfileService({
        setUserDepartment: vi.fn().mockRejectedValue(new Error('DEPT_NOT_FOUND')),
      })
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.setUserDept')
      await expect(
        handler({ userId: 'user-001', deptId: 'nonexistent' }, makeCtx('admin-001', 'admin'))
      ).rejects.toThrow('DEPT_NOT_FOUND')
    })

    it('unauthenticated call throws UNAUTHENTICATED', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.setUserDept')
      await expect(handler({ userId: 'u', deptId: 'd' }, {} as RpcContext)).rejects.toThrow('UNAUTHENTICATED')
    })
  })

  // ── profile.createCompany ──────────────────────────────────────────────────
  describe('profile.createCompany', () => {
    it('creates company and returns { id }', async () => {
      const svc = makeProfileService()
      const methods = createProfileMethods(svc as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.createCompany')
      const result = await handler({ name: 'Acme Corp' }, makeCtx('admin-001'))
      expect(result).toHaveProperty('id')
    })
  })

  // ── CR-TRACE-015: profile:updateLayer tracing (TASK-BE-015.5) ─────────────
  describe('profile:updateLayer tracing', () => {
    function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
      const events: TraceEvent[] = []
      const unregister = registerTraceSink((e) => events.push(e))
      return { events, stop: unregister }
    }

    it('profile.updateCompany → step(invalidateCache) always runs before ok()', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateCompany')
      const { events, stop } = captureTraceEvents()

      await handler({ companyId: 'co-001', profile: {} }, makeCtx('admin-001'))
      stop()

      const spanEvents = events.filter((e) => e.flow === 'profile:updateLayer')
      const invalidateIdx = spanEvents.findIndex((e) => e.level === 'step' && e.label === 'invalidateCache')
      const okIdx = spanEvents.findIndex((e) => e.level === 'ok')
      expect(invalidateIdx).toBeGreaterThanOrEqual(0)
      expect(okIdx).toBeGreaterThan(invalidateIdx)
    })

    it('profile.updateDept → step(invalidateCache) always runs before ok()', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateDept')
      const { events, stop } = captureTraceEvents()

      await handler({ deptId: 'dept-001', profile: {} }, makeCtx('admin-001'))
      stop()

      const spanEvents = events.filter((e) => e.flow === 'profile:updateLayer')
      const invalidateIdx = spanEvents.findIndex((e) => e.level === 'step' && e.label === 'invalidateCache')
      const okIdx = spanEvents.findIndex((e) => e.level === 'ok')
      expect(invalidateIdx).toBeGreaterThanOrEqual(0)
      expect(okIdx).toBeGreaterThan(invalidateIdx)
    })

    it('profile.updateUser → step(invalidateCache) always runs before ok()', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateUser')
      const { events, stop } = captureTraceEvents()

      await handler({ profile: { editor: { fontSize: 14 } } }, makeCtx('user-001'))
      stop()

      const spanEvents = events.filter((e) => e.flow === 'profile:updateLayer')
      const invalidateIdx = spanEvents.findIndex((e) => e.level === 'step' && e.label === 'invalidateCache')
      const okIdx = spanEvents.findIndex((e) => e.level === 'ok')
      expect(invalidateIdx).toBeGreaterThanOrEqual(0)
      expect(okIdx).toBeGreaterThan(invalidateIdx)
    })

    it('profile.invalidate → step(invalidateCache) always runs before ok()', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.invalidate')
      const { events, stop } = captureTraceEvents()

      await handler({ userId: 'user-001' }, makeCtx('admin-001'))
      stop()

      const spanEvents = events.filter((e) => e.flow === 'profile:updateLayer')
      const invalidateIdx = spanEvents.findIndex((e) => e.level === 'step' && e.label === 'invalidateCache')
      const okIdx = spanEvents.findIndex((e) => e.level === 'ok')
      expect(invalidateIdx).toBeGreaterThanOrEqual(0)
      expect(okIdx).toBeGreaterThan(invalidateIdx)
    })

    it('profile.updateUser with a security section → span.fail(PROFILE_FIELD_LOCKED)', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateUser')
      const { events, stop } = captureTraceEvents()

      await expect(
        handler({ profile: { security: { allowShellEscape: true } } }, makeCtx('user-001'))
      ).rejects.toThrow('PROFILE_FIELD_LOCKED')
      stop()

      const failEvent = events.find((e) => e.flow === 'profile:updateLayer' && e.level === 'fail')
      expect(failEvent?.fields.err).toContain('PROFILE_FIELD_LOCKED')
    })

    it('profile.updateCompany with params.traceId resumes span.id === params.traceId', async () => {
      const methods = createProfileMethods(makeProfileService() as any, makeProfileResolver() as any)
      const handler = findHandler(methods, 'profile.updateCompany')
      const { events, stop } = captureTraceEvents()

      await handler({ companyId: 'co-001', profile: {}, traceId: 'resume-profile-1' }, makeCtx('admin-001'))
      stop()

      const spanEvents = events.filter((e) => e.flow === 'profile:updateLayer')
      expect(spanEvents.every((e) => e.id === 'resume-profile-1')).toBe(true)
    })
  })
})
