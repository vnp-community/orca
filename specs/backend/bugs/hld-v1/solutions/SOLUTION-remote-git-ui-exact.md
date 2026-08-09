# SOLUTION: BUG-BE-HLD-018 — DevServerGitProvider missing operations — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế
**Files nguồn đã đọc:**
`backend/src/main/providers/dev-server-git-provider.ts` (toàn bộ, 535 dòng),
`backend/src/main/providers/ssh-git-provider.ts` (toàn bộ, 884 dòng),
`backend/src/main/providers/types.ts` (interface `IGitProvider`),
`backend/src/main/providers/dev-server-relay-connection.ts`,
`agent/src/relay/git-handler.ts` (class `GitHandler` — dùng cho SSH relay daemon, `relay.ts:472`),
`agent/src/relay/agent-rpc-dispatch.ts` (router RPC dùng cho Dev Server WS agent),
`agent/src/relay/agent-git-handler.ts` (`ALLOWED_GIT_SUBCOMMANDS`),
`agent/src/relay/git-handler-ops.ts`, `git-handler-commit-diff-ops.ts`, `git-handler-check-ignore.ts`,
`agent/src/relay/git-handler-submodule-ops.ts`, `git-handler-status-ops.ts`,
`agent/src/shared/git-history.ts`, `agent/src/shared/git-fork-sync.ts`,
`backend/src/shared/git-history.ts`, `backend/src/shared/types.ts`.

---

## 0. Phát hiện quan trọng — vì sao gap này tồn tại (đọc trước khi vá)

Có **HAI** bộ RPC handler git khác nhau ở phía Agent, không phải một:

| Transport | Dispatcher phía Agent | Danh sách method git đăng ký |
|---|---|---|
| SSH relay (`relay-ssh`) | `agent/src/relay/relay.ts:472` → `new GitHandler(dispatcher, context)` (class trong `git-handler.ts`) | **Đầy đủ**: `git.status`, `git.submoduleStatus`, `git.checkIgnored`, `git.history`, `git.branchCompare`, `git.commitCompare`, `git.branchDiff`, `git.commitDiff`, `git.forkSync`, `git.exec`, `git.worktree.*`, ... (xem `git-handler.ts:290-339`) |
| Dev Server WS (`relay-websocket` / `direct-websocket`) | `agent/src/relay/agent-rpc-dispatch.ts` (`route()`, dòng 256-856) | **Hẹp**: chỉ `git.exec`, `git.execStream`, `git.pr.create`, `git.worktree.list/add/remove` (xem case blocks dòng 298-452) — KHÔNG có `git.history`, `git.branchCompare`, `git.commitCompare`, `git.branchDiff`, `git.commitDiff`, `git.submoduleStatus`, `git.checkIgnored`, `git.forkSync` |

`SshGitProvider` gọi `mux.request('git.history', ...)` và Agent (qua `relay.ts`) trả lời được vì `GitHandler` class đã đăng ký handler đó. `DevServerGitProvider` gọi cùng phương thức qua `DevServerRelayConnection.call(...)` nhưng **kết nối tới Agent Dev Server chạy `agent-rpc-dispatch.ts`**, dispatcher này trả `Method not found` (`AgentErrorCode.MethodNotFound`) cho các method đó — đây chính là gap thật, không phải giới hạn kỹ thuật của `git.exec` (nhiều lệnh cần, ví dụ `symbolic-ref`, `for-each-ref`, `merge-base`, `check-ignore`, còn không nằm trong `ALLOWED_GIT_SUBCOMMANDS` của `agent-git-handler.ts:43-66` nữa).

**Tin tốt:** toàn bộ logic nghiệp vụ (`loadGitHistoryFromExecutor`, `branchCompareOp`, `commitCompareOp`, `branchDiffEntries`, `commitDiffEntry`, `checkIgnoredPathsOp`, `syncForkDefaultBranch`, submodule ops) đã tồn tại sẵn dưới dạng **hàm thuần nhận executor `git(args, cwd) => {stdout, stderr}`**, tách rời khỏi class `GitHandler` (comment đầu `git-handler-ops.ts`: *"These async operations accept a git executor callback so they remain decoupled from the GitHandler class"*). Vá đúng là **đăng ký các method còn thiếu vào `agent-rpc-dispatch.ts`, tái dùng các hàm thuần này**, KHÔNG viết lại logic.

