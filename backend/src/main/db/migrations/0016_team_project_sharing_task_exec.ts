/**
 * Migration 0016 — Team, OrcaProject Sharing, Task Execution Pipeline
 *
 * Giai đoạn 2 schema additions:
 * - orca_teams: Team metadata (new parent table; does NOT touch existing
 *   orca_team_members from migration 0010). No department_id/parent_id —
 *   a Team does not belong to a single department by design
 *   (docs/guides/user-profile-team-department-rbac.md §5.2).
 * - orca_team_members.priority: cascade-merge tiebreaker (higher wins)
 *   when a user belongs to multiple teams (same doc, §5.2).
 * - orca_project_source_projects: OrcaProject ↔ per-user JSON Project
 *   sharing join table.
 * - orca_tasks.active_execution_task_id / agent_session_id: Source→Plan→
 *   Execute pipeline linkage (docs/guides/task-automation-orchestration-integration.md §9.4.2).
 *
 * @module db/migrations/0016_team_project_sharing_task_exec
 */

import type { Migration } from './types'

export const migration0016TeamProjectSharingTaskExec: Migration = {
  version: 16,
  name: 'team_project_sharing_task_exec',

  async up(db) {
    // ── orca_teams ────────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_teams (
        id         TEXT    PRIMARY KEY,
        name       TEXT    NOT NULL,
        created_at INTEGER NOT NULL,
        updated_at INTEGER NOT NULL
      )
    `)

    // ── orca_team_members.priority (added by migration 0010) ────────────────────
    // Why: cascade-merge tiebreaker — when a user is in multiple teams, the
    // membership with the higher priority wins conflicting profile fields.
    await db.exec(`ALTER TABLE orca_team_members ADD COLUMN priority INTEGER NOT NULL DEFAULT 0`)

    // ── orca_project_source_projects ─────────────────────────────────────────
    // project_id is a LOGIC FK to Project.id inside owner_user_id's per-user
    // JSON file — not a SQL FK, since Project data lives in JSON, not SQL.
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_project_source_projects (
        orca_project_id TEXT    NOT NULL REFERENCES orca_v5_projects(id) ON DELETE CASCADE,
        owner_user_id   TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        project_id      TEXT    NOT NULL,
        created_at      INTEGER NOT NULL,
        PRIMARY KEY (orca_project_id, owner_user_id, project_id)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_project_source_projects_orca_project
        ON orca_project_source_projects(orca_project_id)
    `)

    // ── orca_tasks: Source→Plan→Execute pipeline linkage (migration 0010) ───────
    // active_execution_task_id: TaskRow.id in OrchestrationDb when running via
    // the complex path. agent_session_id: set when running via the simple path.
    await db.exec(`ALTER TABLE orca_tasks ADD COLUMN active_execution_task_id TEXT`)
    await db.exec(`ALTER TABLE orca_tasks ADD COLUMN agent_session_id TEXT`)
  },

  async down(db) {
    // orca_tasks.agent_session_id / active_execution_task_id and
    // orca_team_members.priority were added via ALTER TABLE ADD COLUMN.
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // cột thừa không ảnh hưởng hành vi nếu rollback (theo pattern migration 0013/0014/0015).
    await db.exec('DROP INDEX IF EXISTS idx_orca_project_source_projects_orca_project')
    await db.exec('DROP TABLE IF EXISTS orca_project_source_projects')
    await db.exec('DROP TABLE IF EXISTS orca_teams')
  }
}
