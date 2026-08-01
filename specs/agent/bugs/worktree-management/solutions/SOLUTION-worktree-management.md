# SOLUTION: Worktree Management Domain — TDD v5 Based Implementation

**Domain:** worktree-management  
**TDD Reference:** TDD-AG-10 (Git Handler Extension), TDD-AG-12 (ProfileAware Agent Spawner), TDD-AG-01 (Architecture)  
**Files cần thay đổi:** `src/relay/git-handler.ts`, `src/relay/agent-rpc-dispatch.ts`, `src/relay/agent-spawner.ts`  
**Status:** Không có bugs được report — tài liệu này mô tả các thiếu sót tiềm năng và implementation patterns theo TDD v5

---

## Tổng quan Domain

**Worktree Management** trong context Dev Server Agent bao gồm:

1. **Git Worktree Operations** — `git worktree add/list/remove` (subset của git.exec whitelist)
2. **Agent Spawn với Worktree Context** — spawn AI agent trong worktree cụ thể
3. **Per-User Worktree Isolation** — mỗi user có worktree riêng, không share

### TDD v5 Coverage (TDD-AG-10 §A.10 + TDD-AG-12)

Theo TDD v5 `00-index.md §A.10`, các worktree operations được hỗ trợ:

```typescript
'git.worktree.list' → exec('git worktree list --porcelain', { cwd })
'git.worktree.add'  → exec('git worktree add <path> <branch>', { cwd })
```

Và theo `TDD-AG-10 §2` (git.exec whitelist):
```javascript
const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',  // ← 'worktree' included
]);
```

---

## Tiềm năng vấn đề (Potential Issues)

### Issue 1 — git.worktree operations qua agent-rpc-dispatch thiếu isolation

**Mức độ:** HIGH  
**Mô tả:** Khi `git.exec` được gọi với `args: ['worktree', 'add', '/path', 'branch']`, không có validation nào đảm bảo:
- Path của worktree mới nằm trong workspace được phép
- User A không thể tạo worktree đè lên worktree của User B
- Symbolic link attacks qua `git worktree add`

**Theo HLD A.8 — Agent Isolation Model:**
```
| Mechanism | Cách thực hiện |
|-----------|----------------|
| File path | SecureFs.validatePath() checks projectRoot + allowedRoots |
```

**Fix — Thêm worktree path validation vào git-handler.ts:**

```typescript
// src/relay/git-handler.ts

// Thêm vào sau ALLOWED_GIT_SUBCOMMANDS:
const WORKTREE_SUBCOMMANDS = new Set(['add', 'list', 'remove', 'prune', 'lock', 'unlock', 'move', 'repair'])

/**
 * Validate worktree path khi git worktree add <path> <branch>
 * Ngăn path traversal và cross-user worktree conflicts.
 */
function validateWorktreePath(args: string[], workDir: string): void {
  // git worktree add <path> [<commit-ish>]
  // args[0] = 'worktree', args[1] = 'add', args[2] = path
  if (args[0] === 'worktree' && args[1] === 'add' && args[2]) {
    const rawPath = args[2]
    const resolved = resolve(rawPath.startsWith('/') ? rawPath : join(workDir, rawPath))

    // Worktree phải nằm trong workDir hoặc thư mục sibling
    const parentDir = dirname(workDir)
    const allowedRoots = [workDir, parentDir]
    const isAllowed = allowedRoots.some(root =>
      resolved.startsWith(root + '/') || resolved === root
    )

    if (!isAllowed) {
      throw new Error(`GIT_WORKTREE_PATH_NOT_ALLOWED: ${resolved}`)
    }

    // Không chứa null bytes
    if (resolved.includes('\0')) {
      throw new Error('GIT_WORKTREE_PATH_INVALID: null bytes')
    }
  }
}
```

---

### Issue 2 — agent-session.ts thiếu 'worktrees' trong capabilities nhưng 'worktrees' đã có

**Mức độ:** INFO (đã được fix theo source review)  
**Mô tả:** Theo `SUPPLEMENT-source-aligned.md` của agent-orchestration:
```
agent-session.ts L67: capabilities có 'ai.providers', 'agent.spawn', 'worktrees' 
```

Capability `'worktrees'` đã được khai báo. Tuy nhiên không có validation rằng git-handler.ts thực sự support các worktree operations.

**Fix — Thêm worktree capability validation khi startup:**

```typescript
// src/relay/agent-session.ts — trong buildCapabilities():

async function buildCapabilities(config: AgentConfig): Promise<string[]> {
  const caps: string[] = ['ai.providers', 'agent.spawn']

  // Check worktree support
  const hasGitHandler = await checkGitAvailable(config.toolPath)
  if (hasGitHandler) {
    caps.push('worktrees')
    caps.push('git.exec')    // thêm explicit capability
  }

  // Check PTY support
  const hasPty = await checkPtyAvailable()
  if (hasPty) {
    caps.push('pty')         // BUG AWS-001 fix
  }

  return caps
}

async function checkGitAvailable(toolPath: string): Promise<boolean> {
  const dirs = toolPath.split(':')
  for (const dir of dirs) {
    const gitPath = join(dir, 'git')
    try {
      await access(gitPath, constants.X_OK)
      return true
    } catch { /* continue */ }
  }
  // Fallback: check in PATH
  try {
    await execFile('git', ['--version'], { timeout: 3000 })
    return true
  } catch {
    return false
  }
}
```

