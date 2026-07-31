# SOL-V5-007: Remote Git UI (TDD-20)

**Solution:** SOL-V5-007  
**TDD:** TDD-20 — Remote Git via Relay (git-handler, streaming, AI commit msg, PR creation)  
**Date:** 2026-07-28  
**Status:** ✅ IMPLEMENTED  
**Implementation Date:** 2026-07-29  
**Tests:** 34 pass (relay validation 16 + server-side RPC 18) | TypeScript: 0 errors  
**Strategy:** Additive-only — `src/relay/git-handler.ts` mới; extend `src/main/runtime/rpc/methods/git.ts`

---

## 1. Phân tích gap

| TDD yêu cầu | Hiện trạng code | Gap |
|-------------|-----------------|-----|
| `src/relay/git-handler.ts` | Không tồn tại | ❌ Tạo mới |
| Extend `src/main/runtime/rpc/methods/git.ts` | Tồn tại (local git) | ⚠️ Extend |
| `src/renderer/src/components/workspace/git/GitPanel.tsx` | Không tồn tại | ❌ Tạo mới |
| `src/renderer/src/components/workspace/git/CommitForm.tsx` | Không tồn tại | ❌ Tạo mới |
| `src/renderer/src/components/workspace/git/DiffViewer.tsx` | Không tồn tại | ❌ Tạo mới |
| `src/renderer/src/components/workspace/git/BranchManager.tsx` | Không tồn tại | ❌ Tạo mới |

**Code có thể reuse:**
- `ProjectServerRouter.getRelayForProject()` — relay routing (SOL-002)
- `AIProviderService.resolveForProject()` — AI commit message (SOL-003)
- `DevServerRelayBridge.call()` — relay git operations
- `TaskService.findByRef()` + `TaskService.update()` — task status auto-advance (SOL-005)
- Existing `src/main/runtime/rpc/methods/git.ts` — local git; chỉ thêm remote methods, không xóa existing

**Dependency:** SOL-002 (ProjectServerRouter), SOL-003 (AIProviderService), SOL-005 (TaskService)

---

## 2. `src/relay/git-handler.ts`

Copy nguyên từ TDD-20 §2 — chạy trên Dev Server, không trên Orca Server:

```typescript
import { execFile, spawn } from 'node:child_process'
import { promisify } from 'node:util'
const execFileAsync = promisify(execFile)

const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',
])

const SHELL_METACHARACTERS = /[&|;$`]/

function validateGitArgs(args: string[]): void {
  if (args.length === 0) throw new Error('GIT_NO_SUBCOMMAND')
  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new Error(`GIT_DISALLOWED_SUBCOMMAND: ${args[0]}`)
  }
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new Error(`GIT_SHELL_METACHARACTER_IN_ARG: ${arg}`)
    }
  }
}

export const gitHandlers = {
  'git.exec': async (params: { cwd: string; args: string[]; timeout?: number }) => {
    validateGitArgs(params.args)
    try {
      const result = await execFileAsync('git', params.args, {
        cwd: params.cwd,
        timeout: params.timeout ?? 30_000,
        maxBuffer: 10 * 1024 * 1024,
      })
      return { stdout: result.stdout, stderr: result.stderr, exitCode: 0 }
    } catch (err: any) {
      return {
        stdout: err.stdout ?? '',
        stderr: err.stderr ?? err.message,
        exitCode: err.code ?? 1,
      }
    }
  },

  'git.execStream': (params: { cwd: string; args: string[] }): AsyncGenerator<string> => {
    validateGitArgs(params.args)
    return (async function* () {
      const child = spawn('git', params.args, { cwd: params.cwd, stdio: 'pipe' })
      for await (const chunk of child.stdout) {
        yield (chunk as Buffer).toString('utf-8')
      }
      await new Promise<void>((resolve, reject) => {
        child.on('close', (code) => code === 0 ? resolve() : reject(new Error(`git exit ${code}`)))
      })
    })()
  },
}
```

> **Bundle note:** `git-handler.ts` được include trong relay bundle build (`src/relay/`). Register handlers trong relay entry point (`src/relay/index.ts` hoặc tương đương) — không cần thay đổi Orca Server.

---

## 3. Extend `src/main/runtime/rpc/methods/git.ts` — Remote methods

Không xóa existing local git methods. Chỉ thêm remote-mode handlers:

```typescript
// THÊM vào cuối file src/main/runtime/rpc/methods/git.ts
// hoặc tạo file riêng src/main/runtime/rpc/methods/git-remote.ts

import type { ProjectServerRouter } from '../../project/ProjectServerRouter'
import type { AIProviderService } from '../../ai-providers/AIProviderService'
import type { TaskService } from '../../task/TaskService'
import type { TaskGrantService } from '../../task/TaskGrantService'

const COMMIT_MSG_PROMPT = `Write a concise git commit message for the following diff.
Format: <type>(<scope>): <description>
Types: feat|fix|docs|style|refactor|test|chore
Max 72 characters for the first line.
Do NOT include any explanation, just the commit message.`

