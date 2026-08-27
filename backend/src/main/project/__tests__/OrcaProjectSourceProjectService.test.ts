/**
 * Tests for OrcaProjectSourceProjectService
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, following ProjectService.test.ts /
 * TeamService.test.ts pattern.
 *
 * @module main/project/__tests__/OrcaProjectSourceProjectService.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { OrcaProjectSourceProjectService } from '../OrcaProjectSourceProjectService'

async function makeService(): Promise<{
  pool: SqliteSingleConnectionPool
  service: OrcaProjectSourceProjectService
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  return { pool, service: new OrcaProjectSourceProjectService(pool) }
}

/** Insert a minimal user row — orca_project_source_projects.owner_user_id FKs to orca_users(id). */
async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

/** Insert a minimal OrcaProject row — orca_project_source_projects.orca_project_id FKs to orca_v5_projects(id). */
async function insertOrcaProject(pool: SqliteSingleConnectionPool, orcaProjectId: string): Promise<void> {
  const now = Date.now()
  await pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_v5_projects
         (id, name, dev_server_id, repo_path, default_branch, visibility, created_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [orcaProjectId, orcaProjectId, 'dev-server-001', '/irrelevant', 'main', 'team', 'u-A', now, now]
    )
  )
}

describe('OrcaProjectSourceProjectService', () => {
  let pool: SqliteSingleConnectionPool
  let service: OrcaProjectSourceProjectService

  beforeEach(async () => {
    const setup = await makeService()
    pool = setup.pool
    service = setup.service
    // FK prerequisites for every test below: orca_project_source_projects
    // references orca_users(id) and orca_v5_projects(id).
    await insertUser(pool, 'u-A')
    await insertUser(pool, 'u-B')
    await insertUser(pool, 'u-lonely')
    await insertOrcaProject(pool, 'orca-1')
    await insertOrcaProject(pool, 'orca-2')
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  // ── linkProject ──────────────────────────────────────────────────────────────

  it('linkProject inserts a row visible via listSourceProjects', async () => {
    await service.linkProject(
      { orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )
    const sources = await service.listSourceProjects('orca-1')
    expect(sources).toEqual([{ ownerUserId: 'u-A', projectId: 'proj-P' }])
  })

  it('linkProject upserts idempotently — relinking the same triple does not duplicate', async () => {
    await service.linkProject(
      { orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )
    await service.linkProject(
      { orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )
    const sources = await service.listSourceProjects('orca-1')
    expect(sources).toHaveLength(1)
  })

  // ── SECURITY: linkProject anti-spoofing ─────────────────────────────────────

  it('SECURITY: linkProject rejects when ownerUserId does not match the acting user', async () => {
    await expect(
      service.linkProject(
        { orcaProjectId: 'orca-1', ownerUserId: 'u-B', projectId: 'proj-P' },
        'u-A' // u-A tries to link a project claiming u-B as the owner
      )
    ).rejects.toThrow(/FORBIDDEN/)
    // Nothing was written.
    expect(await service.listSourceProjects('orca-1')).toEqual([])
  })

  // ── unlinkProject ────────────────────────────────────────────────────────────

  it('unlinkProject removes the row', async () => {
    await service.linkProject(
      { orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )
    await service.unlinkProject({ orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-P' })
    expect(await service.listSourceProjects('orca-1')).toEqual([])
  })

  it('unlinkProject is a no-op (does not throw) when the row does not exist', async () => {
    await expect(
      service.unlinkProject({ orcaProjectId: 'orca-nope', ownerUserId: 'u-A', projectId: 'proj-P' })
    ).resolves.toBeUndefined()
  })

  // ── listSourceProjects ───────────────────────────────────────────────────────

  it('listSourceProjects returns empty array for an OrcaProject with nothing linked', async () => {
    expect(await service.listSourceProjects('orca-empty')).toEqual([])
  })

  it('listSourceProjects only returns rows for the requested orcaProjectId', async () => {
    await service.linkProject(
      { orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )
    await service.linkProject(
      { orcaProjectId: 'orca-2', ownerUserId: 'u-A', projectId: 'proj-Q' },
      'u-A'
    )
    expect(await service.listSourceProjects('orca-1')).toEqual([
      { ownerUserId: 'u-A', projectId: 'proj-P' }
    ])
  })

  // ── listOrcaProjectsForOwner ─────────────────────────────────────────────────

  it('listOrcaProjectsForOwner lists distinct OrcaProject ids across multiple links', async () => {
    await service.linkProject(
      { orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )
    await service.linkProject(
      { orcaProjectId: 'orca-1', ownerUserId: 'u-A', projectId: 'proj-Q' },
      'u-A'
    )
    await service.linkProject(
      { orcaProjectId: 'orca-2', ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )
    const orcaProjectIds = await service.listOrcaProjectsForOwner('u-A')
    expect(orcaProjectIds.sort()).toEqual(['orca-1', 'orca-2'])
  })

  it('listOrcaProjectsForOwner returns empty array for an owner with nothing shared', async () => {
    expect(await service.listOrcaProjectsForOwner('u-lonely')).toEqual([])
  })
})
