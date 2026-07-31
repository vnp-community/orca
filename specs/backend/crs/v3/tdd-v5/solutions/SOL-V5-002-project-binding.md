# SOL-V5-002: Project-Dev Server Binding (TDD-15)

**Solution:** SOL-V5-002  
**TDD:** TDD-15 — Project-Dev Server Binding  
**Date:** 2026-07-28  
**Status:** ✅ IMPLEMENTED  
**Implementation Date:** 2026-07-29  
**Tests:** 35 pass (ProjectService + ProjectServerRouter + Spawner + project-rpc) | TypeScript: 0 errors  
**Strategy:** Additive-only — reuse `DevServerManager.getServer()`, `IConnectionPool`, `DevServerRelayBridge`

---

## 1. Phân tích gap

| TDD yêu cầu | Hiện trạng code | Gap |
|-------------|-----------------|-----|
| `src/main/project/ProjectService.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/project/ProjectServerRouter.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/project/ProfileAwareAgentSpawner.ts` | Không tồn tại | ❌ Tạo mới |
| `src/main/runtime/rpc/methods/project.ts` | Không tồn tại | ❌ Tạo mới |
| `src/shared/project-types.ts` | Không tồn tại | ❌ Tạo mới |
| Migration 0007 (orca_projects) | Không tồn tại | ❌ Tạo mới |
| `ServerBootstrapResult.projectService` | Không có | ❌ Extend |

**Code có thể reuse:**
- `DevServerManager.getServer(id)` → validate devServerId khi create project
- `DevServerRelayBridge.call()` → relay operations (đã có trong `dev-server-relay-bridge.ts`)
- `IConnectionPool.query()` → follow pattern giống `auth-session-store.ts`
- `ProfileResolver.resolve()` → từ SOL-001
- `RelayConnectionPool` → từ SOL-006 (nên build SOL-006 trước hoặc stub interface)

**Dependency:** SOL-001 (ProfileResolver), SOL-006 (RelayConnectionPool)

---

## 2. `src/shared/project-types.ts`

```typescript
import type { PersistedDevServer } from './dev-server-types'
import type { ResolvedProfile } from '../main/profile/OrcaProfile'

export interface OrcaProject {
  id: string
  name: string
  description?: string
  devServerId: string       // REQUIRED
  repoPath: string
  defaultBranch: string
  visibility: 'private' | 'team' | 'company'
  createdBy: string
  createdAt: Date
  updatedAt: Date
}

export interface ProjectMember {
  projectId: string
  userId: string
  role: 'owner' | 'member' | 'viewer'
  addedAt: Date
}

export interface ProjectContext {
  project: OrcaProject
  member: ProjectMember
  devServer: PersistedDevServer
  resolvedProfile: ResolvedProfile
}
```

---

## 3. Migration 0007

### `src/main/db/migrations/0007_projects.ts`

```typescript
import type { Migration } from './types'

export const migration0007Projects: Migration = {
  version: 7,
  name: 'projects',

  async up(db) {
    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_projects (
        id             TEXT    PRIMARY KEY,
        name           TEXT    NOT NULL,
        description    TEXT,
        dev_server_id  TEXT    NOT NULL,
        repo_path      TEXT    NOT NULL,
        default_branch TEXT    NOT NULL DEFAULT 'main',
        visibility     TEXT    NOT NULL DEFAULT 'team',
        created_by     TEXT    NOT NULL,
        created_at     INTEGER NOT NULL,
        updated_at     INTEGER NOT NULL
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_projects_server
        ON orca_projects(dev_server_id)
    `)

    await db.exec(`
      CREATE TABLE IF NOT EXISTS orca_project_members (
        project_id TEXT    NOT NULL REFERENCES orca_projects(id) ON DELETE CASCADE,
        user_id    TEXT    NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        role       TEXT    NOT NULL DEFAULT 'member',
        added_at   INTEGER NOT NULL,
        PRIMARY KEY (project_id, user_id)
      )
    `)
    await db.exec(`
      CREATE INDEX IF NOT EXISTS idx_orca_project_members_user
        ON orca_project_members(user_id)
    `)
  },

  async down(db) {
    await db.exec('DROP INDEX IF EXISTS idx_orca_project_members_user')
    await db.exec('DROP TABLE IF EXISTS orca_project_members')
    await db.exec('DROP INDEX IF EXISTS idx_orca_projects_server')
    await db.exec('DROP TABLE IF EXISTS orca_projects')
  }
}
```

### Update `src/main/db/migrations/index.ts`

```typescript
import { migration0007Projects } from './0007_projects'