Method duy nhất **không cần sửa Agent** là `getStagedCommitContext` — chỉ dùng `diff` và `branch`, cả hai đều nằm trong `ALLOWED_GIT_SUBCOMMANDS` hiện có của `agent-git-handler.ts`, nên compose thẳng qua `this.exec()` (đã có sẵn trong `DevServerGitProvider`) là đủ.

---

## 1. `getStagedCommitContext` — KHÔNG cần sửa Agent, compose qua `git.exec` có sẵn

**File:** `backend/src/main/providers/dev-server-git-provider.ts`
**Lines:** 186-188 (thay thế)

### Code sai hiện tại:
```typescript
async getStagedCommitContext(): Promise<CommitMessageDraftContext | null> {
  throw NOT_SUPPORTED('AI commit-message context')
}
```

### Fix — mirror `SshGitProvider.getStagedCommitContext` (`ssh-git-provider.ts:176-212`), dùng `this.exec()` sẵn có (dòng 84-98 của cùng file):
```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 186-188:
async getStagedCommitContext(worktreePath: string): Promise<CommitMessageDraftContext | null> {
  const branchPromise = this.exec(['branch', '--show-current'], worktreePath).catch(() => ({
    stdout: '',
    stderr: ''
  }))
  const [branchResult, summaryResult] = await Promise.all([
    branchPromise,
    this.exec(['diff', '--cached', '--name-status'], worktreePath)
  ])
  const stagedSummary = summaryResult.stdout.trim()
  if (!stagedSummary) {
    return null
  }
  let stagedPatch = ''
  try {
    const patchResult = await this.exec(
      ['diff', '--cached', '--patch', '--minimal', '--no-color', '--no-ext-diff'],
      worktreePath
    )
    stagedPatch = patchResult.stdout
  } catch (error) {
    if (!isMaxBufferOverflowError(error)) {
      throw error
    }
    // Why: a very large staged diff can overflow the agent exec buffer. The
    // patch is optional context (truncated later anyway), so degrade to the
    // file-name summary only, matching SshGitProvider's fallback.
    console.warn(
      '[dev-server-git] Staged patch too large to read; using file summary only:',
      describeMaxBufferOverflowError(error)
    )
  }
  return {
    branch: branchResult.stdout.trim() || null,
    stagedSummary,
    stagedPatch
  }
}
```

Thêm import ở đầu file (cùng nhóm với `isBinaryBuffer`):
```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thêm vào import block đầu file:
import {
  describeMaxBufferOverflowError,
  isMaxBufferOverflowError
} from '../git/max-buffer-overflow'
```

---

## 2. `getHistory` — CẦN bổ sung Agent trước (`git.history`)

### 2a. Agent-side (prerequisite) — `agent/src/relay/agent-rpc-dispatch.ts`

Thêm case mới ngay sau case `'git.execStream'` (dòng 320), tái dùng `loadGitHistoryFromExecutor` (đã import sẵn trong `git-handler.ts`, cùng cây `agent/src/shared/git-history.ts`):

