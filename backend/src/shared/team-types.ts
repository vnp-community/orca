/**
 * Team Types — Company → Department (tree) → Team (cross-cutting) → User (§5.2)
 *
 * Shared types for the Team entity and team membership.
 * Used by TeamService (main), ProfileResolver (cascade-merge tiebreak), and
 * potentially renderer admin UI later.
 *
 * See docs/guides/user-profile-team-department-rbac.md §5.2 — Team has no
 * departmentId/parentId by design (a Team does not belong to one department).
 *
 * @module shared/team-types
 */

/** A team — cross-cutting grouping, independent of the department tree. */
export type Team = {
  id: string
  name: string
  createdAt: Date
  updatedAt: Date
}

/**
 * Team membership record.
 *
 * `priority`: cascade-merge tiebreaker when a user belongs to multiple teams
 * — higher priority wins conflicting profile fields (§5.2 decision, 2026-08-13).
 */
export type TeamMember = {
  teamId: string
  userId: string
  role: string
  priority: number
  addedAt: Date
}
