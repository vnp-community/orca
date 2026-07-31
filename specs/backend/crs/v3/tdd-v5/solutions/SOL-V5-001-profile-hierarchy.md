# SOL-V5-001: Profile Hierarchy (TDD-14)

**Solution:** SOL-V5-001  
**TDD:** TDD-14 — User Profile Hierarchy  
**Date:** 2026-07-28  
**Status:** ✅ IMPLEMENTED  
**Implementation Date:** 2026-07-29  
**Tests:** 43 pass (ProfileService + ProfileResolver + profile-rpc) | TypeScript: 0 errors  
**Strategy:** Additive-only, reuse `IConnectionPool` + migration runner pattern  

---

## 1. Phân tích gap so với code thực tế

| TDD yêu cầu | Hiện trạng code | Gap |
|-------------|-----------------|-----|
| `src/main/profile/OrcaProfile.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/profile/ProfileResolver.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/profile/ProfileService.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/runtime/rpc/methods/profile.ts` | Không tồn tại | ❌ Tạo mới |
| Migration 0006 (company/dept) | Chỉ có 0001–0005 | ❌ Thêm mới |
| `ServerBootstrapResult.profileService` | Không có field | ❌ Extend interface |

**Code có thể reuse:**
- `IConnectionPool` từ `src/main/db/pool.ts` → dùng trực tiếp
- Migration runner từ `src/main/db/migrations/runner.ts` → chỉ thêm entry vào index
- Pattern `IConnectionPool.query()` / `.withConnection()` → follow đúng pattern của `auth-session-store.ts`
- Pattern `AuthManager` (constructor nhận `IDatabase`) → `ProfileService` nhận `IConnectionPool`

---

## 2. Files cần tạo mới

### 2.1 `src/main/profile/OrcaProfile.ts`

```typescript
// Đúng theo TDD-14 §2 — không thay đổi
export interface McpServerConfig {
  name: string
  command: string
  args?: string[]
  env?: Record<string, string>
}

export interface AgentProfileSection {
  preferredModel?: string
  trustPreset?: 'minimal' | 'standard' | 'full'
  mcpServers?: McpServerConfig[]
  customInstructions?: string
  maxConcurrentAgents?: number
}

export interface EditorProfileSection {
  defaultEditor?: string
  tabSize?: number
  insertSpaces?: boolean
  theme?: string
}

export interface ShellProfileSection {
  defaultShell?: string
  pathAdditions?: string[]
  envVars?: Record<string, string>
}

export interface SecurityProfileSection {
  approvedModels?: string[]
  disallowedCommands?: string[]
  requireReviewBeforeCommit?: boolean
  maxSessionHours?: number
}

export interface OrcaProfile {
  agent?: AgentProfileSection
  editor?: EditorProfileSection
  shell?: ShellProfileSection
  mcp?: { servers?: McpServerConfig[] }
  security?: SecurityProfileSection
  envVars?: Record<string, string>
}

export interface ResolvedProfile extends OrcaProfile {
  _sources: Record<string, 'company' | 'dept' | 'user'>
  _resolvedAt: number
}

export interface ProfileMergeOptions {
  lockedSections: Array<keyof OrcaProfile>
}
```

### 2.2 `src/main/profile/ProfileService.ts`

