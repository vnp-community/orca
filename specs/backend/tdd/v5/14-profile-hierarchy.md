# TDD-14: User Profile Hierarchy

**Document:** TDD-14 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Profile Hierarchy — 3-layer deep-merge (Company → Dept → User)
**Feature:** F33, F34 (partial)
**ADR:** ADR-007
**HLD Ref:** C3.10a, C4.7
**Source files (to create):**
- `src/main/profile/OrcaProfile.ts`
- `src/main/profile/ProfileResolver.ts`
- `src/main/profile/ProfileService.ts`
- `src/main/runtime/rpc/methods/profile.ts`
- `src/main/db/migrations/0006_company_dept.ts`

> **Status: ❌ TODO** — v5.0 proposed; migration 0006 prerequisite

---

## 1. Mục tiêu

Cho phép mỗi user trong Orca Web Server có **profile riêng** kế thừa từ Company → Department → User. Admin kiểm soát security policy ở tầng công ty; user chỉ override những gì được phép.

---

## 2. OrcaProfile Schema

```typescript
// src/main/profile/OrcaProfile.ts

export interface McpServerConfig {
  name: string
  command: string
  args?: string[]
  env?: Record<string, string>
}

export interface AgentProfileSection {
  preferredModel?: string        // e.g. 'claude-opus-4-5', 'gpt-4o'
  trustPreset?: 'minimal' | 'standard' | 'full'
  mcpServers?: McpServerConfig[]
  customInstructions?: string
  maxConcurrentAgents?: number
}

export interface EditorProfileSection {
  defaultEditor?: string         // 'vscode' | 'cursor' | 'vim' | 'emacs'
  tabSize?: number
  insertSpaces?: boolean
  theme?: string
}

export interface ShellProfileSection {
  defaultShell?: string          // '/bin/bash' | '/bin/zsh' | 'pwsh'
  pathAdditions?: string[]       // prepend to PATH (all layers concatenated)
  envVars?: Record<string, string> // override merge (User > Dept > Company)
}

export interface SecurityProfileSection {
  approvedModels?: string[]      // LOCKED — Company sets, lower tiers cannot override
  disallowedCommands?: string[]  // LOCKED at Company level
  requireReviewBeforeCommit?: boolean
  maxSessionHours?: number
}

export interface OrcaProfile {
  agent?: AgentProfileSection
  editor?: EditorProfileSection
  shell?: ShellProfileSection
  mcp?: { servers?: McpServerConfig[] }
  security?: SecurityProfileSection   // LOCKED if set at Company level
  envVars?: Record<string, string>    // top-level shorthand (same as shell.envVars)
}

/** Resolved profile with source attribution per field path */
export interface ResolvedProfile extends OrcaProfile {
  _sources: Record<string, 'company' | 'dept' | 'user'>
  _resolvedAt: number  // Date.now()
}

/** Merge options */
export interface ProfileMergeOptions {
  lockedSections: Array<keyof OrcaProfile>
}
```

---

## 3. ProfileResolver — Deep-Merge Algorithm

```typescript
// src/main/profile/ProfileResolver.ts

const PROFILE_TTL_MS = 60_000  // 60 seconds

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
      this.cache.clear()  // Company profile change → invalidate all
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

  // Locked sections: always use company value
  for (const section of options.lockedSections) {
    if (company[section] !== undefined) {
      ;(result as any)[section] = company[section]
      sources[section] = 'company'
    }
  }

  // Agent section: scalar override (User > Dept > Company)
  result.agent = mergeScalarSection('agent', company, dept, user, sources)

  // Editor section: scalar override
  result.editor = mergeScalarSection('editor', company, dept, user, sources)

  // Shell section: special handling
  result.shell = {
    defaultShell: pickFirst('shell.defaultShell', [user.shell, dept.shell, company.shell], sources),
    // pathAdditions: CONCAT all (company + dept + user)
    pathAdditions: [
      ...(company.shell?.pathAdditions ?? []),
      ...(dept.shell?.pathAdditions ?? []),
      ...(user.shell?.pathAdditions ?? []),
    ],
    // envVars: key-level override (User wins per-key)
    envVars: {
      ...(company.shell?.envVars ?? {}),
      ...(dept.shell?.envVars ?? {}),
      ...(user.shell?.envVars ?? {}),
    },
  }
  sources['shell.pathAdditions'] = 'user'  // concatenated
  sources['shell.envVars'] = 'user'

  // MCP servers: union by name (User overrides same-name)
  const mcpMap = new Map<string, McpServerConfig>()
  for (const s of [...(company.mcp?.servers ?? []), ...(dept.mcp?.servers ?? []),
                    ...(user.mcp?.servers ?? [])]) {
    mcpMap.set(s.name, s)
  }
  result.mcp = { servers: [...mcpMap.values()] }

  return { ...result, _sources: sources, _resolvedAt: Date.now() }
}
```

