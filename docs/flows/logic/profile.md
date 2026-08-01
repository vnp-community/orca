# Luồng Dữ liệu — Profile & Project Management

**Domain:** Profile & Project Management  
**Nghiệp vụ:** BL-PRF-01 → BL-PRF-04  
**Kiến trúc tham chiếu:** HLD v1 — Profile/Project Service (C3.10, C4.7/C4.8), ADR-007, F33/F34

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Admin/Lead Browser | UI | Profile management UI, project assignment |
| Orca Web Server | Backend | REST API /api/profiles, /api/projects |
| CompanyService | Business Logic | CRUD company + department profiles |
| ProfileResolver | Business Logic | 3-layer merge, in-memory cache TTL 60s |
| ProjectService | Business Logic | CRUD project + dev server binding |
| ProjectServerRouter | Business Logic | Auto-route đến đúng dev server |
| ProfileAwareAgentSpawner | Business Logic | Inject profile vào agent env |
| Server Database | Persistence | orca_company, orca_departments, orca_projects |

---

## BL-PRF-01 — Tạo và Cập nhật Profile (Company/Dept/User)

```
Admin (Company level)
    │
    ▼
[Admin SPA] Settings → Profiles → Company → Edit
    Input: {
      agentModel: "claude-opus-4",
      envVars: { NODE_ENV: "production", OPENAI_API_BASE: "..." },
      allowedProviders: ["anthropic", "openai"],
      maxConcurrentAgents: 10
    }
    │ PATCH /api/profiles/company
    ▼
[ProfileService.updateCompany()]
    ├─ requireAdmin() guard
    ├─ Validate schema (Zod)
    ├─ UPDATE orca_company SET profile_json=?   ← Server DB
    ├─ Invalidate ProfileResolver cache (flush all userId entries)
    └─ emit: profile:updated { scope: 'company' }

Lead (Department level):
    PATCH /api/profiles/departments/:deptId
    ├─ requireLead() guard (must be dept lead)
    ├─ UPDATE orca_departments SET profile_json=?  ← Server DB
    └─ Invalidate cache for dept users

User (Personal level):
    PATCH /api/profiles/me
    ├─ Auth check (own profile only)
    ├─ UPDATE orca_users SET profile_json=?  ← Server DB
    └─ Invalidate cache for userId

Luồng:
Admin/Lead/User → PATCH /api/profiles/* → ProfileService
               → Server DB (UPDATE profile_json)
               → Cache invalidate
```

---

## BL-PRF-02 — Profile Inheritance Resolution (3-layer merge)

```
[ProfileResolver.resolve(userId)] — called khi agent spawn hoặc context switch
    │
    ▼
[In-memory cache check: profileCache.get(userId)]
    IF hit (TTL < 60s): return cached ResolvedProfile
    │
    IF miss:
    ▼
[Load 3 layers]:
    ├─ SELECT company_profile FROM orca_company        ← Server DB
    ├─ SELECT dept_profile FROM orca_departments
    │   WHERE id = (SELECT dept_id FROM orca_users WHERE id=?)
    └─ SELECT user_profile FROM orca_users WHERE id=?

[Deep merge — Company ← Dept ← User]:
    merged = deepMerge(companyProfile, deptProfile, userProfile)
    // User overrides Dept overrides Company
    // Arrays: user replaces (không concat) — theo ADR-007

[Build ResolvedProfile]:
    {
      agentModel: merged.agentModel,        // e.g. "claude-opus-4"
      envVars: merged.envVars,             // merged env variables
      allowedProviders: merged.allowedProviders,
      maxConcurrentAgents: merged.maxConcurrentAgents,
      path: merged.path,                   // additional PATH entries
    }

[Cache: profileCache.set(userId, resolved, TTL=60s)]
[Return ResolvedProfile]

Luồng:
Caller → ProfileResolver.resolve(userId)
       → Check in-memory cache (TTL 60s)
       → Server DB: 3 SELECT queries (company + dept + user)
       → deepMerge(company ← dept ← user)
       → Cache + return ResolvedProfile
```

---

## BL-PRF-03 — Project-Dev Server Assignment

