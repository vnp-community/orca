/**
 * Migration 0010 — Task Graph Tables
 *
 * Adds task management tables for TDD-18 (Task Graph Management):
 * - orca_tasks: Task entities with tree structure (parent_id)
 * - orca_task_edges: Dependency edges between tasks (DAG)
 * - orca_task_grants: Permission grants with BFS ancestor resolution
 * - orca_task_comments: Comments and activity feed
 * - orca_team_members: Team membership for grant scope resolution
 *
 * @module db/migrations/0010_tasks
 */

import type { Migration } from './types'

export const migration0010Tasks: Migration = {
  version: 10,
  name: 'tasks',

  async up(db) {
    // ── orca_tasks ────────────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_tasks (
        id               TEXT    PRIMARY KEY,
        project_id       TEXT    REFERENCES orca_v5_projects(id) ON DELETE SET NULL,
        parent_id        TEXT    REFERENCES orca_tasks(id) ON DELETE CASCADE,
        title            TEXT    NOT NULL,
        description      TEXT,
        type             TEXT    NOT NULL DEFAULT 'task',
        status           TEXT    NOT NULL DEFAULT 'backlog',
        priority         TEXT    NOT NULL DEFAULT 'medium',
        labels           TEXT    NOT NULL DEFAULT '[]',
        visibility       TEXT    NOT NULL DEFAULT 'team',
        reporter_id      TEXT,
        assignee_id      TEXT,
        estimated_hours  REAL,
        progress_percent INTEGER NOT NULL DEFAULT 0,
        ai_context       TEXT,
        prompt_template  TEXT,
        due_date         INTEGER,
        created_at       INTEGER NOT NULL,
        updated_at       INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_tasks_project
        ON orca_tasks(project_id, status)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_tasks_parent
        ON orca_tasks(parent_id)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_tasks_assignee
        ON orca_tasks(assignee_id, status)
    `)

    // ── orca_task_edges ───────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_edges (
        from_task_id TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        to_task_id   TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        edge_type    TEXT    NOT NULL DEFAULT 'depends_on',
        created_at   INTEGER NOT NULL DEFAULT (strftime('%s', 'now') * 1000),
        PRIMARY KEY (from_task_id, to_task_id, edge_type)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_edges_from
        ON orca_task_edges(from_task_id)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_edges_to
        ON orca_task_edges(to_task_id)
    `)

    // ── orca_task_grants ──────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_grants (
        id          TEXT    PRIMARY KEY,
        task_id     TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        scope       TEXT    NOT NULL,
        scope_id    TEXT,
        permission  TEXT    NOT NULL,
        apply_tree  INTEGER NOT NULL DEFAULT 0,
        granted_by  TEXT    NOT NULL,
        expires_at  INTEGER,
        created_at  INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_grants_task
        ON orca_task_grants(task_id)
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_grants_scope
        ON orca_task_grants(scope, scope_id)
    `)

    // ── orca_task_comments ────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_task_comments (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        task_id     TEXT    NOT NULL REFERENCES orca_tasks(id) ON DELETE CASCADE,
        user_id     TEXT    NOT NULL,
        content     TEXT    NOT NULL,
        type        TEXT    NOT NULL DEFAULT 'comment',
        created_at  INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_task_comments_task
        ON orca_task_comments(task_id, created_at DESC)
    `)

    // ── orca_team_members ─────────────────────────────────────────────────────
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_team_members (
        team_id  TEXT    NOT NULL,
        user_id  TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        role     TEXT    NOT NULL DEFAULT 'member',
        added_at INTEGER NOT NULL,
        PRIMARY KEY (team_id, user_id)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_team_members_user
        ON orca_team_members(user_id)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_team_members_user')
    await db.exec('DROP TABLE IF EXISTS orca_team_members')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_comments_task')
    await db.exec('DROP TABLE IF EXISTS orca_task_comments')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_grants_scope')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_grants_task')
    await db.exec('DROP TABLE IF EXISTS orca_task_grants')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_edges_to')
    await db.exec('DROP INDEX IF EXISTS idx_orca_task_edges_from')
    await db.exec('DROP TABLE IF EXISTS orca_task_edges')
    await db.exec('DROP INDEX IF EXISTS idx_orca_tasks_assignee')
    await db.exec('DROP INDEX IF EXISTS idx_orca_tasks_parent')
    await db.exec('DROP INDEX IF EXISTS idx_orca_tasks_project')
    await db.exec('DROP TABLE IF EXISTS orca_tasks')
  }
}