---

## 4. ProfileService — CRUD

```typescript
// src/main/profile/ProfileService.ts

export class ProfileService {
  constructor(private readonly pool: IConnectionPool) {}

  // Company profile
  async getCompanyProfile(companyId: string): Promise<OrcaProfile | null>
  async setCompanyProfile(companyId: string, profile: OrcaProfile, updatedBy: string): Promise<void>

  // Department profile
  async getDeptProfile(deptId: string): Promise<OrcaProfile | null>
  async setDeptProfile(deptId: string, profile: OrcaProfile, updatedBy: string): Promise<void>

  // User profile
  async getUserProfile(userId: string): Promise<OrcaProfile | null>
  async setUserProfile(userId: string, profile: OrcaProfile): Promise<void>

  // Helpers: resolve parent chain
  async getCompanyProfileForUser(userId: string): Promise<OrcaProfile | null>
  async getDeptProfileForUser(userId: string): Promise<OrcaProfile | null>

  // Company/Dept CRUD
  async createCompany(name: string, adminUserId: string): Promise<string>  // returns id
  async createDepartment(companyId: string, name: string, parentDeptId?: string): Promise<string>
  async setUserDepartment(userId: string, deptId: string): Promise<void>
}
```

---

## 5. RPC Methods (`src/main/runtime/rpc/methods/profile.ts`)

```typescript
// namespace: 'profile'

'profile.getResolved'     // → ResolvedProfile (for current user)
'profile.getCompany'      // → OrcaProfile (admin only)
'profile.updateCompany'   // (admin only) → void
'profile.getDept'         // → OrcaProfile (admin/lead only)
'profile.updateDept'      // (admin/lead) → void
'profile.getUserProfile'  // → OrcaProfile (own user)
'profile.updateUser'      // (own user, non-locked fields) → void
'profile.invalidate'      // (admin) → void — force cache clear for userId
'profile.listDepts'       // → Department[] for company
'profile.setUserDept'     // (admin) → void
```

---

## 6. Permission Model

| RPC | Required Role |
|-----|--------------|
| `profile.getResolved` | any authenticated user |
| `profile.getCompany` | admin |
| `profile.updateCompany` | admin |
| `profile.getDept` | admin or team-lead of that dept |
| `profile.updateDept` | admin or team-lead of that dept |
| `profile.getUserProfile` | own userId or admin |
| `profile.updateUser` | own userId (locked fields rejected) |
| `profile.invalidate` | admin only |
| `profile.setUserDept` | admin only |

---

## 7. Error Handling

| Scenario | Error code |
|---------|-----------|
| User has no company | `PROFILE_NO_COMPANY` — return base defaults |
| Locked field override attempt | `PROFILE_FIELD_LOCKED` — 403 |
| Company not found | `COMPANY_NOT_FOUND` — 404 |
| Dept not found | `DEPT_NOT_FOUND` — 404 |
| Profile JSON parse error | `PROFILE_INVALID_JSON` — 400 |
| Cache invalidate non-existent user | silent no-op |

---

## 8. Test Coverage

```
src/main/profile/__tests__/
├── ProfileResolver.test.ts
│   ├── cache hit (TTL not expired)
│   ├── cache miss → recompute
│   ├── locked section: user cannot override security
│   ├── pathAdditions: concatenation (company+dept+user)
│   ├── envVars: user key overrides company key
│   ├── mcpServers: union by name (user overrides same name)
│   └── invalidate: cache cleared
├── ProfileService.test.ts
│   ├── setCompanyProfile → getCompanyProfile (round-trip)
│   ├── setUserDept → getCompanyProfileForUser resolves chain
│   └── createDepartment (nested parent)
└── profile-rpc.test.ts
    ├── profile.getResolved (authenticated user)
    ├── profile.updateCompany (admin OK, non-admin 403)
    ├── profile.updateUser (own user OK, locked field rejected)
    └── profile.invalidate (admin only)
```

**Target:** ≥ 30 tests, 100% branch coverage on deepMergeProfiles()