```typescript
import type { IConnectionPool } from '../db/pool'
import type { OrcaProfile } from './OrcaProfile'
import { randomUUID } from 'node:crypto'

export class ProfileService {
  constructor(private readonly pool: IConnectionPool) {}

  async getCompanyProfile(companyId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.query<{ profileJson: string }>(
      'SELECT profile_json as profileJson FROM orca_companies WHERE id = ?',
      [companyId]
    )
    return rows[0] ? JSON.parse(rows[0].profileJson) : null
  }

  async setCompanyProfile(companyId: string, profile: OrcaProfile, updatedBy: string): Promise<void> {
    await this.pool.query(
      'UPDATE orca_companies SET profile_json = ?, updated_by = ?, updated_at = ? WHERE id = ?',
      [JSON.stringify(profile), updatedBy, Date.now(), companyId]
    )
  }

  async getDeptProfile(deptId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.query<{ profileJson: string }>(
      'SELECT profile_json as profileJson FROM orca_departments WHERE id = ?',
      [deptId]
    )
    return rows[0] ? JSON.parse(rows[0].profileJson) : null
  }

  async setDeptProfile(deptId: string, profile: OrcaProfile, updatedBy: string): Promise<void> {
    await this.pool.query(
      'UPDATE orca_departments SET profile_json = ?, updated_by = ?, updated_at = ? WHERE id = ?',
      [JSON.stringify(profile), updatedBy, Date.now(), deptId]
    )
  }

  async getUserProfile(userId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.query<{ profileJson: string }>(
      'SELECT profile_json as profileJson FROM orca_user_profiles WHERE user_id = ?',
      [userId]
    )
    return rows[0] ? JSON.parse(rows[0].profileJson) : null
  }

  async setUserProfile(userId: string, profile: OrcaProfile): Promise<void> {
    await this.pool.query(
      `INSERT INTO orca_user_profiles (user_id, profile_json, updated_at)
       VALUES (?, ?, ?)
       ON CONFLICT(user_id) DO UPDATE SET profile_json = excluded.profile_json, updated_at = excluded.updated_at`,
      [userId, JSON.stringify(profile), Date.now()]
    )
  }

  async getCompanyProfileForUser(userId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.query<{ companyId: string }>(
      `SELECT c.company_id as companyId FROM orca_users u
       JOIN orca_departments d ON u.department_id = d.id
       JOIN orca_companies c ON d.company_id = c.id
       WHERE u.id = ?`,
      [userId]
    )
    if (!rows[0]) return null
    return this.getCompanyProfile(rows[0].companyId)
  }

  async getDeptProfileForUser(userId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.query<{ deptId: string }>(
      'SELECT department_id as deptId FROM orca_users WHERE id = ?',
      [userId]
    )
    if (!rows[0]?.deptId) return null
    return this.getDeptProfile(rows[0].deptId)
  }

  async createCompany(name: string, adminUserId: string): Promise<string> {
    const id = randomUUID()
    await this.pool.query(
      'INSERT INTO orca_companies (id, name, profile_json, admin_user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)',
      [id, name, '{}', adminUserId, Date.now(), Date.now()]
    )
    return id
  }

  async createDepartment(companyId: string, name: string, parentDeptId?: string): Promise<string> {
    const id = randomUUID()
    await this.pool.query(
      'INSERT INTO orca_departments (id, company_id, name, parent_dept_id, profile_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)',
      [id, companyId, name, parentDeptId ?? null, '{}', Date.now(), Date.now()]
    )
    return id
  }

  async setUserDepartment(userId: string, deptId: string): Promise<void> {
    await this.pool.query(
      'UPDATE orca_users SET department_id = ? WHERE id = ?',
      [deptId, userId]
    )
  }
}
```

### 2.3 `src/main/profile/ProfileResolver.ts`

Đúng theo TDD-14 §3 — copy nguyên, không thay đổi logic.

