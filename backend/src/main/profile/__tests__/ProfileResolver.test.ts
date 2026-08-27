/**
 * Tests for ProfileResolver — cascade merge (Company → Department → Team(s) → User).
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, following the TeamService.test.ts pattern.
 *
 * @module main/profile/__tests__/ProfileResolver.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { ProfileService } from '../ProfileService'
import { ProfileResolver } from '../ProfileResolver'
import type { OrcaProfile } from '../OrcaProfile'

// ── helpers ────────────────────────────────────────────────────────────────

async function makeResolver(): Promise<{
  pool: SqliteSingleConnectionPool
  service: ProfileService
  resolver: ProfileResolver
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const service = new ProfileService(pool)
  return { pool, service, resolver: new ProfileResolver(service) }
}

async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

async function insertCompanyWithProfile(
  pool: SqliteSingleConnectionPool,
  companyId: string,
  profile: OrcaProfile
): Promise<void> {
  const now = Date.now()
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_companies (id, name, profile_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?)',
      [companyId, companyId, JSON.stringify(profile), now, now]
    )
  )
}

async function insertDeptWithProfile(
  pool: SqliteSingleConnectionPool,
  deptId: string,
  companyId: string,
  profile: OrcaProfile
): Promise<void> {
  const now = Date.now()
  await pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_departments (id, company_id, name, profile_json, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?)`,
      [deptId, companyId, deptId, JSON.stringify(profile), now, now]
    )
  )
}

async function insertTeamWithProfile(
  pool: SqliteSingleConnectionPool,
  teamId: string,
  profile: OrcaProfile
): Promise<void> {
  const now = Date.now()
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_teams (id, name, profile_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?)',
      [teamId, teamId, JSON.stringify(profile), now, now]
    )
  )
}

async function addTeamMember(
  pool: SqliteSingleConnectionPool,
  teamId: string,
  userId: string,
  priority: number
): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_team_members (team_id, user_id, role, priority, added_at)
       VALUES (?, ?, 'member', ?, ?)`,
      [teamId, userId, priority, Date.now()]
    )
  )
}

// ── tests ──────────────────────────────────────────────────────────────────

describe('ProfileResolver', () => {
  let pool: SqliteSingleConnectionPool
  let service: ProfileService
  let resolver: ProfileResolver

  beforeEach(async () => {
    const setup = await makeResolver()
    pool = setup.pool
    service = setup.service
    resolver = setup.resolver
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  it('merges Company → Department → User when the user belongs to no team', async () => {
    await insertUser(pool, 'user-1')
    await insertCompanyWithProfile(pool, 'company-1', {
      agent: { preferredModel: 'company-model' },
    })
    await insertDeptWithProfile(pool, 'dept-1', 'company-1', {
      agent: { trustPreset: 'standard' },
    })
    await service.setUserDepartment('user-1', 'dept-1')
    await service.setUserProfile('user-1', { editor: { theme: 'dark' } })

    const resolved = await resolver.resolve('user-1')

    expect(resolved.agent).toEqual({ preferredModel: 'company-model', trustPreset: 'standard' })
    expect(resolved.editor).toEqual({ theme: 'dark' })
    expect(resolved._sources['agent.preferredModel']).toBe('company')
    expect(resolved._sources['agent.trustPreset']).toBe('dept')
    expect(resolved._sources['editor.theme']).toBe('user')
    // No 'team:*' key should ever appear when the user has no memberships.
    expect(Object.keys(resolved._sources).some((k) => k.startsWith('team:'))).toBe(false)
  })

  it('inserts Team between Department and User, and the higher-priority Team wins on conflict', async () => {
    await insertUser(pool, 'user-2')
    await insertCompanyWithProfile(pool, 'company-2', {
      agent: { preferredModel: 'company-model' },
    })
    await insertDeptWithProfile(pool, 'dept-2', 'company-2', {
      agent: { preferredModel: 'dept-model' },
    })
    await service.setUserDepartment('user-2', 'dept-2')

    await insertTeamWithProfile(pool, 'team-low', { agent: { preferredModel: 'low-priority-model' } })
    await insertTeamWithProfile(pool, 'team-high', { agent: { preferredModel: 'high-priority-model' } })
    await addTeamMember(pool, 'team-low', 'user-2', 1)
    await addTeamMember(pool, 'team-high', 'user-2', 10)

    const resolved = await resolver.resolve('user-2')

    // Team (either priority) beats Department; the higher-priority Team wins the conflict.
    expect(resolved.agent?.preferredModel).toBe('high-priority-model')
    expect(resolved._sources['agent.preferredModel']).toBe('team:team-high')
  })

  it('lets User override Team, and Team override Department/Company for non-conflicting fields', async () => {
    await insertUser(pool, 'user-3')
    await insertCompanyWithProfile(pool, 'company-3', {
      agent: { preferredModel: 'company-model', maxConcurrentAgents: 1 },
    })
    await insertDeptWithProfile(pool, 'dept-3', 'company-3', {
      agent: { trustPreset: 'standard' },
    })
    await service.setUserDepartment('user-3', 'dept-3')
    await insertTeamWithProfile(pool, 'team-3', {
      agent: { preferredModel: 'team-model', customInstructions: 'from team' },
    })
    await addTeamMember(pool, 'team-3', 'user-3', 5)
    await service.setUserProfile('user-3', { agent: { preferredModel: 'user-model' } })

    const resolved = await resolver.resolve('user-3')

    expect(resolved.agent).toEqual({
      preferredModel: 'user-model', // user beats team
      maxConcurrentAgents: 1, // from company, untouched by dept/team/user
      trustPreset: 'standard', // from dept
      customInstructions: 'from team', // from team, untouched by user
    })
    expect(resolved._sources['agent.preferredModel']).toBe('user')
    expect(resolved._sources['agent.maxConcurrentAgents']).toBe('company')
    expect(resolved._sources['agent.trustPreset']).toBe('dept')
    expect(resolved._sources['agent.customInstructions']).toBe('team:team-3')
  })

  it('never lets a Team profile override the company-locked security section', async () => {
    await insertUser(pool, 'user-4')
    await insertCompanyWithProfile(pool, 'company-4', {
      security: { approvedModels: ['company-approved'] },
    })
    await insertDeptWithProfile(pool, 'dept-4', 'company-4', {})
    await service.setUserDepartment('user-4', 'dept-4')
    // A team profile that (incorrectly) sets `security` must be ignored entirely.
    await insertTeamWithProfile(pool, 'team-4', {
      security: { approvedModels: ['team-should-not-win'] },
    })
    await addTeamMember(pool, 'team-4', 'user-4', 99)

    const resolved = await resolver.resolve('user-4')

    expect(resolved.security).toEqual({ approvedModels: ['company-approved'] })
    expect(resolved._sources['security']).toBe('company')
  })

  it('handles an empty orca_teams/orca_team_members table without throwing', async () => {
    await insertUser(pool, 'user-5')
    await insertCompanyWithProfile(pool, 'company-5', { agent: { preferredModel: 'company-model' } })
    await insertDeptWithProfile(pool, 'dept-5', 'company-5', {})
    await service.setUserDepartment('user-5', 'dept-5')

    await expect(resolver.resolve('user-5')).resolves.toMatchObject({
      agent: { preferredModel: 'company-model' },
    })
  })
})