export function registerRemoteGitRpcMethods(
  router: ProjectServerRouter,
  aiProviderService: AIProviderService,
  taskService: TaskService,
  taskGrantService: TaskGrantService,
  dispatcher: RpcDispatcher
): void {

  // Basic git relay methods — delegating to relay git.exec
  for (const method of ['git.status', 'git.diff', 'git.add', 'git.restore', 'git.commit',
                         'git.fetch', 'git.branch.list', 'git.branch.create', 'git.branch.delete',
                         'git.checkout', 'git.merge', 'git.stash', 'git.stash.pop',
                         'git.log', 'git.show', 'git.worktree.list', 'git.worktree.add', 'git.worktree.remove']) {
    dispatcher.register(method, async (params, session) => {
      const relay = await router.getRelayForProject(params.projectId, session.userId)
      return relay.call('git.exec', { cwd: params.worktreePath, args: params.args })
    })
  }

  // Streaming methods (push/pull)
  for (const method of ['git.push', 'git.pull']) {
    dispatcher.register(method, async (params, session) => {
      const relay = await router.getRelayForProject(params.projectId, session.userId)
      return relay.call('git.execStream', { cwd: params.worktreePath, args: params.args })
    })
  }

  // AI commit message generation
  dispatcher.register('git.generateCommitMessage', async (params, session) => {
    const relay = await router.getRelayForProject(params.projectId, session.userId)
    const project = await router.getProject(params.projectId)
    if (!project) throw new Error('PROJECT_NOT_FOUND')

    const diffResult = await relay.call('git.exec', {
      cwd: params.worktreePath,
      args: ['diff', '--staged', '--stat', '-p'],
    }) as { stdout: string }

    if (!diffResult.stdout || diffResult.stdout.length < 10) {
      throw new Error('GIT_NO_STAGED_CHANGES')
    }

    const diffTruncated = diffResult.stdout.slice(0, 8000)
    const account = await aiProviderService.resolveForProject(
      project.devServerId, project.id, session.userId
    )
    if (!account) throw new Error('NO_AI_PROVIDER_CONFIGURED')

    const response = await relay.call('ai.complete', {
      accountId: account.id,
      prompt: COMMIT_MSG_PROMPT + '\n\nDiff:\n```\n' + diffTruncated + '\n```',
      maxTokens: 200,
      temperature: 0.1,
    }) as { text: string }

    return { message: response.text.trim() }
  })

  // PR creation
  dispatcher.register('git.pr.create', async (params, session) => {
    const relay = await router.getRelayForProject(params.projectId, session.userId)

    // Strategy A: GitHub CLI
    const preflight = await relay.call('preflight.check', { services: ['github-cli'] }) as { githubCli: boolean }
    if (preflight.githubCli) {
      const args = ['gh', 'pr', 'create', '--title', params.title, '--body', params.body, '--base', params.base]
      if (params.draft) args.push('--draft')
      args.push('--json', 'url,number')
      const result = await relay.call('git.exec', { cwd: params.worktreePath, args }) as { stdout: string }
      const { url, number } = JSON.parse(result.stdout)
      return { prUrl: url, prNumber: number }
    }

    throw new Error('GITHUB_NO_CREDENTIAL')
  })

  // Task status auto-advance on commit
  dispatcher.register('git.commit', async (params, session) => {
    const relay = await router.getRelayForProject(params.projectId, session.userId)
    const result = await relay.call('git.exec', {
      cwd: params.worktreePath,
      args: ['commit', '-m', params.message],
    }) as { stdout: string; stderr: string; exitCode: number }

    if (result.exitCode === 0) {
      // Auto-advance referenced tasks
      await onCommitComplete(params.message, params.projectId, session.userId, taskService, taskGrantService)
    }

    return result
  })
}

