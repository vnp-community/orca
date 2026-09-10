// ─── Admin User Management Types ───────────────────────────────────────────
// Backs the Admin Console's Users tab — mirrors auth-service's User proto
// message, as returned by the api-gateway wscompat channels
// admin.listUsers / admin.updateUserRole / admin.deactivateUser /
// admin.reactivateUser. Pure type file — no imports from other project
// modules.

export type AdminUserRole = 'admin' | 'user'

export type AdminUser = {
  id: string
  tenantId: string
  email: string
  name: string
  role: AdminUserRole
  isActive: boolean
  createdAtUnixMs: number
  /** Empty string = no department assigned. Sourced from tenant-service's
   *  per-user profile, joined in server-side (see admin.listUsers). */
  departmentId: string
}
