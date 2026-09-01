// ─── Tenant User Profile / Department Types ────────────────────────────────
// Backs the First-Login Department Gate (CR-DS-008). Deliberately a
// separate file/type from renderer/src/types/profile-types.ts's `Department`
// (that one models an unrelated org-hierarchy admin panel with a different
// shape — id/name/parentId/leadId — and is not backed by any real RPC yet).
// These types mirror tenant-service's actual UserProfile/Department proto
// messages, as returned by the api-gateway wscompat channels
// profile.getUserProfile / profile.listDepts / profile.updateUser.
// Pure type file — no imports from other project modules.

export type TenantUserProfile = {
  userId: string
  companyId: string
  departmentId: string // Empty string = no department set yet.
  settingsJson: string
}

export type TenantDepartment = {
  id: string
  companyId: string
  name: string
  settingsJson: string
}

export type TenantCompany = {
  id: string
  name: string
  settingsJson: string
}
