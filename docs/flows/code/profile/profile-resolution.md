# Profile Resolution Flow — F33 User Profile Hierarchy

> **Scope**: Luồng giải quyết profile 3 tầng (Company ← Department ← User) và inject vào Agent environment
>
> **Key files**:
> - [`src/main/profile/profile-resolver.ts`](../../src/main/profile/profile-resolver.ts) — ProfileResolver
> - [`src/main/profile/profile-cache.ts`](../../src/main/profile/profile-cache.ts) — In-memory cache TTL 60s
> - [`src/main/profile/profile-service.ts`](../../src/main/profile/profile-service.ts) — CRUD for all tiers
> - [`src/main/profile/company-service.ts`](../../src/main/profile/company-service.ts) — CompanyService
> - [`src/main/profile/department-service.ts`](../../src/main/profile/department-service.ts) — DepartmentService
> - [`src/main/project/profile-aware-agent-spawner.ts`](../../src/main/project/profile-aware-agent-spawner.ts) — Agent spawn with profile
> - **Feature**: [F33 Profile Hierarchy](../features/F33-user-profile-hierarchy.md)
> - **Business Logic**: [BL-PRF-01](../logic/profile/BL-PRF-01-profile-crud.md), [BL-PRF-02](../logic/profile/BL-PRF-02-profile-inheritance.md), [BL-PRF-04](../logic/profile/BL-PRF-04-profile-aware-agent-execution.md)

---

## 1. Tổng quan — 3-Tier Profile Hierarchy

```
┌─────────────────────────────────────────────┐
│  Company Profile (1 per Orca server)        │
│  orca_company.profile_json                  │
│  → Security, allowed models, fleet policy   │
├─────────────────────────────────────────────┤
│  Department Profile (N per company)         │
│  orca_departments.profile_json              │
│  → Team-specific tools, env vars, reviewer  │
├─────────────────────────────────────────────┤
│  User Profile (1 per user)                  │
│  orca_users.profile_json                    │
│  → Personal preferences, local overrides    │
└─────────────────────────────────────────────┘
                    │
                    ▼ deepMerge(company ← dept ← user)
             ResolvedProfile
             (cached 60s per userId)
```

---

## 2. Data Flow: resolve(userId)

```
Caller: ProfileAwareAgentSpawner.spawn(userId, ...)
    │
    ▼ ProfileResolver.resolve(userId)
    │
    ├── ProfileCache.get(userId)
    │   → HIT: return cached ResolvedProfile (TTL 60s)
    │   → MISS: continue
    │
    ├── DB: SELECT u.profile_json, u.department_id
    │        FROM orca_users WHERE id = userId
    │
    ├── DB: SELECT d.profile_json, d.company_id
    │        FROM orca_departments WHERE id = department_id
    │
    ├── DB: SELECT c.profile_json
    │        FROM orca_company WHERE id = company_id
    │
    ├── deepMergeProfiles(companyProfile, deptProfile, userProfile)
    │   → Result: ResolvedProfile { agent, editor, shell, integrations, fleet, security }
    │   → _sources: { 'agent.preferredModel': 'user', 'shell.envVars': 'department', ... }
    │
    ├── Validate: security.approvedModels restriction
    │   if company.approvedModels && !approvedModels.includes(user.agent.preferredModel)
    │     → override user.agent.preferredModel with company default
    │
    ├── ProfileCache.set(userId, resolved, TTL=60s)
    │
    └── return ResolvedProfile
```

---

## 3. deepMergeProfiles() Algorithm