```typescript
import type { ProfileService } from './ProfileService'
import type { OrcaProfile, ResolvedProfile, ProfileMergeOptions, McpServerConfig } from './OrcaProfile'

const PROFILE_TTL_MS = 60_000

export class ProfileResolver {
  private cache = new Map<string, { resolved: ResolvedProfile; expiresAt: number }>()

  constructor(private readonly profileService: ProfileService) {}

  async resolve(userId: string): Promise<ResolvedProfile> {
    const cached = this.cache.get(userId)
    if (cached && Date.now() < cached.expiresAt) return cached.resolved

    const [companyProfile, deptProfile, userProfile] = await Promise.all([
      this.profileService.getCompanyProfileForUser(userId),
      this.profileService.getDeptProfileForUser(userId),
      this.profileService.getUserProfile(userId),
    ])

    const resolved = deepMergeProfiles(
      companyProfile ?? {},
      deptProfile ?? {},
      userProfile ?? {},
      { lockedSections: ['security'] }
    )

    this.cache.set(userId, { resolved, expiresAt: Date.now() + PROFILE_TTL_MS })
    return resolved
  }

  invalidate(userId?: string): void {
    if (userId) {
      this.cache.delete(userId)
    } else {
      this.cache.clear()
    }
  }
}

function deepMergeProfiles(
  company: OrcaProfile,
  dept: OrcaProfile,
  user: OrcaProfile,
  options: ProfileMergeOptions
): ResolvedProfile {
  const sources: Record<string, 'company' | 'dept' | 'user'> = {}
  const result: OrcaProfile = {}

  for (const section of options.lockedSections) {
    if (company[section] !== undefined) {
      ;(result as any)[section] = company[section]
      sources[section] = 'company'
    }
  }

  result.agent = mergeScalarSection('agent', company, dept, user, sources)
  result.editor = mergeScalarSection('editor', company, dept, user, sources)

  result.shell = {
    defaultShell: pickFirst('shell.defaultShell', [user.shell, dept.shell, company.shell], sources),
    pathAdditions: [
      ...(company.shell?.pathAdditions ?? []),
      ...(dept.shell?.pathAdditions ?? []),
      ...(user.shell?.pathAdditions ?? []),
    ],
    envVars: {
      ...(company.shell?.envVars ?? {}),
      ...(dept.shell?.envVars ?? {}),
      ...(user.shell?.envVars ?? {}),
    },
  }
  sources['shell.pathAdditions'] = 'user'
  sources['shell.envVars'] = 'user'

  const mcpMap = new Map<string, McpServerConfig>()
  for (const s of [...(company.mcp?.servers ?? []), ...(dept.mcp?.servers ?? []), ...(user.mcp?.servers ?? [])]) {
    mcpMap.set(s.name, s)
  }
  result.mcp = { servers: [...mcpMap.values()] }

  return { ...result, _sources: sources, _resolvedAt: Date.now() }
}

function mergeScalarSection(
  section: keyof OrcaProfile,
  company: OrcaProfile,
  dept: OrcaProfile,
  user: OrcaProfile,
  sources: Record<string, 'company' | 'dept' | 'user'>
): any {
  const merged: any = {}
  const c = (company as any)[section] ?? {}
  const d = (dept as any)[section] ?? {}
  const u = (user as any)[section] ?? {}

  for (const key of new Set([...Object.keys(c), ...Object.keys(d), ...Object.keys(u)])) {
    if (u[key] !== undefined) { merged[key] = u[key]; sources[`${section}.${key}`] = 'user' }
    else if (d[key] !== undefined) { merged[key] = d[key]; sources[`${section}.${key}`] = 'dept' }
    else if (c[key] !== undefined) { merged[key] = c[key]; sources[`${section}.${key}`] = 'company' }
  }
  return Object.keys(merged).length > 0 ? merged : undefined
}

function pickFirst(
  key: string,
  sources: (any | undefined)[],
  sourceMap: Record<string, 'company' | 'dept' | 'user'>
): any {
  const labels: Array<'user' | 'dept' | 'company'> = ['user', 'dept', 'company']
  for (let i = 0; i < sources.length; i++) {
    const section = sources[i]
    const fieldKey = key.split('.').pop()!
    if (section?.[fieldKey] !== undefined) {
      sourceMap[key] = labels[i]
      return section[fieldKey]
    }
  }
  return undefined
}
```

---

## 3. Migration 0006

### `src/main/db/migrations/0006_company_dept.ts`