```typescript
// agent/src/relay/agent-rpc-dispatch.ts — thêm case mới sau dòng 320:

// ── v5.0: git.history ────────────────────────────────────────────────────
case 'git.history': {
  try {
    const { handleGitHistory } = await import('./agent-git-handler-extended')
    return (await handleGitHistory(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.history unavailable: ${msg}`)
  }
}
```

Tạo module mới `agent/src/relay/agent-git-handler-extended.ts` — chứa handler cho toàn bộ 6 method còn thiếu (mục 2-6 trong file này), dùng chung một executor nội bộ giống hệt cách `handleGitExec` spawn git nhưng KHÔNG áp `ALLOWED_GIT_SUBCOMMANDS` (các hàm ops bên dưới tự quyết định subcommand, không nhận input tùy ý từ client — an toàn tương đương `GitHandler.git()` riêng tư trong `git-handler.ts:393-428`):

```typescript
// agent/src/relay/agent-git-handler-extended.ts (NEW)
// Handlers for the Dev Server WS agent's git RPC surface that SSH relay's
// GitHandler class already has (git-handler.ts) but agent-rpc-dispatch.ts's
// narrow git.exec-only router does not. Reuses the same decoupled ops
// functions GitHandler calls internally — no logic duplicated.
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { loadGitHistoryFromExecutor } from '../shared/git-history'
import { branchCompare as branchCompareOp, branchDiffEntries } from './git-handler-ops'
import { commitCompare as commitCompareOp, commitDiffEntry } from './git-handler-commit-diff-ops'
import { checkIgnoredPathsOp } from './git-handler-check-ignore'
import { syncForkDefaultBranch, validateGitForkSyncExpectedUpstream } from '../shared/git-fork-sync'
import { parseBranchDiff } from './git-handler-utils'
import { parseNumstat } from '../shared/git-uncommitted-line-stats'
import {
  resolveSubmoduleWorktreePath,
  resolveSubmoduleCommitRange,
  computeSubmoduleRangeEntries
} from './git-handler-submodule-ops'
import { getStatusOp } from './git-handler-status-ops'

const execFileAsync = promisify(execFile)
const MAX_GIT_BUFFER = 10 * 1024 * 1024

// Why: internal executor for the fixed-shape ops below (branch compare,
// history, ...) — NOT exposed as free-form exec, so it does not need
// agent-git-handler.ts's ALLOWED_GIT_SUBCOMMANDS whitelist (that whitelist
// guards the generic git.exec passthrough only). Mirrors GitHandler.git()
// in git-handler.ts:393-428, minus SSH-specific env knobs not needed here.
async function git(
  args: string[],
  cwd: string,
  opts?: { stdin?: string; timeout?: number }
): Promise<{ stdout: string; stderr: string }> {
  const { stdout, stderr } = await execFileAsync('git', args, {
    cwd,
    encoding: 'utf-8',
    maxBuffer: MAX_GIT_BUFFER,
    timeout: opts?.timeout
  })
  return { stdout: String(stdout), stderr: String(stderr) }
}

async function gitBuffer(args: string[], cwd: string): Promise<Buffer> {
  const { stdout } = (await execFileAsync('git', args, {
    cwd,
    encoding: 'buffer',
    maxBuffer: MAX_GIT_BUFFER
  })) as unknown as { stdout: Buffer }
  return stdout
}

// ── git.history ─────────────────────────────────────────────────────────
export async function handleGitHistory(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const worktreePath = params.worktreePath as string
  try {
    const result = await loadGitHistoryFromExecutor(git, worktreePath, {
      limit: typeof params.limit === 'number' ? params.limit : undefined,
      baseRef: typeof params.baseRef === 'string' ? params.baseRef : null
    })
    log.info(`git.history: worktreePath=${worktreePath} items=${result.items.length}`)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.branchCompare ──────────────────────────────────────────────────
export async function handleGitBranchCompare(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const baseRef = params.baseRef as string
  if (baseRef.startsWith('-')) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Base ref must not start with "-"' }
    }
  }
  const result = await branchCompareOp(git, worktreePath, baseRef, async (mergeBase, headOid) => {
    const { stdout } = await git(
      ['-c', 'core.quotePath=false', 'diff', '--name-status', '-M', '-C', mergeBase, headOid],
      worktreePath
    )
    const { stdout: numstat } = await git(
      ['-c', 'core.quotePath=false', 'diff', '--numstat', '-M', '-C', mergeBase, headOid],
      worktreePath
    )
    return parseBranchDiff(stdout, parseNumstat(numstat))
  })
  return { jsonrpc: '2.0', id, result }
}

// ── git.commitCompare ──────────────────────────────────────────────────
export async function handleGitCommitCompare(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const commitId = params.commitId as string
  const result = await commitCompareOp(git, worktreePath, commitId)
  return { jsonrpc: '2.0', id, result }
}

// ── git.branchDiff ─────────────────────────────────────────────────────
export async function handleGitBranchDiff(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const baseRef = params.baseRef as string
  if (baseRef.startsWith('-')) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Base ref must not start with "-"' }
    }
  }
  const result = await branchDiffEntries(git, gitBuffer, worktreePath, baseRef, {
    includePatch: params.includePatch as boolean | undefined,
    filePath: params.filePath as string | undefined,
    oldPath: params.oldPath as string | undefined
  })
  return { jsonrpc: '2.0', id, result }
}

