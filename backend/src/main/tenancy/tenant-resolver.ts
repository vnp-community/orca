/**
 * TenantResolver — resolves a `userId` to its `tenantId` (ADR-021 §3)
 *
 * Thin wrapper around `ProfileService.getCompanyIdForUser()` — exists as its
 * own module (rather than calling ProfileService directly from
 * server-bootstrap.ts) so the "userId → tenantId" policy has one place to
 * change (e.g. if tenant resolution later needs to consider something beyond
 * department→company, like a user belonging to multiple companies).
 *
 * @module main/tenancy/tenant-resolver
 */

import type { ProfileService } from '../profile/ProfileService'

/**
 * Resolve `userId`'s tenant (= `orca_companies.id`, via `orca_users.department_id`
 * → `orca_departments.company_id`). Returns `undefined` (not `null`) on any
 * failure — including "user has no department yet" — so callers can spread it
 * straight into an options object (`{ ...(tenantId ? { tenantId } : {}) }`)
 * without an extra null-check, matching how `userId` is already handled at
 * every call site (see runtime/rpc/core.ts's `RpcContext.userId` doc comment).
 *
 * Never throws — a tenant-resolution failure must not block server boot or
 * RPC server startup (the same non-fatal posture server-bootstrap.ts already
 * takes for migrations/health-monitor wiring). Logs a warning instead.
 */
export async function resolveTenantId(
  profileService: ProfileService,
  userId: string | undefined
): Promise<string | undefined> {
  if (!userId) {return undefined}
  try {
    const companyId = await profileService.getCompanyIdForUser(userId)
    return companyId ?? undefined
  } catch (err) {
    console.warn(
      `[TenantResolver] Failed to resolve tenantId for userId=${userId} (non-fatal, ` +
        `tenant-scoped RPC methods will see ctx.tenantId=undefined):`,
      (err as Error)?.message
    )
    return undefined
  }
}
