/**
 * Tests for TeamService + team-rpc-handler
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, following ProfileService.test.ts pattern.
 *
 * @module main/team/__tests__/TeamService.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TeamService } from '../TeamService'
import { createTeamMethods } from '../team-rpc-handler'
import type { RpcMethod, RpcContext } from '../../runtime/rpc/core'

// ── helpers ────────────────────────────────────────────────────────────────

async function makeService(): Promise<{
  pool: SqliteSingleConnectionPool
  service: TeamService
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  return { pool, service: new TeamService(pool) }
}

/** Insert a minimal user row to satisfy orca_users FK where relevant. */
async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

function findMethod(methods: RpcMethod[], name: string): RpcMethod {
  const method = methods.find((m) => m.name === name)
  if (!method) {throw new Error(`RPC method not found: ${name}`)}
  return method
}

/** Minimal fake RpcContext — handlers under test only touch ctx.userId. */
function fakeCtx(userId?: string): RpcContext {
  return { userId } as RpcContext
}

// ── tests ──────────────────────────────────────────────────────────────────

describe('TeamService', () => {
  let pool: SqliteSingleConnectionPool
  let service: TeamService

  beforeEach(async () => {
    const setup = await makeService()
    pool = setup.pool
    service = setup.service
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  // ── createTeam / listTeams / getTeam ────────────────────────────────────────

  it('createTeam creates a team and returns UUID id', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    expect(team.id).toMatch(/^[0-9a-f-]{36}$/)
    expect(team.name).toBe('Platform')
    expect(team.createdAt).toBeInstanceOf(Date)
    expect(team.updatedAt).toBeInstanceOf(Date)
  })

  it('createTeam: two teams get different IDs', async () => {
    const t1 = await service.createTeam({ name: 'Alpha' })
    const t2 = await service.createTeam({ name: 'Beta' })
    expect(t1.id).not.toBe(t2.id)
  })

  it('getTeam returns the created team', async () => {
    const created = await service.createTeam({ name: 'Platform' })
    const fetched = await service.getTeam(created.id)
    expect(fetched?.id).toBe(created.id)
    expect(fetched?.name).toBe('Platform')
  })

  it('getTeam returns null for unknown team', async () => {
    const fetched = await service.getTeam('non-existent')
    expect(fetched).toBeNull()
  })

  it('listTeams lists all teams ordered by name', async () => {
    await service.createTeam({ name: 'Zeta' })
    await service.createTeam({ name: 'Alpha' })
    const teams = await service.listTeams()
    expect(teams.map((t) => t.name)).toEqual(['Alpha', 'Zeta'])
  })

  // ── addMember / removeMember / listMembers ──────────────────────────────────

  it('addMember adds a member to a team', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    await insertUser(pool, 'u-1')
    await service.addMember({ teamId: team.id, userId: 'u-1', role: 'engineer', priority: 5 })
    const members = await service.listMembers(team.id)
    expect(members).toHaveLength(1)
    expect(members[0]).toMatchObject({
      teamId: team.id,
      userId: 'u-1',
      role: 'engineer',
      priority: 5
    })
    expect(members[0].addedAt).toBeInstanceOf(Date)
  })

  it('addMember upserts — second call replaces role/priority for the same (team,user)', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    await insertUser(pool, 'u-1')
    await service.addMember({ teamId: team.id, userId: 'u-1', role: 'engineer', priority: 1 })
    await service.addMember({ teamId: team.id, userId: 'u-1', role: 'lead', priority: 9 })
    const members = await service.listMembers(team.id)
    expect(members).toHaveLength(1)
    expect(members[0].role).toBe('lead')
    expect(members[0].priority).toBe(9)
  })

  it('removeMember removes a member', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    await insertUser(pool, 'u-1')
    await service.addMember({ teamId: team.id, userId: 'u-1', role: 'engineer', priority: 0 })
    await service.removeMember({ teamId: team.id, userId: 'u-1' })
    const members = await service.listMembers(team.id)
    expect(members).toHaveLength(0)
  })

  it('listMembers returns empty array for team with no members', async () => {
    const team = await service.createTeam({ name: 'Empty' })
    const members = await service.listMembers(team.id)
    expect(members).toEqual([])
  })

  // ── listTeamsForUser ─────────────────────────────────────────────────────────

  it('listTeamsForUser lists all team memberships across teams for a user', async () => {
    const teamA = await service.createTeam({ name: 'Alpha' })
    const teamB = await service.createTeam({ name: 'Beta' })
    await insertUser(pool, 'u-1')
    await service.addMember({ teamId: teamA.id, userId: 'u-1', role: 'engineer', priority: 1 })
    await service.addMember({ teamId: teamB.id, userId: 'u-1', role: 'lead', priority: 9 })

    const memberships = await service.listTeamsForUser('u-1')
    expect(memberships).toHaveLength(2)
    expect(memberships.map((m) => m.teamId).sort()).toEqual([teamA.id, teamB.id].sort())
  })

  it('listTeamsForUser returns empty array for user with no team memberships', async () => {
    await insertUser(pool, 'u-lonely')
    const memberships = await service.listTeamsForUser('u-lonely')
    expect(memberships).toEqual([])
  })
})

