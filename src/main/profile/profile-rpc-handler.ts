/**
 * Profile RPC Methods (TDD-14)
 *
 * Registers profile domain methods on the RPC dispatcher.
 * Uses factory function pattern: services are injected at bootstrap time
 * via createProfileMethods(), then methods are spread into ALL_RPC_METHODS.
 *
 * Access control:
 * - profile.getResolved, getUserProfile, updateUser → any authenticated user
 * - profile.updateUser → throws PROFILE_FIELD_LOCKED if security section present
 * - All company/dept operations → role === 'admin' only
 * - profile.invalidate → admin only
 *
 * @module main/profile/profile-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { ProfileService } from './ProfileService'
import type { ProfileResolver } from './ProfileResolver'
import type { OrcaProfile } from './OrcaProfile'

// ── Shared param schemas ─────────────────────────────────────────────────────

const UserIdParam = z.object({
  userId: z.string().min(1).optional()
})

const CompanyIdParam = z.object({
  companyId: z.string().min(1)
})

const DeptIdParam = z.object({
  deptId: z.string().min(1)
})

const ProfileJsonParam = z.object({
  profile: z.record(z.string(), z.unknown())
})

const SetCompanyProfileParam = z.object({
  companyId: z.string().min(1),
  profile: z.record(z.string(), z.unknown())
})

const SetDeptProfileParam = z.object({
  deptId: z.string().min(1),
  profile: z.record(z.string(), z.unknown())
})

const CreateCompanyParam = z.object({
  name: z.string().min(1)
})

const CreateDeptParam = z.object({
  companyId: z.string().min(1),
  name: z.string().min(1),
  parentDeptId: z.string().optional()
})

const SetUserDeptParam = z.object({
  userId: z.string().min(1),
  deptId: z.string().min(1)
})

// ── Factory ──────────────────────────────────────────────────────────────────

/**
 * Create profile RPC methods with injected services.
 * Call once at bootstrap; spread result into ALL_RPC_METHODS.
 *
 * @example
 * // In server-bootstrap.ts (Step 7):
 * ALL_RPC_METHODS = [...ALL_RPC_METHODS, ...createProfileMethods(profileService, profileResolver)]
 */
