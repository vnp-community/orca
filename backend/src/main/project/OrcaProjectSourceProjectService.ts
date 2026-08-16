/**
 * OrcaProjectSourceProjectService — CRUD for the OrcaProject ↔ per-user JSON
 * Project sharing join table (migration 0016: orca_project_source_projects).
 *
 * `project_id` is a LOGIC FK into the owner's per-user orca-data.json — not a
 * SQL FK, since Project data lives in JSON, not SQL (see migration 0016 comment).
 *
 * Pattern follows ProjectService.ts / TeamService.ts: pool.withConnection((db) => db.query(...))
 *
 * SECURITY: this service does authorization for exactly one thing — linkProject()
 * refuses to let caller A link a project on behalf of owner B (identity check).
 * All OTHER access control (is caller a member of orcaProjectId? is projectId
 * really linked to orcaProjectId?) lives in orca-project-sharing-rpc-handler.ts,
 * which reuses ProjectService.assertAccess() rather than duplicating checks here.
 *
 * @module main/project/OrcaProjectSourceProjectService
 */

import type { IConnectionPool } from '../db/pool'

export type LinkProjectParams = {
  orcaProjectId: string
  ownerUserId: string
  projectId: string
}

export type UnlinkProjectParams = {
  orcaProjectId: string
  ownerUserId: string
  projectId: string
}

export type SourceProjectRef = {
  ownerUserId: string
  projectId: string
}

type SourceProjectRow = {
  ownerUserId: string
  projectId: string
}

export class OrcaProjectSourceProjectService {
  constructor(private readonly pool: IConnectionPool) {}

  /**
   * Link a per-user JSON Project into an OrcaProject.
   *
   * @param actingUserId The authenticated caller (ctx.userId). MUST equal
   *   params.ownerUserId — a user can only link THEIR OWN projects, never
   *   link a project on behalf of someone else (anti-spoofing).
   * @throws 'FORBIDDEN: ownerUserId must match the acting user' on mismatch
   */
  async linkProject(params: LinkProjectParams, actingUserId: string): Promise<void> {
    if (params.ownerUserId !== actingUserId) {
      throw new Error(
        `FORBIDDEN: ownerUserId '${params.ownerUserId}' must match the acting user '${actingUserId}'`
      )
    }
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_project_source_projects
           (orca_project_id, owner_user_id, project_id, created_at)
         VALUES (?, ?, ?, ?)
         ON CONFLICT(orca_project_id, owner_user_id, project_id) DO UPDATE SET
           created_at = excluded.created_at`,
        [params.orcaProjectId, params.ownerUserId, params.projectId, now]
      )
    )
  }

  /** Unlink a per-user JSON Project from an OrcaProject. Idempotent no-op if absent. */
  async unlinkProject(params: UnlinkProjectParams): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `DELETE FROM orca_project_source_projects
         WHERE orca_project_id = ? AND owner_user_id = ? AND project_id = ?`,
        [params.orcaProjectId, params.ownerUserId, params.projectId]
      )
    )
  }

  /** List every (ownerUserId, projectId) pair linked into an OrcaProject. */
  async listSourceProjects(orcaProjectId: string): Promise<SourceProjectRef[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<SourceProjectRow>(
        `SELECT owner_user_id as "ownerUserId", project_id as "projectId"
         FROM orca_project_source_projects
         WHERE orca_project_id = ?`,
        [orcaProjectId]
      )
    )
    return rows.map((r) => ({ ownerUserId: r.ownerUserId, projectId: r.projectId }))
  }

  /** List the distinct OrcaProject ids that contain at least one Project owned by ownerUserId. */
  async listOrcaProjectsForOwner(ownerUserId: string): Promise<string[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ orcaProjectId: string }>(
        `SELECT DISTINCT orca_project_id as "orcaProjectId"
         FROM orca_project_source_projects
         WHERE owner_user_id = ?`,
        [ownerUserId]
      )
    )
    return rows.map((r) => r.orcaProjectId)
  }
}