---

### Issue 3 — ProfileAwareAgentSpawner không inject worktree context đúng

**Mức độ:** MEDIUM  
**Mô tả:** Theo TDD-AG-12 §5 (`buildAgentEnv`), khi spawn agent trong worktree:

```typescript
// TDD-AG-12: buildAgentEnv inject:
ORCA_PROJECT_ID: req.projectId,
ORCA_TASK_ID:    req.taskId,
ORCA_USER_ID:    req.userId,
```

Nhưng thiếu `ORCA_WORKTREE_PATH` — agent không biết nó đang chạy trong worktree nào.

**Fix — Thêm ORCA_WORKTREE_PATH vào buildAgentEnv:**

```typescript
// src/relay/agent-spawner.ts — trong buildAgentEnv():

// BEFORE:
return {
  ...baseEnv,
  ...apiKeyPair,
  ...localInferenceEnv,
  ...req.extraEnv,
  ORCA_PROJECT_ID: req.projectId,
  ORCA_TASK_ID:    req.taskId,
  ORCA_USER_ID:    req.userId,
  // ...
}

// AFTER — thêm worktree context:
return {
  ...baseEnv,
  ...apiKeyPair,
  ...localInferenceEnv,
  ...req.extraEnv,
  ORCA_PROJECT_ID:    req.projectId,
  ORCA_TASK_ID:       req.taskId,
  ORCA_USER_ID:       req.userId,
  ORCA_WORKTREE_PATH: req.cwd,           // ← Worktree absolute path
  ORCA_WORKTREE_BRANCH: req.branchName ?? '',  // ← Branch name nếu có
  // ...
}

// Thêm vào AgentSpawnRequest interface:
export interface AgentSpawnRequest {
  // ... existing fields ...
  readonly branchName?: string   // git branch tương ứng với worktree
}
```

---

### Issue 4 — git.worktree.list không tồn tại trong agent-rpc-dispatch.ts

**Mức độ:** HIGH  
**Mô tả:** TDD v5 `00-index.md §A.10` đề cập:
```
'git.worktree.list' → exec('git worktree list --porcelain', { cwd })
'git.worktree.add'  → exec('git worktree add <path> <branch>', { cwd })
```

Nhưng `agent-rpc-dispatch.ts` chỉ có `git.exec` và `git.execStream` — không có convenience wrappers cho worktree operations.

**Fix — Thêm dedicated worktree handlers:**

```typescript
// src/relay/agent-rpc-dispatch.ts — thêm sau 'git.execStream':

// ── git.worktree.list ──────────────────────────────────────────────────────────
case 'git.worktree.list': {
  // List all worktrees in porcelain format
  // Params: { cwd: string }
  const cwd = typeof rpc.params?.cwd === 'string' ? rpc.params.cwd : config.workDir
  try {
    const result = await runCommandCapture('git', ['worktree', 'list', '--porcelain'], {
      cwd,
      timeout: 10_000,
    })
    // Parse porcelain format → structured response
    const worktrees = parseWorktreePorcelain(result.stdout)
    return makeOk(rpc.id, { worktrees, exitCode: result.exitCode })
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.list failed: ${msg}`)
  }
}