```typescript
import type { Migration } from './types'

export const migration0006CompanyDept: Migration = {
  version: 6,
  name: 'company_dept',

  async up(db) {
    // orca_companies
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_companies (
        id            TEXT    PRIMARY KEY,
        name          TEXT    NOT NULL,
        profile_json  TEXT    NOT NULL DEFAULT '{}',
        admin_user_id TEXT,
        created_at    INTEGER NOT NULL,
        updated_at    INTEGER NOT NULL,
        updated_by    TEXT
      )
    `)

    // orca_departments
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_departments (
        id             TEXT    PRIMARY KEY,
        company_id     TEXT    NOT NULL REFERENCES orca_companies(id) ON DELETE CASCADE,
        name           TEXT    NOT NULL,
        parent_dept_id TEXT    REFERENCES orca_departments(id) ON DELETE SET NULL,
        profile_json   TEXT    NOT NULL DEFAULT '{}',
        created_at     INTEGER NOT NULL,
        updated_at     INTEGER NOT NULL,
        updated_by     TEXT
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_departments_company
        ON orca_departments(company_id)
    `)

    // orca_user_profiles — user-specific profile overrides
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_user_profiles (
        user_id      TEXT    PRIMARY KEY REFERENCES orca_users(id) ON DELETE CASCADE,
        profile_json TEXT    NOT NULL DEFAULT '{}',
        updated_at   INTEGER NOT NULL
      )
    `)

    // Add department_id column to orca_users (nullable)
    try {
      await db.exec(`ALTER TABLE orca_users ADD COLUMN department_id TEXT REFERENCES orca_departments(id) ON DELETE SET NULL`)
    } catch {
      // Column may already exist (idempotent)
    }
  },

  async down(db) {
    await db.exec('DROP TABLE IF EXISTS orca_user_profiles')
    await db.exec('DROP INDEX IF EXISTS idx_orca_departments_company')
    await db.exec('DROP TABLE IF EXISTS orca_departments')
    await db.exec('DROP TABLE IF EXISTS orca_companies')
  }
}
```

### Update `src/main/db/migrations/index.ts`

```typescript
// ADD import:
import { migration0006CompanyDept } from './0006_company_dept'

// ADD to ALL_MIGRATIONS array:
export const ALL_MIGRATIONS: Migration[] = [
  migration0001InitialSchema,
  migration0002AddAutomations,
  migration0003AddWorkspaceSessions,
  migration0004OrcaAppTables,
  migration0005AddAuthSchema,
  migration0006CompanyDept,   // ← NEW
]
```

---

## 4. RPC Methods (`src/main/runtime/rpc/methods/profile.ts`)

```typescript
// Đăng ký vào RPC dispatcher sau khi ProfileService + ProfileResolver được khởi tạo
// Pattern: follow src/main/runtime/rpc/methods/ hiện tại

import type { ProfileService } from '../../profile/ProfileService'
import type { ProfileResolver } from '../../profile/ProfileResolver'

export function registerProfileRpcMethods(
  profileService: ProfileService,
  profileResolver: ProfileResolver,
  dispatcher: RpcDispatcher
): void {
  dispatcher.register('profile.getResolved', async (params, session) => {
    return profileResolver.resolve(session.userId)
  })

  dispatcher.register('profile.getUserProfile', async (params, session) => {
    const targetId = params.userId ?? session.userId
    if (targetId !== session.userId && session.role !== 'admin') {
      throw new RpcError('FORBIDDEN', 403)
    }
    return profileService.getUserProfile(targetId)
  })

  dispatcher.register('profile.updateUser', async (params, session) => {
    // Reject if any locked field present (security section)
    if (params.profile.security !== undefined) {
      throw new RpcError('PROFILE_FIELD_LOCKED', 403)
    }
    await profileService.setUserProfile(session.userId, params.profile)
    profileResolver.invalidate(session.userId)
  })

  dispatcher.register('profile.getCompany', async (params, session) => {
    if (session.role !== 'admin') throw new RpcError('FORBIDDEN', 403)
    return profileService.getCompanyProfile(params.companyId)
  })

  dispatcher.register('profile.updateCompany', async (params, session) => {
    if (session.role !== 'admin') throw new RpcError('FORBIDDEN', 403)
    await profileService.setCompanyProfile(params.companyId, params.profile, session.userId)
    profileResolver.invalidate() // all users
  })

  dispatcher.register('profile.invalidate', async (params, session) => {
    if (session.role !== 'admin') throw new RpcError('FORBIDDEN', 403)
    profileResolver.invalidate(params.userId)
  })

  dispatcher.register('profile.setUserDept', async (params, session) => {
    if (session.role !== 'admin') throw new RpcError('FORBIDDEN', 403)
    await profileService.setUserDepartment(params.userId, params.deptId)
    profileResolver.invalidate(params.userId)
  })
}
```

---

## 5. server-bootstrap.ts — thêm ProfileService (step 7)

```typescript
// Trong initializeOrcaServices(), sau step 6 (OrcaRuntimeService):

