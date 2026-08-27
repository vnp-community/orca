/**
 * Migration 0017 — Team profile storage
 *
 * orca_teams (added by migration 0016) had no profile_json column — Team could
 * not carry any agent/editor/shell/mcp overrides, so ProfileResolver's cascade
 * merge had nowhere to read Team-layer data from
 * (docs/guides/user-profile-team-department-rbac.md §5.2, "Team KHÔNG có
 * security field riêng" — same OrcaProfile shape as Company/Department, minus
 * security which stays company-locked).
 *
 * Mirrors orca_companies.profile_json / orca_departments.profile_json.
 *
 * @module db/migrations/0017_team_profile_json
 */

import type { Migration } from './types'

export const migration0017TeamProfileJson: Migration = {
  version: 17,
  name: 'team_profile_json',

  async up(db) {
    // Idempotent — same pattern as migration 0006's department_id ALTER TABLE.
    try {
      await db.exec(
        `ALTER TABLE orca_teams ADD COLUMN profile_json TEXT NOT NULL DEFAULT '{}'`
      )
    } catch {
      // Column may already exist — safe to ignore
    }
  },

  async down(_db) {
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // cột thừa không ảnh hưởng hành vi nếu rollback (theo pattern migration 0013/0014/0015/0016).
  }
}
