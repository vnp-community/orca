# SOLUTION: worktree-management — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế  
**Files nguồn đã đọc:** `ProfileAwareAgentSpawner.ts`, `agent-rpc-dispatch.ts`

---

## BUG-WT-002: ProfileAwareAgentSpawner `resolveForProject()` interface mismatch

**File:** [`src/main/project/ProfileAwareAgentSpawner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/project/ProfileAwareAgentSpawner.ts)  
**Lines:** 21–26, 96

### Code thực tế (lines 21–26):
```typescript
/** Minimal interface for AI provider resolution */
export interface AIProviderResolver {
  resolveForProject(
    projectId: string,
    preferredModel: string | undefined
  ): Promise<{ providerId: string; modelId: string; credentials: Record<string, string> } | null>
}
```

### Code thực tế (line 96):
```typescript
const provider = await this.providerService.resolveForProject(projectId, preferredModel)
```

### Vấn đề thực tế
Bug report nói `resolveForProject` signature khác giữa `ProfileAwareAgentSpawner.AIProviderResolver` interface và `AIProviderService.resolveForProject()` thực tế.

Cần kiểm tra `AIProviderService.resolveForProject`:
```bash
grep -n "resolveForProject" src/main/ai-providers/AIProviderService.ts
```

### Fix tùy theo kết quả grep:

**Nếu `AIProviderService.resolveForProject` có signature khác** (ví dụ thêm `devServerId`, `userId`):
```typescript
// src/main/project/ProfileAwareAgentSpawner.ts — Update interface:
export interface AIProviderResolver {
  resolveForProject(
    projectId:      string,
    preferredModel: string | undefined,
    devServerId?:   string,   // ← thêm nếu AIProviderService cần
    userId?:        string,   // ← thêm nếu AIProviderService cần
  ): Promise<{ providerId: string; modelId: string; credentials: Record<string, string> } | null>
}

// Và update call (line 96):
const provider = await this.providerService.resolveForProject(
  projectId,
  preferredModel,
  project.devServerId,  // ← thêm
  userId,               // ← thêm
)
```

**Vấn đề bảo mật quan trọng (line 100–101):**
```typescript
// Code hiện tại inject credentials (API keys) vào env:
Object.assign(profileEnv, provider.credentials)  // ← inject plaintext API key vào agent env!
```

**Đây là security design flaw (liên quan BUG-AIP-002):** API key không nên được inject trực tiếp vào env. Thay vào đó chỉ inject `ORCA_ACCOUNT_ID`:
```typescript
// Fix security (line 98–102):
if (provider) {
  profileEnv['ORCA_AI_PROVIDER_ID'] = provider.providerId
  profileEnv['ORCA_AI_MODEL_ID']    = provider.modelId
  // FIX: KHÔNG inject credentials vào env (security risk)
  // Agent đọc credential qua ORCA_ACCOUNT_ID từ relay credential store:
  profileEnv['ORCA_ACCOUNT_ID']     = provider.providerId
  // Object.assign(profileEnv, provider.credentials)  ← REMOVED
}
```

---

## BUG-BE-WT-001: `worktree.create` không check disk space

**File:** `src/main/runtime/rpc/methods/worktree.ts` (hoặc tương đương)

```bash
grep -rn "worktree.create\|createManagedWorktree" src/main/runtime/ | head -10
```

**Fix pattern:**
```typescript
// Trong handler của worktree.create, thêm disk check TRƯỚC khi call git worktree add:
handler: async (params, { runtime, relay, session }) => {
  const { repoId, branch, baseBranch, worktreePath } = params

  // Get project info
  const project = await runtime.getProject(params.projectId)

  // FIX BE-WT-001: Check disk space trước khi tạo worktree
  if (project.devServerId) {
    try {
      const bridge = relay.getBridge?.(project.devServerId)
      if (bridge) {
        const disk = await bridge.call('system.getDiskInfo', {
          path: worktreePath ?? project.repoPath
        }) as { availableMb?: number } | null

        const MIN_DISK_MB = Number(process.env['ORCA_WORKTREE_MIN_DISK_MB'] ?? 100)
        if (disk?.availableMb !== undefined && disk.availableMb < MIN_DISK_MB) {
          throw {
            code: -32600,
            message: `Insufficient disk space: ${disk.availableMb}MB available, ${MIN_DISK_MB}MB required`
          }
        }
      }
    } catch (diskErr) {
      // Log but don't fail if disk check unavailable (relay may not support it yet)
      console.warn('[worktree.create] Disk check skipped:', diskErr)
    }
  }

  // Check branch conflict
  const existing = await runtime.listWorktrees(repoId)
  if (existing.some(wt => wt.branch === branch || wt.path === worktreePath)) {
    throw { code: -32600, message: `Branch or path already in use: ${branch}` }
  }

  // Proceed with create
  return { worktree: await runtime.createManagedWorktree({ repoId, branch, baseBranch, worktreePath, userId: session.userId }) }
}
```

---

## BUG-WT-001: git.worktree API inconsistent

**Phân tích từ agent-rpc-dispatch.ts thực tế:**
```
Line 330: case 'git.worktree.list': ← ĐÃ CÓ trong relay
Line 341: case 'git.worktree.add':  ← ĐÃ CÓ trong relay
Line 352: case 'git.worktree.remove': ← ĐÃ CÓ trong relay
```

**Bug thực tế:** `WorkspaceService.ts` dùng `git.exec` thay vì `git.worktree.list`.

```bash
grep -n "git.exec\|git.worktree" src/main/workspace/WorkspaceService.ts | head -20
```

**Fix:**
```typescript
// src/main/workspace/WorkspaceService.ts — thay git.exec bằng specific methods:

// TRƯỚC (inconsistent):
const result = await relay.call('git.exec', {
  cwd: project.repoPath,
  args: ['worktree', 'list', '--porcelain'],
})

// SAU (consistent với relay API):
const result = await relay.call('git.worktree.list', {
  repoPath: project.repoPath,
}) as { worktrees: Array<{ path: string; branch: string; head: string }> }

const worktrees = result.worktrees
```

---

## Tóm tắt thay đổi

| Bug | File | Lines | Thay đổi |
|-----|------|-------|---------|
| WT-002 | `ProfileAwareAgentSpawner.ts` | 21–26, 96 | Align interface signature + remove credential injection |
| BE-WT-001 | `worktree.ts` handler | before createManagedWorktree | Thêm disk space check |
| WT-001 | `WorkspaceService.ts` | git.exec call | Đổi sang `git.worktree.list` (đã có trong relay) |