```typescript
// src/main/profile/profile-resolver.ts
function deepMergeProfiles(
  company: OrcaProfile,
  dept: OrcaProfile,
  user: OrcaProfile
): ResolvedProfile {
  const sources: Record<string, 'company' | 'department' | 'user'> = {}

  // Rule 1: User > Dept > Company (user wins by default)
  // Rule 2: shell.pathAdditions = CONCAT (not override)
  // Rule 3: shell.envVars = deep merge (user key wins)
  // Rule 4: security = Company ONLY (user/dept cannot override)

  const merged: OrcaProfile = {
    agent: mergeObject([company.agent, dept.agent, user.agent], sources, 'agent'),
    editor: mergeObject([company.editor, dept.editor, user.editor], sources, 'editor'),
    shell: {
      ...mergeObject([company.shell, dept.shell, user.shell], sources, 'shell'),
      // Special: pathAdditions CONCAT all tiers
      pathAdditions: [
        ...(company.shell?.pathAdditions ?? []),
        ...(dept.shell?.pathAdditions ?? []),
        ...(user.shell?.pathAdditions ?? []),
      ],
      // Special: envVars deep-merge (user key overrides dept overrides company)
      envVars: {
        ...company.shell?.envVars,
        ...dept.shell?.envVars,
        ...user.shell?.envVars,
      },
    },
    integrations: mergeObject([company.integrations, dept.integrations, user.integrations], sources, 'integrations'),
    fleet: mergeObject([company.fleet, dept.fleet, user.fleet], sources, 'fleet'),
    // Security: company-level ONLY, user/dept cannot override
    security: company.security,
  }

  return { ...merged, _sources: sources }
}
```

---

## 4. Cache Invalidation

```typescript
// src/main/profile/profile-cache.ts
class ProfileCache {
  private cache = new Map<string, { value: ResolvedProfile; expiresAt: number }>()
  private TTL_MS = 60_000  // 60 seconds

  get(userId: string): ResolvedProfile | null {
    const entry = this.cache.get(userId)
    if (!entry || Date.now() > entry.expiresAt) {
      this.cache.delete(userId)
      return null
    }
    return entry.value
  }

  set(userId: string, profile: ResolvedProfile): void {
    this.cache.set(userId, { value: profile, expiresAt: Date.now() + this.TTL_MS })
  }

  // Invalidate when profile data changes:
  invalidateByUser(userId: string): void { this.cache.delete(userId) }

  invalidateByDept(deptId: string): void {
    // Must invalidate all users in this department
    // → triggers DB lookup to find affected users
    for (const [uid, _] of this.cache) {
      if (userBelongsToDept(uid, deptId)) this.cache.delete(uid)
    }
  }

  invalidateByCompany(): void {
    // Company profile change → invalidate ALL users
    this.cache.clear()
  }
}
```

### Trigger Matrix

| Event | Invalidation Scope |
|---|---|
| Admin updates Company profile | `invalidateByCompany()` — toàn bộ |
| Lead updates Department profile | `invalidateByDept(deptId)` |
| User updates own profile | `invalidateByUser(userId)` |
| User moves to different dept | `invalidateByUser(userId)` |
| Orca server restart | Cache empty (in-memory) |

---

## 5. Profile Injection vào Agent

```typescript
// src/main/project/profile-aware-agent-spawner.ts
async spawn(input: AgentSpawnInput): Promise<AgentSession> {
  const { userId, projectId, worktreePath, userPrompt } = input

  // 1. Resolve profile (60s cache)
  const profile = await ProfileResolver.resolve(userId)

  // 2. Resolve agent binary
  const model = profile.agent?.preferredModel ?? 'claude'
  const agentBinary = resolveAgentBinary(model)
  // → 'claude' | 'codex' | 'gemini' | 'opencode'

  // 3. Build env from profile
  const env: Record<string, string> = {
    // Shell env vars from profile (merged 3 tiers)
    ...profile.shell?.envVars,

    // PATH extension
    PATH: [
      ...(profile.shell?.pathAdditions ?? []),
      process.env.PATH,
    ].join(':'),

    // Per-user Git auth isolation (F30 credential store)
    GH_CONFIG_DIR: path.join(userDataPath, 'gh-config'),
    GLAB_CONFIG_DIR: path.join(userDataPath, 'glab-config'),

    // AI credential (per devServer account)
    ANTHROPIC_API_KEY: await ProviderResolver.resolveApiKey(userId, projectId, 'anthropic'),

    // Trust preset
    CLAUDE_TRUST_LEVEL: profile.agent?.trustPreset ?? 'standard',
  }

  // 4. Build agent args
  const agentArgs = buildAgentArgs({
    trust: profile.agent?.trustPreset,
    maxTokens: profile.agent?.maxTokensPerSession,
    autoApproveFileRead: profile.agent?.autoApproveFileRead,
  })

  // 5. Spawn via relay
  const relay = await RelayConnectionPool.getOrConnect(project.devServerId)
  return relay.call('pty.spawn', {
    binary: agentBinary,
    args: agentArgs,
    cwd: worktreePath,
    env,
    userId,
    sessionId: generateSessionId(),
  })
}
```