```
Admin/Lead
    │
    ▼
[Admin SPA] Projects → "New Project" hoặc "Edit Project"
    Input: {
      name: "vnp-backend",
      repoPath: "/home/ubuntu/vnp-blc",
      devServerId: "dev-01",
      members: [{ userId, role: 'developer' | 'lead' }]
    }
    │ POST /api/projects
    ▼
[ProjectService.create()]
    ├─ requireAdmin() or requireLead() guard
    ├─ Validate devServerId exists in orca_dev_servers  ← Server DB
    ├─ INSERT orca_projects { id, name, repoPath, devServerId }  ← Server DB
    ├─ INSERT orca_project_members × N (userId, projectId, role)  ← Server DB
    └─ emit: project:created { projectId }

ASSIGN to different server:
    PATCH /api/projects/:id { devServerId: 'dev-02' }
    ├─ UPDATE orca_projects SET devServerId=?  ← Server DB
    ├─ ProjectServerRouter.updateRouting(projectId, newDevServerId)
    └─ emit: project:serverChanged (triggers workspace reconnect)

[ProjectServerRouter.getServer(projectId)]
    SELECT devServerId FROM orca_projects WHERE id=?  ← Server DB
    → RelayConnectionPool.getOrConnect(devServerId)
    → Return active relay connection

Luồng:
Admin/Lead → POST /api/projects → ProjectService
           → Server DB (INSERT project + members)
           → ProjectServerRouter (update routing table)

Usage (per request):
WorkspaceContext → ProjectServerRouter.getServer(projectId)
                → Server DB → devServerId
                → RelayConnectionPool → relay connection
```

---

## BL-PRF-04 — Profile-Aware Agent Execution Routing

```
Developer/Lead
    │
    ▼
[Renderer] Task → [Run Agent] hoặc worktree → [Start Agent]
    │ contextBridge.invoke('agent.start', { worktreeId, taskId? })
    ▼
[ProfileAwareAgentSpawner.spawn(userId, worktreeId)]
    │
    ├── ProfileResolver.resolve(userId) → ResolvedProfile
    │   { agentModel: "claude-opus-4", envVars: {...}, allowedProviders: [...] }
    │
    ├── ProjectServerRouter.getServer(projectId)
    │   → devServerId: "dev-01"
    │   → relay: RelayConnectionPool.getOrConnect("dev-01")
    │
    ├── AIProviderResolver.resolve(userId, projectId)
    │   → { provider: "anthropic", apiKeyEnvVar: "ANTHROPIC_API_KEY" }
    │   (BL-AIP-02 flow)
    │
    ├── Build agent spawn command:
    │   cmd: resolvedProfile.agentModel  // "claude"
    │   env: {
    │     ...resolvedProfile.envVars,    // company/dept/user env
    │     ANTHROPIC_API_KEY: "<resolved from dev server credential file>",
    │     WORKTREE_PATH: worktreePath,
    │     TASK_ID: taskId
    │   }
    │
    └── relay.call('agent.spawn', { cmd, env, cwd: worktreePath })
        → Dev Server: node-pty.spawn(cmd, { env, cwd })
        → Agent running on dev server with correct profile + provider

Luồng:
User → Renderer → IPC → ProfileAwareAgentSpawner
                       → ProfileResolver (3-layer merge, cached)
                       → ProjectServerRouter (get dev server)
                       → AIProviderResolver (get credentials)
                       → relay.call('agent.spawn') → Dev Server PTY
```

---

## Sơ đồ tổng quan — Profile & Project Management

```
┌──────────────┐  HTTP  ┌─────────────────────────────────────────────┐
│  Admin/Lead  │◄──────►│  Orca Web Server                            │
│  Browser SPA │        │  ProfileService (CRUD)                      │
│  Project mgmt│        │  ProjectService (CRUD + member mgmt)        │
└──────────────┘        └──────────┬──────────────────────────────────┘
                                   │
                        ┌──────────▼──────────────────────────────────┐
                        │  Server Database                             │
                        │  orca_company (company profile)             │
                        │  orca_departments (dept profile)            │
                        │  orca_users.profile_json (user profile)     │
                        │  orca_projects (project + devServerId)      │
                        │  orca_project_members                       │
                        └──────────┬──────────────────────────────────┘
                                   │
                        ┌──────────▼──────────────────────────────────┐
                        │  ProfileResolver (in-memory cache TTL 60s)  │
                        │  3-layer deepMerge: Company ← Dept ← User  │
                        └──────────┬──────────────────────────────────┘
                                   │ ResolvedProfile
                        ┌──────────▼──────────────────────────────────┐
                        │  ProfileAwareAgentSpawner                   │
                        │  + ProjectServerRouter                      │
                        │  + AIProviderResolver                       │
                        └──────────┬──────────────────────────────────┘
                                   │ relay.call('agent.spawn')
                        ┌──────────▼──────────────────────────────────┐
                        │  Dev Server (relay) → node-pty → Agent      │
                        │  (with merged env vars + correct provider)  │
                        └─────────────────────────────────────────────┘
```