describe('team-rpc-handler', () => {
  let pool: SqliteSingleConnectionPool
  let service: TeamService

  beforeEach(async () => {
    const setup = await makeService()
    pool = setup.pool
    service = setup.service
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  const adminRole = async (userId: string) => (userId === 'admin-1' ? 'admin' : 'developer')

  it('team.create succeeds for an admin user', async () => {
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.create')
    const result = (await method.handler({ name: 'Platform' }, fakeCtx('admin-1'))) as {
      id: string
      name: string
    }
    expect(result.name).toBe('Platform')
    expect(await service.getTeam(result.id)).not.toBeNull()
  })

  it('team.create is rejected for a non-admin user', async () => {
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.create')
    await expect(
      method.handler({ name: 'Platform' }, fakeCtx('dev-1'))
    ).rejects.toThrow(/FORBIDDEN/)
  })

  it('team.create is rejected for an unauthenticated caller', async () => {
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.create')
    await expect(method.handler({ name: 'Platform' }, fakeCtx())).rejects.toThrow(/UNAUTHENTICATED/)
  })

  it('team.addMember succeeds for an admin user', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    await insertUser(pool, 'u-1')
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.addMember')
    const result = (await method.handler(
      { teamId: team.id, userId: 'u-1', role: 'engineer', priority: 3 },
      fakeCtx('admin-1')
    )) as { success: boolean }
    expect(result.success).toBe(true)
    const members = await service.listMembers(team.id)
    expect(members[0]).toMatchObject({ userId: 'u-1', role: 'engineer', priority: 3 })
  })

  it('team.addMember is rejected for a non-admin user', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    await insertUser(pool, 'u-1')
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.addMember')
    await expect(
      method.handler(
        { teamId: team.id, userId: 'u-1', role: 'engineer', priority: 0 },
        fakeCtx('dev-1')
      )
    ).rejects.toThrow(/FORBIDDEN/)
    expect(await service.listMembers(team.id)).toEqual([])
  })

  it('team.removeMember is rejected for a non-admin user', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    await insertUser(pool, 'u-1')
    await service.addMember({ teamId: team.id, userId: 'u-1', role: 'engineer', priority: 0 })
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.removeMember')
    await expect(
      method.handler({ teamId: team.id, userId: 'u-1' }, fakeCtx('dev-1'))
    ).rejects.toThrow(/FORBIDDEN/)
    expect(await service.listMembers(team.id)).toHaveLength(1)
  })

  it('team.list works for any authenticated user (non-admin included)', async () => {
    await service.createTeam({ name: 'Platform' })
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.list')
    const result = (await method.handler(null, fakeCtx('dev-1'))) as { name: string }[]
    expect(result).toHaveLength(1)
    expect(result[0].name).toBe('Platform')
  })

  it('team.list rejects unauthenticated caller', async () => {
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.list')
    await expect(method.handler(null, fakeCtx())).rejects.toThrow(/UNAUTHENTICATED/)
  })

  it('team.listMembers works for any authenticated user (non-admin included)', async () => {
    const team = await service.createTeam({ name: 'Platform' })
    await insertUser(pool, 'u-1')
    await service.addMember({ teamId: team.id, userId: 'u-1', role: 'engineer', priority: 0 })
    const methods = createTeamMethods(service, adminRole)
    const method = findMethod(methods, 'team.listMembers')
    const result = (await method.handler({ teamId: team.id }, fakeCtx('dev-1'))) as {
      userId: string
    }[]
    expect(result).toHaveLength(1)
    expect(result[0].userId).toBe('u-1')
  })
})