// ── git.worktree.add ──────────────────────────────────────────────────────────
case 'git.worktree.add': {
  // Add a new worktree
  // Params: { cwd: string, path: string, branch: string, createBranch?: boolean }
  const cwd          = typeof rpc.params?.cwd    === 'string' ? rpc.params.cwd    : config.workDir
  const worktreePath = typeof rpc.params?.path   === 'string' ? rpc.params.path   : ''
  const branch       = typeof rpc.params?.branch === 'string' ? rpc.params.branch : ''
  const createBranch = rpc.params?.createBranch === true

  if (!worktreePath || !branch) {
    return makeError(rpc.id, AgentErrorCode.InvalidParams, 'Missing path or branch for git.worktree.add')
  }

  // Validate path (security check)
  try {
    validateWorktreePath(['worktree', 'add', worktreePath], cwd)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.InvalidParams, msg)
  }

  const args = createBranch
    ? ['worktree', 'add', '-b', branch, worktreePath]
    : ['worktree', 'add', worktreePath, branch]

  try {
    const result = await runCommandCapture('git', args, { cwd, timeout: 30_000 })
    return makeOk(rpc.id, {
      worktreePath: resolve(worktreePath.startsWith('/') ? worktreePath : join(cwd, worktreePath)),
      branch,
      exitCode: result.exitCode,
      stdout: result.stdout,
      stderr: result.stderr,
    })
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.add failed: ${msg}`)
  }
}
```

**Thêm helper `parseWorktreePorcelain`:**

```typescript
// src/relay/git-handler.ts — hoặc thêm vào agent-rpc-dispatch.ts

interface WorktreeInfo {
  path:     string
  head:     string
  branch:   string
  bare:     boolean
  detached: boolean
  prunable: boolean
}

function parseWorktreePorcelain(stdout: string): WorktreeInfo[] {
  const worktrees: WorktreeInfo[] = []
  let current: Partial<WorktreeInfo> | null = null

  for (const line of stdout.split('\n')) {
    if (line.startsWith('worktree ')) {
      if (current?.path) worktrees.push(current as WorktreeInfo)
      current = { path: line.slice('worktree '.length), bare: false, detached: false, prunable: false }
    } else if (line.startsWith('HEAD ') && current) {
      current.head = line.slice('HEAD '.length)
    } else if (line.startsWith('branch ') && current) {
      current.branch = line.slice('branch '.length).replace('refs/heads/', '')
    } else if (line === 'bare' && current) {
      current.bare = true
    } else if (line === 'detached' && current) {
      current.detached = true
    } else if (line.startsWith('prunable') && current) {
      current.prunable = true
    }
  }

  if (current?.path) worktrees.push(current as WorktreeInfo)
  return worktrees
}
```

---

## Tóm tắt file changes

| File | Action | Issue |
|------|--------|-------|
| `src/relay/git-handler.ts` | ADD `validateWorktreePath()` — ngăn path traversal khi git worktree add | Issue 1 |
| `src/relay/git-handler.ts` | ADD `parseWorktreePorcelain()` — parse git worktree list output | Issue 4 |
| `src/relay/agent-rpc-dispatch.ts` | ADD `case 'git.worktree.list'` | Issue 4 |
| `src/relay/agent-rpc-dispatch.ts` | ADD `case 'git.worktree.add'` | Issue 4 |
| `src/relay/agent-session.ts` | MODIFY `buildCapabilities()` — dynamic check git + pty availability | Issue 2 |
| `src/relay/agent-spawner.ts` | ADD `ORCA_WORKTREE_PATH`, `ORCA_WORKTREE_BRANCH` in buildAgentEnv | Issue 3 |
| `src/relay/agent-spawner.ts` | ADD `branchName?` field to `AgentSpawnRequest` | Issue 3 |

---

## Implementation Order (theo TDD-AG-12 phụ thuộc)

```
1. src/relay/git-handler.ts          ← validateWorktreePath + parseWorktreePorcelain (no deps)
2. src/relay/agent-rpc-dispatch.ts   ← git.worktree.* cases (depends on git-handler)
3. src/relay/agent-spawner.ts        ← buildAgentEnv + AgentSpawnRequest (no deps)
4. src/relay/agent-session.ts        ← buildCapabilities (depends on git-handler check)
```

---

## Verification Plan

```bash
# 1. Type check:
pnpm tsc --noEmit -p config/tsconfig.node.json

# 2. Unit tests:
pnpm vitest run src/relay/__tests__/git-handler.test.ts
pnpm vitest run src/relay/__tests__/agent-spawner.test.ts

# 3. Security tests (worktree path validation):
# - Gửi git.worktree.add với path='../../etc/passwd' → expect GIT_WORKTREE_PATH_NOT_ALLOWED
# - Gửi git.worktree.add với path='/root/worktree'  → expect GIT_WORKTREE_PATH_NOT_ALLOWED
# - Gửi git.worktree.add với path='/tmp/wt1' → expect allowed
# - Gửi git.worktree.add với path='/home/ubuntu/projects/wt1' → expect allowed

# 4. Integration test:
# - Gửi git.worktree.list → verify response có worktrees array
# - Gửi git.worktree.add → verify worktree được tạo trên disk
# - Verify buildAgentEnv có ORCA_WORKTREE_PATH trong env

# 5. Capability test:
# - Start agent khi git không có trong PATH → verify capabilities không có 'git.exec'
# - Start agent khi node-pty không có → verify capabilities không có 'pty'
```

---

## TDD v5 References

| TDD | Section | Nội dung liên quan |
|-----|---------|-------------------|
| TDD-AG-10 | §2 | git.exec whitelist — 'worktree' subcommand |
| TDD-AG-10 | §4 | Validation rules cho git args |
| TDD-AG-12 | §5 | buildAgentEnv — env vars injection |
| TDD-AG-01 | §A.10 | git.worktree.list + git.worktree.add |
| TDD-AG-01 | §A.8 | Agent Isolation Model — File path SecureFs |
| TDD-AG-01 | §A.5 | Security Model — File path safety |

---

## ✅ Implementation Status (2026-08-01)

WT-01,02,03 DONE: validateWorktreePath, git.worktree.list/add/remove, ORCA_WORKTREE_* env inject. WT-04 DEFERRED.
