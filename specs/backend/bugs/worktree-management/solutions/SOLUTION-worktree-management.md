# SOLUTION: Worktree Management Domain — Fix tất cả Bugs

**Domain:** worktree-management  
**TDD Reference:** TDD-07 (Runtime Service), TDD-19 (Project Workspace), TDD-20 (Remote Git UI)  
**Files cần thay đổi:** `src/main/runtime/rpc/methods/worktree.ts`, `src/main/workspace/WorkspaceService.ts`, `src/main/project/ProfileAwareAgentSpawner.ts`  
**Tổng số bugs:** 3 (WT-001, WT-002, BE-WT-001)

---

## Tổng quan phụ thuộc

```
BUG-BE-WT-001 (worktree.create no disk check) — độc lập
BUG-WT-001 (git.worktree API inconsistent) — liên quan relay dispatch
BUG-WT-002 (agent spawner provider interface mismatch) — phụ thuộc AIProviderService
```

**Thứ tự fix:** `BE-WT-001 → WT-001 → WT-002`

---

## BUG-BE-WT-001 — Fix worktree.create không check disk space

**Mức độ:** 🟡 MEDIUM  
**Root cause:** `worktree.ts` gọi thẳng `runtime.createManagedWorktree()` mà không check disk space.

### Fix — Thêm disk space và path conflict validation

```typescript
// src/main/runtime/rpc/methods/worktree.ts

export const worktreeCreate: RpcMethod = {
  name: 'worktree.create',
  handler: async (params, { runtime, relay, session }) => {
    const { repoId, branch, baseBranch, worktreePath } = params as {
      repoId:       string
      branch:       string
      baseBranch:   string
      worktreePath: string
    }

    // 1. Get repo info
    const repo = await runtime.showRepo(repoId)
    if (!repo) throw new RpcError(404, `Repo not found: ${repoId}`)

    // FIX BE-WT-001: Validate disk space (BR-WT-01)
    const devServerId = repo.devServerId
    if (devServerId && relay.getBridge(devServerId)) {
      const bridge    = relay.getBridge(devServerId)!
      const diskInfo  = await bridge.callAsUser('system.getDiskInfo', {
        path: repo.path,
      }, session.userId).catch(() => null) as { availableMb?: number } | null

      if (diskInfo?.availableMb !== undefined && diskInfo.availableMb < 100) {
        throw new RpcError(507, `Insufficient disk space: ${diskInfo.availableMb}MB available, 100MB required`)
      }
    }

    // FIX BE-WT-01: Validate path conflict (BR-WT-03)
    const existingWorktrees = await runtime.listWorktrees(repoId)
    const conflictPath = existingWorktrees.find(wt =>
      wt.path === worktreePath || wt.branch === branch
    )
    if (conflictPath) {
      throw new RpcError(409, `Worktree conflict: path or branch already in use`)
    }

    // 3. Create worktree
    const result = await runtime.createManagedWorktree({
      repoId,
      branch,
      baseBranch,
      worktreePath,
      userId: session.userId,
    })

    return { worktree: result }
  },
}

// Helper để get disk info (cần thêm vào relay agent):
// relay: case 'system.getDiskInfo' → statvfs/df trên remote
```

---

## BUG-WT-001 — Fix git.worktree API inconsistent

**Mức độ:** 🔴 HIGH  
**Root cause:** `WorkspaceService.ts` dùng `git.exec { args: ['worktree', 'list'] }` nhưng relay có dedicated `git.worktree.list` handler.

### Fix — Thống nhất sử dụng specific methods

**Option A (recommended): Cập nhật WorkspaceService dùng specific methods:**

```typescript
// src/main/workspace/WorkspaceService.ts

// TRƯỚC (inconsistent):
const result = await relay.call('git.exec', {
  cwd: project.repoPath,
  args: ['worktree', 'list', '--porcelain'],
})
const worktrees = parseWorktreePorcelain(result.stdout)

// SAU — dùng dedicated method:
const result = await relay.callAsUser('git.worktree.list', {
  repoPath: project.repoPath,
}, userId) as { worktrees: WorktreeInfo[] }
const worktrees = result.worktrees  // Already parsed by relay handler

// Cho git.worktree.add:
// TRƯỚC:
await relay.call('git.exec', { cwd, args: ['worktree', 'add', path, branch] })

// SAU:
await relay.callAsUser('git.worktree.add', {
  repoPath:     project.repoPath,
  worktreePath: worktreePath,
  branch:       branch,
  createBranch: true,  // Tạo branch mới nếu chưa tồn tại
}, userId)
```

**Thêm handler vào relay nếu chưa có:**