// ── git.commitDiff ─────────────────────────────────────────────────────
export async function handleGitCommitDiff(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const result = await commitDiffEntry(gitBuffer, worktreePath, {
    commitOid: params.commitOid as string,
    parentOid: params.parentOid as string | null | undefined,
    filePath: params.filePath as string,
    oldPath: params.oldPath as string | undefined
  })
  return { jsonrpc: '2.0', id, result }
}

// ── git.checkIgnored ───────────────────────────────────────────────────
export async function handleGitCheckIgnored(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const result = await checkIgnoredPathsOp(git, params)
  return { jsonrpc: '2.0', id, result }
}

// ── git.forkSync ───────────────────────────────────────────────────────
export async function handleGitForkSync(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  try {
    const expectedUpstream = validateGitForkSyncExpectedUpstream(params.expectedUpstream, {
      required: true
    })
    const result = await syncForkDefaultBranch(
      (args) => git(args, worktreePath, { timeout: 60_000 }),
      { expectedUpstream }
    )
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ── git.submoduleStatus ────────────────────────────────────────────────
export async function handleGitSubmoduleStatus(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const worktreePath = params.worktreePath as string
  const submodulePath = params.submodulePath as string
  const area = params.area === 'staged' || params.area === 'untracked' ? params.area : 'unstaged'
  const staged = area === 'staged'
  const resolved = resolveSubmoduleWorktreePath(worktreePath, submodulePath)
  const workingResult = await getStatusOp(git, { ...params, worktreePath: resolved })
  const { fromOid, toOid } = await resolveSubmoduleCommitRange(
    git,
    worktreePath,
    submodulePath,
    staged
  )
  if (fromOid && toOid && fromOid !== toOid) {
    const rangeEntries = await computeSubmoduleRangeEntries(git, resolved, fromOid, toOid)
    const result = staged
      ? { ...workingResult, entries: rangeEntries }
      : {
          ...workingResult,
          entries: [
            ...rangeEntries,
            ...workingResult.entries.filter(
              (e: { path: string }) => !rangeEntries.some((r: { path: string }) => r.path === e.path)
            )
          ]
        }
    return { jsonrpc: '2.0', id, result }
  }
  const result = staged ? { ...workingResult, entries: [] } : workingResult
  return { jsonrpc: '2.0', id, result }
}
```

Đăng ký 5 case còn lại (`git.branchCompare`, `git.commitCompare`, `git.branchDiff`, `git.commitDiff`, `git.checkIgnored`, `git.forkSync`, `git.submoduleStatus`) vào `agent-rpc-dispatch.ts` theo đúng khuôn mẫu ở trên (mỗi case: dynamic import + gọi handler + bọc try/catch trả `AgentErrorCode.ServerError`).

> **Lưu ý:** đây là điều kiện tiên quyết bắt buộc — nếu chỉ sửa `backend/` mà không deploy Agent mới lên Dev Server, các method này sẽ tiếp tục trả lỗi `Method not found` từ Agent cũ đang chạy trên host. Cần version-gate hoặc yêu cầu người dùng nâng cấp Agent (xem BUG-BE-HLD-019 mục version-mismatch check — cùng cơ chế phát hiện Agent cũ có thể tái dùng để cảnh báo "Dev Server agent chưa hỗ trợ git log/compare, vui lòng cập nhật").

### 2b. Backend-side — `backend/src/main/providers/dev-server-git-provider.ts`

**Lines:** 182-184 (thay thế)

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 182-184:
async getHistory(
  worktreePath: string,
  options: GitHistoryOptions = {}
): Promise<GitHistoryResult> {
  return this.relay.call<GitHistoryResult>('git.history', {
    worktreePath,
    ...options
  })
}
```

Thêm `GitHistoryOptions` vào import (hiện file chỉ import `GitHistoryResult`):
```typescript
// backend/src/main/providers/dev-server-git-provider.ts — sửa import dòng 32:
import type { GitHistoryOptions, GitHistoryResult } from '../../shared/git-history'
```

---

## 3. `getBranchCompare` / `getCommitCompare` — CẦN Agent (mục 2a đã bổ sung `git.branchCompare` / `git.commitCompare`)

**File:** `backend/src/main/providers/dev-server-git-provider.ts`
**Lines:** 297-303 (thay thế cả hai)

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 297-303:
async getBranchCompare(worktreePath: string, baseRef: string): Promise<GitBranchCompareResult> {
  return this.relay.call<GitBranchCompareResult>('git.branchCompare', { worktreePath, baseRef })
}

async getCommitCompare(worktreePath: string, commitId: string): Promise<GitCommitCompareResult> {
  return this.relay.call<GitCommitCompareResult>('git.commitCompare', { worktreePath, commitId })
}
```

(`GitBranchCompareResult`, `GitCommitCompareResult` đã có sẵn trong import block của file — không cần thêm.)

---

## 4. `getBranchDiff` / `getCommitDiff` — CẦN Agent (mục 2a đã bổ sung `git.branchDiff` / `git.commitDiff`)

**File:** `backend/src/main/providers/dev-server-git-provider.ts`
**Lines:** 289-295 (thay thế cả hai)

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 289-295:
async getBranchDiff(
  worktreePath: string,
  baseRef: string,
  options?: { includePatch?: boolean; filePath?: string; oldPath?: string }
): Promise<GitDiffResult[]> {
  return this.relay.call<GitDiffResult[]>('git.branchDiff', {
    worktreePath,
    baseRef,
    ...options
  })
}

async getCommitDiff(
  worktreePath: string,
  args: { commitOid: string; parentOid?: string | null; filePath: string; oldPath?: string }
): Promise<GitDiffResult> {
  return this.relay.call<GitDiffResult>('git.commitDiff', {
    worktreePath,
    ...args
  })
}
```

> Không dùng `requestGitStreamable` (như `SshGitProvider` — dòng 602, 624) ở đây: đó là cơ chế chunk response qua kênh SSH mux riêng (`ssh-git-response-stream-reader.ts` + `git.responseAck`/`git.cancelResponseStream` trên `SshChannelMultiplexer`). `DevServerRelayConnection` không có API tương đương (`onNotification` chỉ nhận, không có cơ chế ack hai chiều theo `streamId`) — diff rất lớn trên Dev Server sẽ đi qua đường JSON-RPC response bình thường, chấp nhận giới hạn `MAX_MESSAGE_SIZE` (16MB, `relay-protocol.ts:15`) của khung WS thay vì streaming. Nếu cần streaming thật cho Dev Server, đó là hạng mục riêng ngoài phạm vi bug này.

---

## 5. `getSubmoduleStatus` — CẦN Agent (mục 2a đã bổ sung `git.submoduleStatus`)

**File:** `backend/src/main/providers/dev-server-git-provider.ts`
**Lines:** 174-176 (thay thế)

Interface thật của `IGitProvider.getSubmoduleStatus` (`providers/types.ts:317-321`) nhận 3 tham số — stub hiện tại (`async getSubmoduleStatus(): Promise<GitStatusResult>`) sai chữ ký, chỉ "khớp" nhờ TypeScript cho phép implement với ít tham số hơn. Sửa đúng chữ ký:

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 174-176:
async getSubmoduleStatus(
  worktreePath: string,
  submodulePath: string,
  area: GitStagingArea = 'unstaged'
): Promise<GitStatusResult> {
  return this.relay.call<GitStatusResult>('git.submoduleStatus', {
    worktreePath,
    submodulePath,
    area
  })
}
```

Thêm `GitStagingArea` vào import type block (dòng 19-30 hiện chưa có):
```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thêm vào import từ '../../shared/types':
import type {
  GitBranchCompareResult,
  GitCommitCompareResult,
  GitConflictOperation,
  GitDiffResult,
  GitForkSyncExpectedUpstream,
  GitForkSyncResult,
  GitPushTarget,
  GitStagingArea,
  GitUpstreamStatus,
  GitWorktreeInfo,
  RemoveWorktreeResult
} from '../../shared/types'
```

---

## 6. `checkIgnoredPaths` — CẦN Agent (mục 2a đã bổ sung `git.checkIgnored`)

**File:** `backend/src/main/providers/dev-server-git-provider.ts`
**Lines:** 178-180 (thay thế)

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 178-180:
async checkIgnoredPaths(worktreePath: string, relativePaths: string[]): Promise<string[]> {
  return this.relay.call<string[]>('git.checkIgnored', {
    worktreePath,
    paths: relativePaths
  })
}
```

---

## 7. `syncForkDefaultBranch` — CẦN Agent (mục 2a đã bổ sung `git.forkSync`)

**File:** `backend/src/main/providers/dev-server-git-provider.ts`
**Lines:** 305-310 (thay thế)

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 305-310:
async syncForkDefaultBranch(
  worktreePath: string,
  expectedUpstream: GitForkSyncExpectedUpstream
): Promise<GitForkSyncResult> {
  return this.relay.call<GitForkSyncResult>('git.forkSync', {
    worktreePath,
    ...(expectedUpstream ? { expectedUpstream } : {})
  })
}
```

---

## 8. Ngoài phạm vi backend — `GitLog.tsx` (frontend)

Bug ticket mục 4 đề xuất thêm `GitLog.tsx` ở frontend để hiển thị git log + branch graph khi repo hosted trên Dev Server — đây là công việc frontend riêng (`frontend/src/renderer/src/components/workspace/git/`), không thuộc scope code-level fix của backend. Sau khi `getHistory()` trả dữ liệu thật (mục 2), frontend có thể tái dùng đúng component/hook hiện đang gọi `getHistory()` cho SSH provider (nếu đã có) thay vì viết mới từ đầu — cần audit riêng xem `GitPanel.tsx` hiện đã có nhánh UI cho `GitHistoryResult` chưa.

---

## Tóm tắt thay đổi

| Method | File | Lines (cũ) | Cần sửa Agent trước? | RPC method |
|---|---|---|---|---|
| `getStagedCommitContext` | `dev-server-git-provider.ts` | 186-188 | ❌ Không (dùng `git.exec` có sẵn) | — |
| `getHistory` | `dev-server-git-provider.ts` | 182-184 | ✅ Có — thêm `git.history` | `git.history` |
| `getBranchCompare` | `dev-server-git-provider.ts` | 297-299 | ✅ Có — thêm `git.branchCompare` | `git.branchCompare` |
| `getCommitCompare` | `dev-server-git-provider.ts` | 301-303 | ✅ Có — thêm `git.commitCompare` | `git.commitCompare` |
| `getBranchDiff` | `dev-server-git-provider.ts` | 289-291 | ✅ Có — thêm `git.branchDiff` | `git.branchDiff` |
| `getCommitDiff` | `dev-server-git-provider.ts` | 293-295 | ✅ Có — thêm `git.commitDiff` | `git.commitDiff` |
| `getSubmoduleStatus` | `dev-server-git-provider.ts` | 174-176 | ✅ Có — thêm `git.submoduleStatus` | `git.submoduleStatus` |
| `checkIgnoredPaths` | `dev-server-git-provider.ts` | 178-180 | ✅ Có — thêm `git.checkIgnored` | `git.checkIgnored` |
| `syncForkDefaultBranch` | `dev-server-git-provider.ts` | 305-310 | ✅ Có — thêm `git.forkSync` | `git.forkSync` |
| Agent RPC router | `agent/src/relay/agent-rpc-dispatch.ts` | thêm 7 case mới sau dòng 320 | — | (prerequisite) |
| Agent handler module | `agent/src/relay/agent-git-handler-extended.ts` (NEW) | toàn bộ file | — | (prerequisite, tái dùng ops sẵn có) |

**Rủi ro triển khai:** các Dev Server đang chạy Agent binary cũ (chưa có `agent-git-handler-extended.ts`) sẽ tiếp tục nhận `Method not found` cho 7/9 method trên cho tới khi Agent được cập nhật/redeploy trên host đó — cần rollout theo cặp (backend + agent) hoặc kiểm tra `agentVersion` từ handshake (`WsHandshakeInfo.agentVersion`, xem SOLUTION-agent-ws-protocol-exact.md) trước khi UI cho phép bật các thao tác này.
