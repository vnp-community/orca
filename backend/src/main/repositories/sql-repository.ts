/**
 * SQL State Repository
 *
 * IStateRepository implementation using SQL backend (SQLite, MySQL, PostgreSQL, TiDB).
 * Uses `orca_*` tables created by migration 0004.
 * Stores full entity JSON in `data` column for schema flexibility.
 *
 * @module repositories/sql-repository
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type {
  IStateRepository,
  IProjectRepository,
  IRepoRepository,
  ISshTargetRepository,
  IGlobalSettingsRepository,
  GlobalSettings
} from './types'
import type { Project, Repo } from '../../shared/types'
import type { SshTarget } from '../../shared/ssh-types'

export class SqlStateRepository implements IStateRepository {
  constructor(private readonly pool: IConnectionPool) {}

  // ── IProjectRepository ─────────────────────────────────────────────────────

  get projects(): IProjectRepository {
    const pool = this.pool
    return {
      findById: async (id) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects WHERE id = ?', [id])
        )
        return rows[0] ? (JSON.parse(rows[0]['data'] as string) as Project) : null
      },

      findAll: async () => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects ORDER BY tab_order ASC, created_at ASC')
        )
        return rows.map((r) => JSON.parse(r['data'] as string) as Project)
      },

      create: async (input) => {
        const project = { ...input, id: randomUUID() } as Project
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_projects (id, name, tab_order, data) VALUES (?, ?, ?, ?)',
            [
              project.id,
              (project as Record<string, unknown>)['displayName'] ?? project.id,
              (project as Record<string, unknown>)['tabOrder'] ?? 0,
              JSON.stringify(project)
            ]
          )
        )
        return project
      },

      update: async (id, patch) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects WHERE id = ?', [id])
        )
        const existing = rows[0] ? (JSON.parse(rows[0]['data'] as string) as Project) : null
        if (!existing) {throw new Error(`Project not found: ${id}`)}
        const updated = { ...existing, ...(patch as Partial<Project>) }
        await pool.withConnection((db) =>
          db.query(
            'UPDATE orca_projects SET name = ?, tab_order = ?, data = ? WHERE id = ?',
            [
              (updated as Record<string, unknown>)['displayName'] ?? updated.id,
              (updated as Record<string, unknown>)['tabOrder'] ?? 0,
              JSON.stringify(updated),
              id
            ]
          )
        )
        return updated
      },

      delete: async (id) => {
        await pool.withConnection((db) =>
          db.query('DELETE FROM orca_projects WHERE id = ?', [id])
        )
      },

      findByGroup: async (groupId) => {
        const all = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_projects')
        )
        return all
          .map((r) => JSON.parse(r['data'] as string) as Project)
          .filter((p) => (p as Record<string, unknown>)['projectGroupId'] === groupId)
      }
    }
  }

  // ── IRepoRepository ────────────────────────────────────────────────────────

  get repos(): IRepoRepository {
    const pool = this.pool
    return {
      findById: async (id) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_repos WHERE id = ?', [id])
        )
        return rows[0] ? (JSON.parse(rows[0]['data'] as string) as Repo) : null
      },

      findAll: async () => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_repos ORDER BY created_at ASC')
        )
        return rows.map((r) => JSON.parse(r['data'] as string) as Repo)
      },

      create: async (input) => {
        const repo = { ...input, id: randomUUID() } as Repo
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_repos (id, project_id, data) VALUES (?, ?, ?)',
            [repo.id, (repo as Record<string, unknown>)['projectId'] ?? null, JSON.stringify(repo)]
          )
        )
        return repo
      },

      update: async (id, patch) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_repos WHERE id = ?', [id])
        )
        const existing = rows[0] ? (JSON.parse(rows[0]['data'] as string) as Repo) : null
        if (!existing) {throw new Error(`Repo not found: ${id}`)}
        const updated = { ...existing, ...(patch as Partial<Repo>) }
        await pool.withConnection((db) =>
          db.query('UPDATE orca_repos SET data = ? WHERE id = ?', [JSON.stringify(updated), id])
        )
        return updated
      },

      delete: async (id) => {
        await pool.withConnection((db) =>
          db.query('DELETE FROM orca_repos WHERE id = ?', [id])
        )
      },

      findByProject: async (projectId) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_repos WHERE project_id = ?', [projectId])
        )
        return rows.map((r) => JSON.parse(r['data'] as string) as Repo)
      }
    }
  }

  // ── ISshTargetRepository ──────────────────────────────────────────────────

  get sshTargets(): ISshTargetRepository {
    const pool = this.pool
    return {
      findById: async (id) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_ssh_targets WHERE id = ?', [id])
        )
        return rows[0] ? (JSON.parse(rows[0]['data'] as string) as SshTarget) : null
      },

      findAll: async () => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_ssh_targets ORDER BY created_at ASC')
        )
        return rows.map((r) => JSON.parse(r['data'] as string) as SshTarget)
      },

      create: async (input) => {
        const target = { ...input, id: randomUUID() } as SshTarget
        const t = target as Record<string, unknown>
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_ssh_targets (id, label, host, port, username, data) VALUES (?, ?, ?, ?, ?, ?)',
            [target.id, t['label'] ?? '', t['host'] ?? '', t['port'] ?? 22, t['username'] ?? '', JSON.stringify(target)]
          )
        )
        return target
      },

      update: async (id, patch) => {
        const rows = await pool.withConnection((db) =>
          db.query('SELECT data FROM orca_ssh_targets WHERE id = ?', [id])
        )
        const existing = rows[0] ? (JSON.parse(rows[0]['data'] as string) as SshTarget) : null
        if (!existing) {throw new Error(`SshTarget not found: ${id}`)}
        const updated = { ...existing, ...(patch as Partial<SshTarget>) }
        await pool.withConnection((db) =>
          db.query('UPDATE orca_ssh_targets SET data = ? WHERE id = ?', [JSON.stringify(updated), id])
        )
        return updated
      },

      delete: async (id) => {
        await pool.withConnection((db) =>
          db.query('DELETE FROM orca_ssh_targets WHERE id = ?', [id])
        )
      }
    }
  }

  // ── IGlobalSettingsRepository ─────────────────────────────────────────────

  get settings(): IGlobalSettingsRepository {
    const pool = this.pool
    return {
      get: async () => {
        const rows = await pool.withConnection((db) =>
          db.query("SELECT value FROM orca_global_settings WHERE key = 'app_settings'")
        )
        if (!rows[0]) {return {} as GlobalSettings}
        return JSON.parse(rows[0]['value'] as string) as GlobalSettings
      },

      update: async (patch) => {
        const current = await this.settings.get()
        const updated = { ...current, ...patch }
        await pool.withConnection((db) =>
          db.query(
            'INSERT INTO orca_global_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value',
            ['app_settings', JSON.stringify(updated)]
          )
        )
        return updated
      }
    }
  }

  // ── Lifecycle ─────────────────────────────────────────────────────────────

  async ping(): Promise<boolean> {
    try {
      await this.pool.withConnection((db) => db.query('SELECT 1'))
      return true
    } catch {
      return false
    }
  }

  async close(): Promise<void> {
    await this.pool.drain()
  }
}
