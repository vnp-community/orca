# TDD-15: Project-Dev Server Binding

**Document:** TDD-15 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Project Management — binding project to dev server, member access, agent routing
**Feature:** F34, F38 (partial)
**ADR:** ADR-007, ADR-011
**HLD Ref:** C3.10b, C4.8
**Source files (to create):**
- `src/main/project/ProjectService.ts`
- `src/main/project/ProjectServerRouter.ts`
- `src/main/project/ProfileAwareAgentSpawner.ts`
- `src/main/runtime/rpc/methods/project.ts`
- `src/main/db/migrations/0007_projects.ts`

> **Status: ❌ TODO** — v5.0 proposed; migration 0007 prerequisite

---

## 1. Mục tiêu

Mỗi **project** gắn bắt buộc với một **dev server**. Khi user làm việc trong project, mọi operations (agent spawn, worktree, terminal, git) tự động chạy trên đúng server đó — không cần user chọn thủ công.

---

## 2. Data Types

```typescript
// src/shared/project-types.ts

export interface OrcaProject {
  id: string
  name: string
  description?: string
  devServerId: string          // REQUIRED — bound dev server
  repoPath: string             // absolute path on dev server
  defaultBranch: string        // default: 'main'
  visibility: 'private' | 'team' | 'company'
  createdBy: string            // userId
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

## 3. ProjectService

```typescript
// src/main/project/ProjectService.ts

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
  }): Promise<OrcaProject> {
    // Validate: devServerId must exist
    const server = await this.devServerManager.getServer(params.devServerId)
    if (!server) throw new Error('DEV_SERVER_NOT_FOUND')

    const id = randomUUID()
    await this.pool.query(`
      INSERT INTO orca_projects (id, name, dev_server_id, repo_path, default_branch, visibility, created_by)
      VALUES (?, ?, ?, ?, ?, ?, ?)
    `, [id, params.name, params.devServerId, params.repoPath,
        params.defaultBranch ?? 'main', params.visibility ?? 'team', params.createdBy])

    // Auto-add creator as owner
    await this.addMember(id, params.createdBy, 'owner')
    return this.get(id)!
  }

  async get(projectId: string): Promise<OrcaProject | null>
  async list(userId: string): Promise<OrcaProject[]>  // projects where user is member
  async update(projectId: string, patch: Partial<OrcaProject>, updatedBy: string): Promise<void>
  async delete(projectId: string, deletedBy: string): Promise<void>

  async addMember(projectId: string, userId: string, role: ProjectMember['role']): Promise<void>
  async removeMember(projectId: string, userId: string): Promise<void>
  async updateMemberRole(projectId: string, userId: string, role: ProjectMember['role']): Promise<void>
  async getMembers(projectId: string): Promise<ProjectMember[]>
  async getMember(projectId: string, userId: string): Promise<ProjectMember | null>

  /** Validate user has at least 'viewer' access */
  async assertAccess(projectId: string, userId: string): Promise<ProjectMember>
}
```

---

## 4. ProjectServerRouter

```typescript
// src/main/project/ProjectServerRouter.ts
// Resolves which dev server to use for a given project operation

export class ProjectServerRouter {
  constructor(
    private readonly projectService: ProjectService,
    private readonly devServerManager: DevServerManager,
    private readonly relayPool: RelayConnectionPool
  ) {}

  async getRelayForProject(
    projectId: string,
    userId: string
  ): Promise<DevServerRelayBridge> {
    const member = await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)
    if (!project) throw new Error('PROJECT_NOT_FOUND')

    const server = await this.devServerManager.getServer(project.devServerId)
    if (!server) throw new Error('DEV_SERVER_NOT_FOUND')

    return this.relayPool.getOrConnect(project.devServerId, server)
  }

  /** Get the full project context for workspace initialization */
  async getProjectContext(
    projectId: string,
    userId: string,
    profileResolver: ProfileResolver
  ): Promise<ProjectContext> {
    const member = await this.projectService.assertAccess(projectId, userId)
    const project = await this.projectService.get(projectId)!
    const devServer = await this.devServerManager.getServer(project.devServerId)!
    const resolvedProfile = await profileResolver.resolve(userId)
    return { project, member, devServer, resolvedProfile }
  }
}
```

---

## 5. ProfileAwareAgentSpawner

```typescript
// src/main/project/ProfileAwareAgentSpawner.ts
// Spawns AI agent on project dev server with resolved profile injected

export interface AgentSpawnOptions {
  projectId: string
  userId: string
  worktreePath: string       // absolute path on dev server
  prompt: string
  taskId?: string            // link to task (optional)
  accountId?: string         // override AI provider account
}