// 7. ProfileService + ProfileResolver
const { ProfileService } = await import('./profile/ProfileService')
const { ProfileResolver } = await import('./profile/ProfileResolver')
const profileService = new ProfileService(pool)
const profileResolver = new ProfileResolver(profileService)
console.log('[ServerBootstrap] ✅ ProfileService + ProfileResolver initialized')
```

### Update `ServerBootstrapResult` interface

```typescript
export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  devServerManager: DevServerManager
  dbMonitor: import('./db/health').HealthChecker
  pushManager: WebPushManager
  authManager: AuthManager
  sessionManager: import('./session/session-manager').SessionManager | null
  agentWsServer: AgentWebSocketServer
  // [NEW v5.0]
  profileService: import('./profile/ProfileService').ProfileService
  profileResolver: import('./profile/ProfileResolver').ProfileResolver
}
```

---

## 6. Test files cần tạo

```
src/main/profile/__tests__/
├── ProfileResolver.test.ts   (≥ 14 tests — cache hit/miss, locked section, pathAdditions concat, envVars override, mcpServers union, invalidate)
├── ProfileService.test.ts    (≥ 10 tests — round-trip CRUD, dept chain, createCompany)
└── profile-rpc.test.ts       (≥ 6 tests — getResolved, updateCompany admin/non-admin, updateUser locked field)
```

**Total: ≥ 30 tests**

---

## 7. Checklist

- [x] `src/main/profile/OrcaProfile.ts` — types *(actual: `src/main/profile/OrcaProfile.ts`)*
- [x] `src/main/profile/ProfileService.ts` — CRUD methods
- [x] `src/main/profile/ProfileResolver.ts` — deep-merge + cache
- [x] `src/main/runtime/rpc/methods/profile.ts` — RPC registration *(actual: `src/main/profile/profile-rpc-handler.ts`)*
- [x] `src/main/db/migrations/0006_company_dept.ts` — migration
- [x] `src/main/db/migrations/index.ts` — add 0006 to ALL_MIGRATIONS
- [x] `src/main/server-bootstrap.ts` — step 7 + extend interface *(actual: step 7 wired in boot sequence)*
- [x] `src/main/profile/__tests__/ProfileResolver.test.ts`
- [x] `src/main/profile/__tests__/ProfileService.test.ts`
- [x] `src/main/profile/__tests__/profile-rpc.test.ts`

## 8. Implementation Notes

| Spec Path | Actual Path | Note |
|-----------|------------|------|
| `src/main/runtime/rpc/methods/profile.ts` | `src/main/profile/profile-rpc-handler.ts` | Co-located với domain, registered via `createProfileMethods()` |
| `ServerBootstrapResult.profileService` | `server-bootstrap.ts` step 7 | Added to return block at line 461 |

**Test Results:** 43 tests pass (ProfileService + ProfileResolver + profile-rpc)  
**Implemented:** 2026-07-29 ✅