```typescript
// src/relay/agent-rpc-dispatch.ts

// git.worktree.list — nếu chưa có:
case 'git.worktree.list': {
  const repoPath = typeof rpc.params?.repoPath === 'string'
    ? rpc.params.repoPath
    : config.workDir

  const result = await runCommandCapture('git', ['worktree', 'list', '--porcelain'], {
    cwd:     repoPath,
    timeout: 10_000,
  })
  const worktrees = parseWorktreePorcelain(result.stdout)
  return makeOk(rpc.id, { worktrees })
}

// git.worktree.add — nếu chưa có:
case 'git.worktree.add': {
  const { repoPath, worktreePath, branch, createBranch } = rpc.params ?? {}

  if (!worktreePath || !branch) {
    return makeError(rpc.id, AgentErrorCode.InvalidParams, 'Missing worktreePath or branch')
  }

  const args = createBranch
    ? ['worktree', 'add', '-b', branch as string, worktreePath as string]
    : ['worktree', 'add', worktreePath as string, branch as string]

  const result = await runCommandCapture('git', args, {
    cwd:     (repoPath as string) ?? config.workDir,
    timeout: 30_000,
  })

  return makeOk(rpc.id, {
    worktreePath,
    branch,
    exitCode: result.exitCode,
    stdout:   result.stdout,
    stderr:   result.stderr,
  })
}
```

---

## BUG-WT-002 — Fix ProfileAwareAgentSpawner provider interface mismatch

**Mức độ:** 🔴 HIGH  
**Root cause:** `ProfileAwareAgentSpawner` gọi `providerService.resolveForProject(projectId, preferredModel)` nhưng `AIProviderService.resolveForProject()` signature là `(devServerId, projectId, userId, modelHint)`.

### Fix — Align interface với implementation

```typescript
// src/main/project/ProfileAwareAgentSpawner.ts

export class ProfileAwareAgentSpawner {
  constructor(
    private readonly relay:           DevServerRelayBridge,
    private readonly router:          ProjectRouter,
    private readonly providerService: AIProviderService,  // đổi từ AIProviderResolver
    private readonly profileResolver: ProfileResolver,
    private readonly log:             Logger,
  ) {}

  async spawn(params: SpawnParams): Promise<SpawnResult> {
    const { projectId, userId, model: preferredModel } = params

    // FIX WT-002: Get devServerId trước khi gọi resolveForProject
    const project = await this.router.getProject(projectId)
    if (!project) throw new Error(`Project not found: ${projectId}`)

    // FIX: Correct signature with all required params
    const provider = await this.providerService.resolveForProject(
      project.devServerId,  // ← Thiếu trong original code
      projectId,
      userId,               // ← Thiếu trong original code
      preferredModel,
    )

    if (!provider) {
      throw new Error(`No AI provider available for project ${projectId}`)
    }

    // Build env (FIX: không inject plaintext API key directly)
    const profile = await this.profileResolver.resolve({ userId, projectId })
    
    const spawnEnv: Record<string, string> = {
      ...profile.shell.envOverrides,
      ORCA_PROJECT_ID:  projectId,
      ORCA_TASK_ID:     params.taskId,
      ORCA_USER_ID:     userId,
      // FIX WT-002 + AIP-002: KHÔNG inject plaintext credentials
      // Thay vào đó, Dev Server sẽ tự đọc credential từ ~/.orca/credentials/
      ORCA_ACCOUNT_ID:  provider.id,  // Agent dùng accountId để load credential
    }

    // spawn qua relay
    return await this.relay.callAsUser('agent.spawn', {
      model:       preferredModel ?? provider.modelId,
      accountId:   provider.id,           // ← Dev Server tự decrypt credential
      cwd:         params.worktreePath ?? project.defaultWorktreePath,
      taskId:      params.taskId,
      userId,
      projectId,
      env:         spawnEnv,
    }, userId) as SpawnResult
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/runtime/rpc/methods/worktree.ts` | Add disk space + path conflict checks | BE-WT-001 |
| `src/main/workspace/WorkspaceService.ts` | Use git.worktree.list/add instead of git.exec | WT-001 |
| `src/relay/agent-rpc-dispatch.ts` | Ensure git.worktree.list + git.worktree.add handlers exist | WT-001 |
| `src/main/project/ProfileAwareAgentSpawner.ts` | Fix resolveForProject signature + remove credential leak | WT-002 |

---

## Verification Plan

```bash
# Test BE-WT-001:
# 1. Create worktree when disk < 100MB → expect 507 error
# 2. Create worktree with existing branch → expect 409 conflict
# 3. Create worktree OK → verify git worktree add called

# Test WT-001:
# 1. WorkspaceService.listWorktrees → verify uses git.worktree.list (not git.exec)
# 2. WorkspaceService.createWorktree → verify uses git.worktree.add

# Test WT-002:
# 1. ProfileAwareAgentSpawner.spawn → verify resolveForProject called with devServerId + userId
# 2. Verify env does NOT contain ANTHROPIC_API_KEY or similar (only ORCA_ACCOUNT_ID)

pnpm vitest run src/main/runtime/__tests__/worktree.test.ts
pnpm vitest run src/main/project/__tests__/profile-aware-agent-spawner.test.ts
```