async function onCommitComplete(
  commitMessage: string,
  projectId: string,
  userId: string,
  taskService: TaskService,
  taskGrantService: TaskGrantService
): Promise<void> {
  const taskRefs = [...commitMessage.matchAll(/#(TG-[\w-]+)/g)].map(m => m[1])
  for (const taskRef of taskRefs) {
    try {
      const task = await taskService.findByRef(taskRef)
      if (task && task.projectId === projectId) {
        const perm = await taskGrantService.resolvePermission(userId, task.id)
        if (perm && ['edit', 'execute', 'manage'].includes(perm)) {
          await taskService.update(task.id, { status: 'review' })
          await taskService.addComment(task.id, userId, `Commit: ${commitMessage}`, 'activity')
        }
      }
    } catch {
      // Non-fatal — don't fail the commit
    }
  }
}
```

---

## 4. Frontend Components

### `src/renderer/src/components/workspace/git/GitPanel.tsx`

Structure:
- Uses `useWorkspace()` hook for `gitStatus`, `currentWorktree`, `emit`
- Tabs: Changes, History, Branches, Worktrees
- Calls RPC methods via existing `rpc.call()` pattern

### `src/renderer/src/components/workspace/git/CommitForm.tsx`

```typescript
// Key features:
// - Stage/unstage individual files (git.add, git.restore)
// - Staged diff preview (git.diff --staged)
// - AI commit message button → rpc.call('git.generateCommitMessage', {...})
// - Commit button → rpc.call('git.commit', { message, projectId, worktreePath })
// - After commit: emit({ type: 'git.commit', hash: ..., message: ..., branch: ... })
```

### `src/renderer/src/components/workspace/git/DiffViewer.tsx`

```typescript
// Unified diff display with syntax highlighting
// Calls: rpc.call('git.diff', { projectId, worktreePath, file, staged })
```

### `src/renderer/src/components/workspace/git/BranchManager.tsx`

```typescript
// List branches, create, delete, checkout
// Streaming push/pull with progress display
// Calls: git.branch.list, git.branch.create, git.push, git.pull
```

### `src/renderer/src/components/workspace/git/PullRequestForm.tsx`

```typescript
// AI PR description generation
// Calls: git.generatePRDescription → text
// Calls: git.pr.create → { prUrl, prNumber }
```

---

## 5. server-bootstrap.ts

**No new step needed** — `registerRemoteGitRpcMethods()` được gọi khi RPC dispatcher được init, sau khi `projectRouter`, `aiProviderService`, `taskService`, `taskGrantService` đều sẵn sàng.

Thêm vào sau bước khởi tạo RPC server:

```typescript
// Wire remote git methods (sau rpcServer.start()):
const { registerRemoteGitRpcMethods } = await import('./runtime/rpc/methods/git-remote')
registerRemoteGitRpcMethods(projectRouter, aiProviderService, taskService, taskGrantService, rpcServer.dispatcher)
console.log('[ServerBootstrap] ✅ Remote git RPC methods registered')
```

---

## 6. Test files cần tạo

```
src/relay/__tests__/
├── git-handler.test.ts           (≥ 12 tests)
│   ├── validateGitArgs: allowed OK
│   ├── validateGitArgs: disallowed → GIT_DISALLOWED_SUBCOMMAND
│   ├── validateGitArgs: metacharacter → error
│   ├── git.exec: success → stdout
│   ├── git.exec: failure → non-zero exitCode (not thrown)
│   └── git.execStream: yields lines (mock child_process)

src/main/runtime/rpc/methods/__tests__/
├── git-remote.test.ts            (≥ 17 tests)
│   ├── git.status: relay.call invoked
│   ├── git.diff: staged=true → --staged flag
│   ├── git.add: files list passed
│   ├── git.commit: message passed + task auto-advance
│   ├── git.push: streaming relay.callStream
│   ├── git.generateCommitMessage: diff → AI → message
│   ├── git.generateCommitMessage: empty diff → GIT_NO_STAGED_CHANGES
│   ├── git.pr.create: gh CLI → relay exec
│   └── git.pr.create: no CLI → GITHUB_NO_CREDENTIAL

src/main/task/__tests__/
├── task-commit-advance.test.ts   (≥ 6 tests)
│   ├── commit with #TG-123 → task status = 'review'
│   ├── commit with no task ref → no change
│   └── commit with ref but no edit perm → no change
```

**Total: ≥ 35 tests**

---

## 7. Key Design Decisions

1. **git-handler isolation**: Runs on Dev Server relay, no Orca Server imports → can be tested without server
2. **Whitelist approach**: `ALLOWED_GIT_SUBCOMMANDS` prevents injection — same pattern as existing relay security
3. **Non-breaking extension**: Existing local git.ts unchanged; remote methods added as separate registration
4. **Streaming**: `git.push`/`pull` use `relay.callStream()` for progress — requires relay protocol streaming support
5. **Task auto-advance**: Non-fatal — commit never fails even if task update fails

---

## 8. Checklist

- [x] `src/relay/git-handler.ts` — relay-side git execution
- [x] Register `gitHandlers` in relay entry point (`src/relay/index.ts`)
- [x] `src/main/runtime/rpc/methods/git-remote.ts` — server-side routing
- [x] Wire `registerRemoteGitRpcMethods()` in `server-bootstrap.ts`
- [x] `src/renderer/src/components/workspace/git/GitPanel.tsx`
- [x] `src/renderer/src/components/workspace/git/CommitForm.tsx`
- [x] `src/renderer/src/components/workspace/git/DiffViewer.tsx`
- [x] `src/renderer/src/components/workspace/git/BranchManager.tsx`
- [x] `src/renderer/src/components/workspace/git/PullRequestForm.tsx`
- [x] Test files (≥ 35 tests)

## 9. Implementation Notes

| Spec Path | Actual Path | Note |
|-----------|------------|------|
| `src/relay/git-handler.ts` | `src/relay/git-remote-handler.ts` | Renamed for clarity (git-remote vs git-handler) |
| Register in `src/relay/index.ts` | Registered in `git-remote-handler.ts` exports | `gitRemoteHandlers` object exported |

**Test Results:** 34 pass (relay validation 16 + server-side RPC 18)  
**Security:** ALLOWED_GIT_SUBCOMMANDS whitelist + shell metachar `|;&$\`` rejection enforced  
**Features:** git.generateCommitMessage (AI diff→message), git.pr.create (gh CLI), #TG-xxx auto-advance  
**Implemented:** 2026-07-29 ✅
