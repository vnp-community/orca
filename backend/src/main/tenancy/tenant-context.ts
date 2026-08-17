/**
 * TenantContext — request-scoped tenant/user identity (ADR-021 Phase 1 primitive)
 *
 * Why application-layer, not Postgres Row-Level Security: ADR-021 §3 (Multi-
 * tenancy) — RLS has no equivalent in MySQL/TiDB, and this codebase's stated
 * future is TiDB (ADR-021 §1). Making tenant isolation depend on a Postgres-only
 * feature would silently stop protecting tenant boundaries the day the dialect
 * switches. AsyncLocalStorage-based context, checked explicitly by every
 * repository, works identically on every dialect this codebase already
 * supports (db/types.ts's `IDatabaseCapabilities.dialect`).
 *
 * This is a primitive, not a policy: it does not by itself scope any query.
 * Every service's repository layer (ADR-021 Phase 1 follow-up work — see
 * specs/backend/models/08-postgres-microservices-target-architecture.md §5)
 * must call `requireTenantId()`/`getTenantContext()` and add `WHERE tenant_id
 * = $1` itself. `withTenantContext()` only ensures that value is available
 * and consistent for the lifetime of one request/RPC call — same shape as
 * how `RpcContext` already threads `userId` through `runtime/rpc/core.ts`
 * (this module intentionally mirrors that pattern rather than introducing a
 * second, competing request-context mechanism).
 *
 * @module main/tenancy/tenant-context
 */

import { AsyncLocalStorage } from 'node:async_hooks'

export type TenantContext = {
  /** `tenant.companies.id` (see ADR-021 §2) — the organization this request acts on behalf of. */
  tenantId: string
  /** `orca_users.id` of the authenticated actor, when known. Some transports
   *  don't populate a clean users-table row (see the `RpcContext.userId` doc
   *  comment in runtime/rpc/core.ts, and orca_task_grants.granted_by's
   *  same-reasoning TEXT-not-FK choice) — kept optional for that reason. */
  userId?: string
  /** Role names for RBAC checks against `auth.access_policies` (0005/0019). */
  roles?: readonly string[]
}

const storage = new AsyncLocalStorage<TenantContext>()

/**
 * Run `fn` with `context` bound as the active tenant context for its entire
 * async call chain (including awaited callbacks) — call once per
 * request/RPC-call entry point, not per repository call.
 */
export function withTenantContext<T>(context: TenantContext, fn: () => T): T {
  return storage.run(context, fn)
}

/** Returns the active tenant context, or `undefined` outside any `withTenantContext()` call. */
export function getTenantContext(): TenantContext | undefined {
  return storage.getStore()
}

/**
 * Returns the active `tenantId`.
 * @throws Error if called outside `withTenantContext()` — a repository method
 * reaching this with no context is a bug (a request path that forgot to
 * establish tenant scope), not a valid "no tenant" state to silently allow
 * through as an unscoped query.
 */
export function requireTenantId(): string {
  const ctx = storage.getStore()
  if (!ctx) {
    throw new Error(
      '[TenantContext] requireTenantId() called outside withTenantContext() — ' +
        'every request/RPC entry point must establish tenant context before any ' +
        'repository call. See ADR-021 §3 and specs/backend/models/08-postgres-microservices-target-architecture.md §5.'
    )
  }
  return ctx.tenantId
}