export class ProfileAwareAgentSpawner {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly profileResolver: ProfileResolver,
    private readonly providerResolver: AIProviderResolver
  ) {}

  async spawn(options: AgentSpawnOptions): Promise<AgentSession> {
    const context = await this.router.getProjectContext(
      options.projectId, options.userId, this.profileResolver
    )
    const { project, resolvedProfile, devServer } = context

    // Resolve AI provider
    const account = options.accountId
      ? await this.providerResolver.getById(options.accountId)
      : await this.providerResolver.resolve({
          devServerId: project.devServerId,
          projectId: project.id,
          userId: options.userId,
          modelHint: resolvedProfile.agent?.preferredModel,
        })

    // Build env for agent
    const agentEnv: Record<string, string> = {
      // Profile-injected env vars
      ...resolvedProfile.shell?.envVars,
      // PATH additions
      PATH: [
        ...(resolvedProfile.shell?.pathAdditions ?? []),
        '/usr/local/bin', '/usr/bin', '/bin',
      ].join(':'),
      // Orca context
      ORCA_PROJECT_ID: project.id,
      ORCA_USER_ID: options.userId,
      ORCA_WORKTREE_PATH: options.worktreePath,
      ORCA_MODEL: account?.model ?? resolvedProfile.agent?.preferredModel ?? '',
      ORCA_TASK_ID: options.taskId ?? '',
    }

    // Read AI credential from dev server (relay-only, server never sees key)
    const credentialEnv = await this.getCredentialEnvFromRelay(
      project.devServerId, account!
    )

    const relay = await this.router.getRelayForProject(options.projectId, options.userId)

    return relay.call('agent.exec', {
      cwd: options.worktreePath,
      prompt: options.prompt,
      env: { ...agentEnv, ...credentialEnv },
      trustPreset: resolvedProfile.agent?.trustPreset ?? 'standard',
      mcpServers: resolvedProfile.mcp?.servers ?? [],
    })
  }

  private async getCredentialEnvFromRelay(
    devServerId: string,
    account: AIProviderAccount
  ): Promise<Record<string, string>> {
    const relay = await this.router.getRelayForProject(/* ... */)
    const credData = await relay.call('ai.provider.readCredential', {
      accountId: account.id
    }) as { envKey: string; value: string }[]
    return Object.fromEntries(credData.map(c => [c.envKey, c.value]))
  }
}
```

---

## 6. RPC Methods (`src/main/runtime/rpc/methods/project.ts`)

```typescript
// namespace: 'project'

'project.create'          // (member+) → OrcaProject
'project.get'             // (member) → OrcaProject
'project.list'            // (any auth) → OrcaProject[] — filtered to user's projects
'project.update'          // (owner/admin) → void
'project.delete'          // (owner/admin) → void
'project.addMember'       // (owner/admin) → void
'project.removeMember'    // (owner/admin) → void
'project.updateMemberRole'// (owner/admin) → void
'project.getMembers'      // (member) → ProjectMember[]
'project.getContext'      // (member) → ProjectContext — initialize workspace
'project.spawnAgent'      // (member with execute) → AgentSession
```

---

## 7. Error Handling

| Scenario | Error code |
|---------|-----------|
| Project not found | `PROJECT_NOT_FOUND` — 404 |
| User not member | `PROJECT_ACCESS_DENIED` — 403 |
| Dev server not found | `DEV_SERVER_NOT_FOUND` — 404 |
| Dev server unreachable | `DEV_SERVER_UNREACHABLE` — 503 |
| Viewer cannot spawn agent | `INSUFFICIENT_PROJECT_ROLE` — 403 |
| Delete project with running workflows | `PROJECT_HAS_ACTIVE_WORKFLOWS` — 409 |

---

## 8. Test Coverage

```
src/main/project/__tests__/
├── ProjectService.test.ts
│   ├── create → auto-add owner member
│   ├── create with invalid devServerId → DEV_SERVER_NOT_FOUND
│   ├── assertAccess: member OK, non-member 403
│   ├── addMember, removeMember, updateRole
│   └── list: only returns user's projects
├── ProjectServerRouter.test.ts
│   ├── getRelayForProject: valid project + member → relay
│   ├── getRelayForProject: non-member → PROJECT_ACCESS_DENIED
│   └── getProjectContext: returns merged context
├── ProfileAwareAgentSpawner.test.ts
│   ├── spawn: profile envVars injected into agent env
│   ├── spawn: pathAdditions prepended to PATH
│   └── spawn: ORCA_PROJECT_ID set correctly
└── project-rpc.test.ts
    ├── project.create (auth OK)
    ├── project.get (member OK, non-member 403)
    └── project.spawnAgent (member with execute role)
```

**Target:** ≥ 35 tests