---

## 6. RPC Methods — profile.*

```typescript
// src/main/runtime/rpc/methods/profile.ts

// GET effective profile (current user)
'profile.getEffective'    // () → ResolvedProfile (cached 60s)

// Update personal profile (non-security sections only)
'profile.updateUser'      // (fields: Partial<OrcaProfile without security>) → void

// Department profiles (lead/admin)
'profile.getDepartment'   // (deptId) → OrcaProfile
'profile.updateDepartment'// (deptId, fields) — requires lead or admin role

// Company profile (admin only)
'profile.getCompany'      // () → OrcaProfile
'profile.updateCompany'   // (fields) — admin only → invalidateByCompany()

// Department management
'profile.listDepartments' // () → Department[]
'profile.createDepartment'// (name, teamLeadId?) — admin
'profile.assignUserToDept'// (userId, deptId) — admin → invalidateByUser()
```

---

## 7. DB Schema (Migration 0006)

```sql
-- Company (singleton per Orca server)
CREATE TABLE orca_company (
  id           TEXT PRIMARY KEY DEFAULT 'default',
  name         TEXT NOT NULL,
  logo_url     TEXT,
  profile_json TEXT DEFAULT '{}',   -- JSON: OrcaProfile
  created_at   INTEGER,
  updated_at   INTEGER
);

-- Departments
CREATE TABLE orca_departments (
  id           TEXT PRIMARY KEY,
  company_id   TEXT NOT NULL REFERENCES orca_company(id),
  name         TEXT NOT NULL,
  team_lead_id TEXT REFERENCES orca_users(id),
  profile_json TEXT DEFAULT '{}',   -- JSON: OrcaProfile
  created_at   INTEGER,
  updated_at   INTEGER
);
CREATE INDEX idx_departments_company ON orca_departments(company_id);

-- Users: thêm 2 columns vào orca_users (tạo từ migration 0004)
ALTER TABLE orca_users ADD COLUMN department_id TEXT REFERENCES orca_departments(id);
ALTER TABLE orca_users ADD COLUMN profile_json  TEXT DEFAULT '{}';
CREATE INDEX idx_users_department ON orca_users(department_id);
```

---

## 8. Full Sequence: User đăng nhập → Agent spawn với profile

```
User đăng nhập (POST /auth/local)
    │
    ▼ WsSessionRouter → getOrCreateUserRuntime(userId)
    │
    ▼ User click "New Agent" for project P
    │
    ▼ RPC: projects.get(P) → { devServerId, repoPath }
    │
    ▼ ProfileResolver.resolve(userId)
    │   → DB: user.profile_json + dept.profile_json + company.profile_json
    │   → deepMerge() → ResolvedProfile { preferredModel: 'claude', envVars: {...} }
    │   → Cache 60s
    │
    ▼ ProfileAwareAgentSpawner.spawn({
    │     userId, projectId, worktreePath,
    │     profile: resolvedProfile
    │   })
    │
    ▼ relay.call('pty.spawn', {
    │     binary: 'claude',
    │     env: { PATH: ..., GH_CONFIG_DIR: ..., ANTHROPIC_API_KEY: ... },
    │     cwd: '/srv/vnp/worktrees/feat-x'
    │   })
    │
    ▼ Dev Server Agent: spawn PTY → claude --trust standard
    │
    ▼ WorkspaceContext.emit('agent.started', { sessionId })
    │
    └── UI: AgentPanel shows output stream
```

---

## 9. Cross-References

| Resource | Mô tả |
|---|---|
| [multi-user-session.md](./multi-user-session.md) | Auth session — tiền đề của profile resolve |
| [project-workspace-switch.md](./project-workspace-switch.md) | switchProject() gọi ProfileResolver |
| [task-agent-execution.md](./task-agent-execution.md) | TaskAgentExecutor dùng ProfileAwareAgentSpawner |
| **HLD C1 Flow 7** | Profile Resolution (Company → Dept → User) |
| **HLD C4.7** | ProfileResolver module detail |
| **HLD C2 Container 13** | Profile & Project Service |
| **F33 Profile Hierarchy** | Feature spec |
| **BL-PRF-02** | deepMerge business logic |