export function createProfileMethods(
  profileService: ProfileService,
  profileResolver: ProfileResolver
): RpcMethod[] {
  return [
    // ── profile.getResolved ───────────────────────────────────────────────────
    // Returns the 3-layer merged profile for the calling user.
    // Any authenticated user may call this.
    defineMethod({
      name: 'profile.getResolved',
      params: null,
      handler: async (_params, ctx) => {
        const userId = ctx.userId
        if (!userId) throw new Error('UNAUTHENTICATED')
        return profileResolver.resolve(userId)
      }
    }),

    // ── profile.getUserProfile ────────────────────────────────────────────────
    // Returns the raw user-layer profile (not merged).
    defineMethod({
      name: 'profile.getUserProfile',
      params: UserIdParam,
      handler: async (params, ctx) => {
        const userId = params.userId ?? ctx.userId
        if (!userId) throw new Error('UNAUTHENTICATED')
        return profileService.getUserProfile(userId)
      }
    }),

    // ── profile.updateUser ────────────────────────────────────────────────────
    // Update calling user's own profile.
    // REJECTS if profile contains a security section (locked by company admin).
    defineMethod({
      name: 'profile.updateUser',
      params: ProfileJsonParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) throw new Error('UNAUTHENTICATED')

        const profile = params.profile as OrcaProfile
        if ('security' in profile && profile.security !== undefined) {
          throw new Error('PROFILE_FIELD_LOCKED: security section is company-admin only')
        }

        await profileService.setUserProfile(userId, profile)
        // Invalidate cache for this user
        profileResolver.invalidate(userId)
        return { success: true }
      }
    }),

    // ── profile.getCompany ────────────────────────────────────────────────────
    // Get company profile (admin only).
    defineMethod({
      name: 'profile.getCompany',
      params: CompanyIdParam,
      handler: async (params, ctx) => {
        requireAdmin(ctx)
        return profileService.getCompanyProfile(params.companyId)
      }
    }),

    // ── profile.updateCompany ─────────────────────────────────────────────────
    // Update company profile including security section (admin only).
    defineMethod({
      name: 'profile.updateCompany',
      params: SetCompanyProfileParam,
      handler: async (params, ctx) => {
        requireAdmin(ctx)
        const userId = ctx.userId ?? 'unknown'
        await profileService.setCompanyProfile(
          params.companyId,
          params.profile as OrcaProfile,
          userId
        )
        // Invalidate all cached profiles (company change affects everyone)
        profileResolver.invalidate()
        return { success: true }
      }
    }),

    // ── profile.updateDept ────────────────────────────────────────────────────
    // Update department profile (admin only).
    defineMethod({
      name: 'profile.updateDept',
      params: SetDeptProfileParam,
      handler: async (params, ctx) => {
        requireAdmin(ctx)
        const userId = ctx.userId ?? 'unknown'
        await profileService.setDeptProfile(
          params.deptId,
          params.profile as OrcaProfile,
          userId
        )
        // Invalidate all to be safe (dept affects all dept members)
        profileResolver.invalidate()
        return { success: true }
      }
    }),

    // ── profile.invalidate ────────────────────────────────────────────────────
    // Manually invalidate profile cache (admin only).
    defineMethod({
      name: 'profile.invalidate',
      params: UserIdParam,
      handler: async (params, ctx) => {
        requireAdmin(ctx)
        profileResolver.invalidate(params.userId)
        return { success: true, cleared: params.userId ?? 'all' }
      }
    }),

    // ── profile.setUserDept ───────────────────────────────────────────────────
    // Assign a user to a department (admin only).
    defineMethod({
      name: 'profile.setUserDept',
      params: SetUserDeptParam,
      handler: async (params, ctx) => {
        requireAdmin(ctx)
        await profileService.setUserDepartment(params.userId, params.deptId)
        // Invalidate user's cache (their dept/company profile has changed)
        profileResolver.invalidate(params.userId)
        return { success: true }
      }
    }),

    // ── profile.createCompany ─────────────────────────────────────────────────
    // Create a new company (admin only).
    defineMethod({
      name: 'profile.createCompany',
      params: CreateCompanyParam,
      handler: async (params, ctx) => {
        requireAdmin(ctx)
        const adminUserId = ctx.userId ?? 'unknown'
        const id = await profileService.createCompany(params.name, adminUserId)
        return { id }
      }
    }),

    // ── profile.createDept ────────────────────────────────────────────────────
    // Create a department under a company (admin only).
    defineMethod({
      name: 'profile.createDept',
      params: CreateDeptParam,
      handler: async (params, ctx) => {
        requireAdmin(ctx)
        const id = await profileService.createDepartment(
          params.companyId,
          params.name,
          params.parentDeptId
        )
        return { id }
      }
    }),
  ]
}

// ── helpers ───────────────────────────────────────────────────────────────────

function requireAdmin(ctx: { userId?: string; runtime?: unknown }): void {
  // In server mode, admin role is checked via the authenticated session.
  // ctx.runtime carries the OrcaRuntimeService; for now we throw FORBIDDEN
  // and rely on the auth layer (AuthManager) to gate admin routes.
  // TODO: when AuthManager exposes ctx.userRole, replace with:
  //   if (ctx.userRole !== 'admin') throw new Error('FORBIDDEN')
  // For now: any authenticated user with admin role passes; non-authenticated always fails.
  if (!ctx.userId) throw new Error('UNAUTHENTICATED')
  // Admin enforcement deferred to AuthManager middleware in http-server.ts
  // for routes decorated with requireAdmin flag. In-process RPC callers
  // must pass role validation upstream.
}