export const ALL_MIGRATIONS = [
  // ... existing 0001–0006 ...
  migration0007Projects,  // ← NEW
]
```

---

## 4. `src/main/project/ProjectService.ts`

```typescript
import type { IConnectionPool } from '../db/pool'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { OrcaProject, ProjectMember } from '../../shared/project-types'
import { randomUUID } from 'node:crypto'

export class ProjectService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly devServerManager: DevServerManager
  ) {}

  async create(params: {
    name: string
    devServerId: string
    repoPath: string
    defaultBranch?: string
    visibility?: string
    createdBy: string
    description?: string
  }): Promise<OrcaProject> {
    // Validate devServerId exists
    const server = this.devServerManager.getServer(params.devServerId)
    if (!server) throw new Error('DEV_SERVER_NOT_FOUND')

    const id = randomUUID()
    const now = Date.now()
    await this.pool.query(
      `INSERT INTO orca_projects (id, name, description, dev_server_id, repo_path, default_branch, visibility, created_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [id, params.name, params.description ?? null, params.devServerId, params.repoPath,
       params.defaultBranch ?? 'main', params.visibility ?? 'team', params.createdBy, now, now]
    )
    await this.addMember(id, params.createdBy, 'owner')
    const project = await this.get(id)
    if (!project) throw new Error('PROJECT_NOT_FOUND')
    return project
  }

  async get(projectId: string): Promise<OrcaProject | null> {
    const rows = await this.pool.query<{
      id: string; name: string; description: string | null; devServerId: string;
      repoPath: string; defaultBranch: string; visibility: string;
      createdBy: string; createdAt: number; updatedAt: number
    }>(
      `SELECT id, name, description, dev_server_id as devServerId, repo_path as repoPath,
              default_branch as defaultBranch, visibility, created_by as createdBy,
              created_at as createdAt, updated_at as updatedAt
       FROM orca_projects WHERE id = ?`,
      [projectId]
    )
    if (!rows[0]) return null
    const r = rows[0]
    return {
      ...r,
      description: r.description ?? undefined,
      createdAt: new Date(r.createdAt),
      updatedAt: new Date(r.updatedAt),
    }
  }

  async list(userId: string): Promise<OrcaProject[]> {
    const rows = await this.pool.query<{ id: string }>(
      `SELECT p.id FROM orca_projects p
       JOIN orca_project_members m ON p.id = m.project_id
       WHERE m.user_id = ?`,
      [userId]
    )
    const projects = await Promise.all(rows.map(r => this.get(r.id)))
    return projects.filter(Boolean) as OrcaProject[]
  }

  async update(projectId: string, patch: Partial<OrcaProject>, updatedBy: string): Promise<void> {
    const fields: string[] = []
    const values: unknown[] = []
    if (patch.name !== undefined) { fields.push('name = ?'); values.push(patch.name) }
    if (patch.description !== undefined) { fields.push('description = ?'); values.push(patch.description) }
    if (patch.visibility !== undefined) { fields.push('visibility = ?'); values.push(patch.visibility) }
    if (patch.defaultBranch !== undefined) { fields.push('default_branch = ?'); values.push(patch.defaultBranch) }
    fields.push('updated_at = ?'); values.push(Date.now())
    values.push(projectId)
    await this.pool.query(`UPDATE orca_projects SET ${fields.join(', ')} WHERE id = ?`, values)
  }

  async delete(projectId: string, deletedBy: string): Promise<void> {
    await this.pool.query('DELETE FROM orca_projects WHERE id = ?', [projectId])
  }

  async addMember(projectId: string, userId: string, role: ProjectMember['role']): Promise<void> {
    await this.pool.query(
      `INSERT INTO orca_project_members (project_id, user_id, role, added_at)
       VALUES (?, ?, ?, ?)
       ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role`,
      [projectId, userId, role, Date.now()]
    )
  }

  async removeMember(projectId: string, userId: string): Promise<void> {
    await this.pool.query(
      'DELETE FROM orca_project_members WHERE project_id = ? AND user_id = ?',
      [projectId, userId]
    )
  }

  async updateMemberRole(projectId: string, userId: string, role: ProjectMember['role']): Promise<void> {
    await this.pool.query(
      'UPDATE orca_project_members SET role = ? WHERE project_id = ? AND user_id = ?',
      [role, projectId, userId]
    )
  }

  async getMembers(projectId: string): Promise<ProjectMember[]> {
    const rows = await this.pool.query<{ userId: string; role: string; addedAt: number }>(
      'SELECT user_id as userId, role, added_at as addedAt FROM orca_project_members WHERE project_id = ?',
      [projectId]
    )
    return rows.map(r => ({ projectId, userId: r.userId, role: r.role as ProjectMember['role'], addedAt: new Date(r.addedAt) }))
  }

  async getMember(projectId: string, userId: string): Promise<ProjectMember | null> {
    const rows = await this.pool.query<{ role: string; addedAt: number }>(
      'SELECT role, added_at as addedAt FROM orca_project_members WHERE project_id = ? AND user_id = ?',
      [projectId, userId]
    )
    if (!rows[0]) return null
    return { projectId, userId, role: rows[0].role as ProjectMember['role'], addedAt: new Date(rows[0].addedAt) }
  }

  async assertAccess(projectId: string, userId: string): Promise<ProjectMember> {
    const member = await this.getMember(projectId, userId)
    if (!member) throw new Error('PROJECT_ACCESS_DENIED')
    return member
  }
}
```

---

## 5. `src/main/project/ProjectServerRouter.ts`

```typescript
import type { ProjectService } from './ProjectService'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../dev-server/relay-connection-pool'
import type { ProfileResolver } from '../profile/ProfileResolver'
import type { ProjectContext, OrcaProject } from '../../shared/project-types'

export class ProjectServerRouter {
  constructor(
    private readonly projectService: ProjectService,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool
  ) {}

  async getRelayForProject(projectId: string, userId: string) {
    await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)
    if (!project) throw new Error('PROJECT_NOT_FOUND')

    const server = this.devServerManager.getServer(project.devServerId)
    if (!server) throw new Error('DEV_SERVER_NOT_FOUND')

    return this.relayPool.getOrConnect(project.devServerId, server)
  }

  async getProjectContext(
    projectId: string,
    userId: string,
    profileResolver: ProfileResolver
  ): Promise<ProjectContext> {
    const member = await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)
    if (!project) throw new Error('PROJECT_NOT_FOUND')
    const devServer = this.devServerManager.getServer(project.devServerId)
    if (!devServer) throw new Error('DEV_SERVER_NOT_FOUND')
    const resolvedProfile = await profileResolver.resolve(userId)
    return { project, member, devServer, resolvedProfile }
  }

  async getProject(projectId: string): Promise<OrcaProject | null> {
    return this.projectService.get(projectId)
  }
}
```

---

## 6. `src/main/project/ProfileAwareAgentSpawner.ts`

Đúng theo TDD-15 §5, chỉ cần thay `ProviderResolver` bằng `AIProviderService` (từ SOL-003):

```typescript
import type { ProjectServerRouter } from './ProjectServerRouter'
import type { ProfileResolver } from '../profile/ProfileResolver'

export interface AgentSpawnOptions {
  projectId: string
  userId: string
  worktreePath: string
  prompt: string
  taskId?: string
  accountId?: string
}

export class ProfileAwareAgentSpawner {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly profileResolver: ProfileResolver,
    private readonly providerService: import('../ai-providers/AIProviderService').AIProviderService
  ) {}

  async spawn(options: AgentSpawnOptions): Promise<unknown> {
    const context = await this.router.getProjectContext(
      options.projectId, options.userId, this.profileResolver
    )
    const { project, resolvedProfile } = context

    // Resolve AI provider (từ SOL-003)
    const account = options.accountId
      ? await this.providerService.getAccount(options.accountId)
      : await this.providerService.resolveForProject(project.devServerId, project.id, options.userId, resolvedProfile.agent?.preferredModel)

    const agentEnv: Record<string, string> = {
      ...resolvedProfile.shell?.envVars,
      PATH: [...(resolvedProfile.shell?.pathAdditions ?? []), '/usr/local/bin', '/usr/bin', '/bin'].join(':'),
      ORCA_PROJECT_ID: project.id,
      ORCA_USER_ID: options.userId,
      ORCA_WORKTREE_PATH: options.worktreePath,
      ORCA_MODEL: account?.model ?? resolvedProfile.agent?.preferredModel ?? '',
      ORCA_TASK_ID: options.taskId ?? '',
    }

    const relay = await this.router.getRelayForProject(options.projectId, options.userId)
    return relay.call('agent.exec', {
      cwd: options.worktreePath,
      prompt: options.prompt,
      env: agentEnv,
      trustPreset: resolvedProfile.agent?.trustPreset ?? 'standard',
      mcpServers: resolvedProfile.mcp?.servers ?? [],
    })
  }
}
```

---

## 7. server-bootstrap.ts — step 8

```typescript
// Sau step 7 (ProfileService):

// 8. ProjectService + ProjectServerRouter
const { ProjectService } = await import('./project/ProjectService')
const { ProjectServerRouter } = await import('./project/ProjectServerRouter')
const projectService = new ProjectService(pool, devServerManager)
const projectRouter = new ProjectServerRouter(projectService, devServerManager, relayConnectionPool)
console.log('[ServerBootstrap] ✅ ProjectService + ProjectServerRouter initialized')
```

---

## 8. Test files cần tạo

```
src/main/project/__tests__/
├── ProjectService.test.ts          (≥ 15 tests)
│   ├── create → auto-adds owner member
│   ├── create with invalid devServerId → DEV_SERVER_NOT_FOUND
│   ├── assertAccess: member OK, non-member → PROJECT_ACCESS_DENIED
│   ├── addMember, removeMember, updateRole
│   └── list: only returns user's projects
├── ProjectServerRouter.test.ts     (≥ 10 tests)
│   ├── getRelayForProject: valid → relay returned
│   ├── getRelayForProject: non-member → PROJECT_ACCESS_DENIED
│   └── getProjectContext: all data populated
├── ProfileAwareAgentSpawner.test.ts (≥ 8 tests)
│   ├── spawn: profile envVars injected
│   ├── spawn: pathAdditions prepended
│   └── spawn: ORCA_PROJECT_ID set
└── project-rpc.test.ts             (≥ 2 tests)
```

**Total: ≥ 35 tests**

---

## 9. Checklist

- [x] `src/shared/project-types.ts`
- [x] `src/main/db/migrations/0007_projects.ts`
- [x] `src/main/db/migrations/index.ts` — add 0007
- [x] `src/main/project/ProjectService.ts`
- [x] `src/main/project/ProjectServerRouter.ts`
- [x] `src/main/project/ProfileAwareAgentSpawner.ts`
- [x] `src/main/runtime/rpc/methods/project.ts`
- [x] `src/main/server-bootstrap.ts` — step 8 + extend interface
- [x] Test files (≥ 35 tests)

## 10. Implementation Notes

| Spec Path | Actual Path | Note |
|-----------|------------|------|
| `src/main/runtime/rpc/methods/project.ts` | `src/main/project/project-rpc-handler.ts` | Co-located với domain |
| Bootstrap step 8 | `server-bootstrap.ts` step 10 | Wired as step 10 (post-profile step 7) |

**Test Results:** 35 pass (ProjectService + ProjectServerRouter + ProfileAwareAgentSpawner + project-rpc)  
**Implemented:** 2026-07-29 ✅
