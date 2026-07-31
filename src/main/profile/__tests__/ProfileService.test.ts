/**
 * Tests for ProfileService (TDD-14)
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, following sql-repository.test.ts pattern.
 *
 * @module main/profile/__tests__/ProfileService.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { ProfileService } from '../ProfileService'

// ── helpers ────────────────────────────────────────────────────────────────

async function makeService(): Promise<{
  pool: SqliteSingleConnectionPool
  service: ProfileService
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  return { pool, service: new ProfileService(pool) }
}

/** Insert a minimal user row to satisfy orca_users FK */
async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

// ── tests ──────────────────────────────────────────────────────────────────

describe('ProfileService', () => {
  let pool: SqliteSingleConnectionPool
  let service: ProfileService

  beforeEach(async () => {
    const setup = await makeService()
    pool = setup.pool
    service = setup.service
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  // ── createCompany ──────────────────────────────────────────────────────────

  it('createCompany creates company and returns UUID id', async () => {
    const id = await service.createCompany('Acme Corp', 'admin-1')
    expect(id).toMatch(/^[0-9a-f-]{36}$/)
  })

  it('createCompany: two companies get different IDs', async () => {
    const id1 = await service.createCompany('Alpha', 'admin-1')
    const id2 = await service.createCompany('Beta', 'admin-1')
    expect(id1).not.toBe(id2)
  })

  // ── getCompanyProfile / setCompanyProfile ──────────────────────────────────

  it('getCompanyProfile returns empty object {} for new company', async () => {
    const id = await service.createCompany('Test', 'admin-1')
    const profile = await service.getCompanyProfile(id)
    expect(profile).toEqual({})
  })

  it('getCompanyProfile returns null for unknown company', async () => {
    const profile = await service.getCompanyProfile('non-existent')
    expect(profile).toBeNull()
  })

  it('setCompanyProfile → getCompanyProfile round-trip', async () => {
    const id = await service.createCompany('Test', 'admin-1')
    await service.setCompanyProfile(id, { agent: { preferredModel: 'claude-3-5' } }, 'admin-1')
    const profile = await service.getCompanyProfile(id)
    expect(profile?.agent?.preferredModel).toBe('claude-3-5')
  })

  // ── createDepartment ───────────────────────────────────────────────────────

  it('createDepartment creates dept under company and returns UUID id', async () => {
    const companyId = await service.createCompany('Test', 'admin-1')
    const deptId = await service.createDepartment(companyId, 'Engineering')
    expect(deptId).toMatch(/^[0-9a-f-]{36}$/)
  })

  it('createDepartment: accepts optional parentDeptId', async () => {
    const companyId = await service.createCompany('Test', 'admin-1')
    const parentId = await service.createDepartment(companyId, 'Engineering')
    const childId = await service.createDepartment(companyId, 'Frontend', parentId)
    expect(childId).toMatch(/^[0-9a-f-]{36}$/)
    expect(childId).not.toBe(parentId)
  })

  // ── getDeptProfile / setDeptProfile ────────────────────────────────────────

  it('getDeptProfile returns empty object {} for new dept', async () => {
    const companyId = await service.createCompany('Test', 'admin-1')
    const deptId = await service.createDepartment(companyId, 'Engineering')
    const profile = await service.getDeptProfile(deptId)
    expect(profile).toEqual({})
  })

  it('setDeptProfile → getDeptProfile round-trip', async () => {
    const companyId = await service.createCompany('Test', 'admin-1')
    const deptId = await service.createDepartment(companyId, 'Engineering')
    await service.setDeptProfile(deptId, { shell: { pathAdditions: ['/opt/bin'] } }, 'admin-1')
    const profile = await service.getDeptProfile(deptId)
    expect(profile?.shell?.pathAdditions).toContain('/opt/bin')
  })

  // ── getUserProfile / setUserProfile (upsert) ───────────────────────────────

  it('getUserProfile returns null for user with no profile set', async () => {
    await insertUser(pool, 'u-no-profile')
    const profile = await service.getUserProfile('u-no-profile')
    expect(profile).toBeNull()
  })

  it('setUserProfile stores profile (first insert)', async () => {
    await insertUser(pool, 'u-1')
    await service.setUserProfile('u-1', { editor: { tabSize: 2 } })
    const profile = await service.getUserProfile('u-1')
    expect(profile?.editor?.tabSize).toBe(2)
  })

  it('setUserProfile upserts — second call replaces existing', async () => {
    await insertUser(pool, 'u-2')
    await service.setUserProfile('u-2', { editor: { tabSize: 2 } })
    await service.setUserProfile('u-2', { editor: { tabSize: 4 } })
    const profile = await service.getUserProfile('u-2')
    expect(profile?.editor?.tabSize).toBe(4)
  })

  // ── setUserDepartment + JOIN queries ───────────────────────────────────────

  it('getCompanyProfileForUser returns null when user has no dept', async () => {
    await insertUser(pool, 'u-no-dept')
    const profile = await service.getCompanyProfileForUser('u-no-dept')
    expect(profile).toBeNull()
  })

  it('getDeptProfileForUser returns null when user has no dept', async () => {
    await insertUser(pool, 'u-no-dept-2')
    const profile = await service.getDeptProfileForUser('u-no-dept-2')
    expect(profile).toBeNull()
  })

  it('getCompanyProfileForUser returns company profile via JOIN after setUserDepartment', async () => {
    const companyId = await service.createCompany('Acme', 'admin-1')
    const deptId = await service.createDepartment(companyId, 'Eng')
    await service.setCompanyProfile(companyId, { agent: { maxConcurrentAgents: 5 } }, 'admin-1')
    await insertUser(pool, 'u-3')
    await service.setUserDepartment('u-3', deptId)
    const profile = await service.getCompanyProfileForUser('u-3')
    expect(profile?.agent?.maxConcurrentAgents).toBe(5)
  })

  it('getDeptProfileForUser returns dept profile via JOIN after setUserDepartment', async () => {
    const companyId = await service.createCompany('Acme', 'admin-1')
    const deptId = await service.createDepartment(companyId, 'Eng')
    await service.setDeptProfile(deptId, { editor: { defaultEditor: 'vim' } }, 'admin-1')
    await insertUser(pool, 'u-4')
    await service.setUserDepartment('u-4', deptId)
    const profile = await service.getDeptProfileForUser('u-4')
    expect(profile?.editor?.defaultEditor).toBe('vim')
  })

  it('setUserDepartment updates to new dept', async () => {
    const companyId = await service.createCompany('Acme', 'admin-1')
    const dept1Id = await service.createDepartment(companyId, 'Engineering')
    const dept2Id = await service.createDepartment(companyId, 'Design')
    await service.setDeptProfile(dept2Id, { editor: { theme: 'dark' } }, 'admin-1')
    await insertUser(pool, 'u-5')
    await service.setUserDepartment('u-5', dept1Id)
    await service.setUserDepartment('u-5', dept2Id) // re-assign
    const profile = await service.getDeptProfileForUser('u-5')
    expect(profile?.editor?.theme).toBe('dark')
  })

  // ── complex profile storage ────────────────────────────────────────────────

  it('setCompanyProfile stores nested security section', async () => {
    const id = await service.createCompany('Secure Corp', 'admin-1')
    await service.setCompanyProfile(
      id,
      { security: { approvedModels: ['claude-3-5', 'gpt-4'], maxSessionHours: 8 } },
      'admin-1'
    )
    const profile = await service.getCompanyProfile(id)
    expect(profile?.security?.approvedModels).toContain('claude-3-5')
    expect(profile?.security?.maxSessionHours).toBe(8)
  })
})
