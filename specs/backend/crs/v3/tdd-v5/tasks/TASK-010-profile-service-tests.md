# TASK-010: ProfileService Tests

**Phase:** 2 — Profile Hierarchy  
**Solution ref:** [SOL-V5-001](../solutions/SOL-V5-001-profile-hierarchy.md) §6  
**Prerequisite:** TASK-007 (ProfileService implementation)  
**Status:** ✅ DONE — 2026-07-28

---

## File cần tạo: `src/main/profile/__tests__/ProfileService.test.ts`

**Setup pattern** (follow auth-session-store.test.ts):
```typescript
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { ProfileService } from '../ProfileService'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

describe('ProfileService', () => {
  let tmpDir: string
  let pool: SqliteSingleConnectionPool
  let service: ProfileService

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-profile-test-'))
    pool = new SqliteSingleConnectionPool(join(tmpDir, 'test.db'))
    await pool.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      await runner.migrate()
    })
    service = new ProfileService(pool)
  })

  afterEach(() => {
    rmSync(tmpDir, { recursive: true })
  })
  // ... tests
})
```

**Test cases cần viết (≥ 10 tests):**

```typescript
// 1. createCompany → returns UUID string
it('createCompany creates company and returns id', async () => {
  const id = await service.createCompany('Acme Corp', 'admin-1')
  expect(id).toMatch(/^[0-9a-f-]{36}$/)
})

// 2. getCompanyProfile → null trước khi set
it('getCompanyProfile returns null for new company', async () => {
  const id = await service.createCompany('Test', 'admin-1')
  const profile = await service.getCompanyProfile(id)
  expect(profile).toEqual({})  // default empty
})

// 3. setCompanyProfile + getCompanyProfile round-trip
it('setCompanyProfile → getCompanyProfile round-trip', async () => {
  const id = await service.createCompany('Test', 'admin-1')
  await service.setCompanyProfile(id, { agent: { preferredModel: 'claude-3-5' } }, 'admin-1')
  const profile = await service.getCompanyProfile(id)
  expect(profile?.agent?.preferredModel).toBe('claude-3-5')
})

// 4. createDepartment with company
it('createDepartment creates dept under company', async () => {
  const companyId = await service.createCompany('Test', 'admin-1')
  const deptId = await service.createDepartment(companyId, 'Engineering')
  expect(deptId).toMatch(/^[0-9a-f-]{36}$/)
})

// 5. setDeptProfile + getDeptProfile round-trip
it('setDeptProfile → getDeptProfile round-trip', async () => {
  const companyId = await service.createCompany('Test', 'admin-1')
  const deptId = await service.createDepartment(companyId, 'Engineering')
  await service.setDeptProfile(deptId, { shell: { pathAdditions: ['/opt/bin'] } }, 'admin-1')
  const profile = await service.getDeptProfile(deptId)
  expect(profile?.shell?.pathAdditions).toContain('/opt/bin')
})

// 6. setUserProfile + getUserProfile (upsert)
it('setUserProfile upserts correctly', async () => {
  // Need user in orca_users — insert directly
  await pool.query('INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
    ['u-1', 'test@test.com', 'Test', 'developer', 'none', Date.now()])
  await service.setUserProfile('u-1', { editor: { tabSize: 2 } })
  const profile = await service.getUserProfile('u-1')
  expect(profile?.editor?.tabSize).toBe(2)
  // Update (upsert)
  await service.setUserProfile('u-1', { editor: { tabSize: 4 } })
  const updated = await service.getUserProfile('u-1')
  expect(updated?.editor?.tabSize).toBe(4)
})

// 7. setUserDepartment + getCompanyProfileForUser chain
it('getCompanyProfileForUser returns company profile via JOIN', async () => {
  // Setup: company → dept → user
  const companyId = await service.createCompany('Acme', 'admin-1')
  const deptId = await service.createDepartment(companyId, 'Eng')
  await service.setCompanyProfile(companyId, { agent: { maxConcurrentAgents: 5 } }, 'admin-1')
  // Insert user
  await pool.query('INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
    ['u-2', 'dev@test.com', 'Dev', 'developer', 'none', Date.now()])
  await service.setUserDepartment('u-2', deptId)
  const profile = await service.getCompanyProfileForUser('u-2')
  expect(profile?.agent?.maxConcurrentAgents).toBe(5)
})

// 8. getDeptProfileForUser returns dept profile
it('getDeptProfileForUser returns dept profile via user', async () => {
  const companyId = await service.createCompany('Acme', 'admin-1')
  const deptId = await service.createDepartment(companyId, 'Eng')
  await service.setDeptProfile(deptId, { editor: { defaultEditor: 'vim' } }, 'admin-1')
  await pool.query('INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
    ['u-3', 'dev3@test.com', 'Dev3', 'developer', 'none', Date.now()])
  await service.setUserDepartment('u-3', deptId)
  const profile = await service.getDeptProfileForUser('u-3')
  expect(profile?.editor?.defaultEditor).toBe('vim')
})

// 9. getUserProfile returns null when no user profile
it('getUserProfile returns null for user with no profile', async () => {
  await pool.query('INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
    ['u-4', 'noprofile@test.com', 'NP', 'developer', 'none', Date.now()])
  const profile = await service.getUserProfile('u-4')
  expect(profile).toBeNull()
})

// 10. getCompanyProfileForUser returns null for user without dept
it('getCompanyProfileForUser returns null when no dept', async () => {
  await pool.query('INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
    ['u-5', 'nodept@test.com', 'ND', 'developer', 'none', Date.now()])
  const profile = await service.getCompanyProfileForUser('u-5')
  expect(profile).toBeNull()
})
```

---

## Verification

```bash
pnpm test --run src/main/profile/__tests__/ProfileService.test.ts
```

## Acceptance Criteria

- [x] ≥ 10 tests
- [x] All tests pass
- [x] Tests cover: createCompany, createDept, round-trip CRUD, upsert, JOIN queries
