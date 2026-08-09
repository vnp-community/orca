# TASK-HLD-029: Tạo `agent-git-handler-extended.ts` — implement 8 method git RPC còn thiếu ở Agent Dev Server WS

**Priority:** 🔴 HIGH — prerequisite bắt buộc cho TASK-HLD-030 và cho toàn bộ UI git trên Dev Server
**Effort:** ~1.5 giờ
**Status:** ✅ DONE — 2026-08-09 (tạo `agent/src/relay/agent-git-handler-extended.ts` đúng theo solution, tái dùng nguyên các hàm ops thuần đã xác nhận tồn tại (`branchCompare`/`branchDiffEntries` từ `git-handler-ops.ts`, `commitCompare`/`commitDiffEntry` từ `git-handler-commit-diff-ops.ts`, `checkIgnoredPathsOp`, `syncForkDefaultBranch`/`validateGitForkSyncExpectedUpstream`, `parseBranchDiff`, `parseNumstat`, `resolveSubmoduleWorktreePath`/`resolveSubmoduleCommitRange`/`computeSubmoduleRangeEntries`, `getStatusOp`, `loadGitHistoryFromExecutor`, `AgentErrorCode` — đã grep xác nhận từng export tồn tại đúng tên trước khi viết file). Đăng ký đủ 8 case (`git.history`, `git.branchCompare`, `git.commitCompare`, `git.branchDiff`, `git.commitDiff`, `git.checkIgnored`, `git.forkSync`, `git.submoduleStatus`) trong `agent-rpc-dispatch.ts` ngay sau `case 'git.execStream'`, đúng khuôn mẫu dynamic-import + try/catch có sẵn. Phát hiện thêm: `ai.provider.readCredential` RPC method mà TASK-HLD-023's `completeRotation()` gọi **đã tồn tại sẵn** ở dispatcher (dòng ~370) — không phải gap mới. Phải sửa 1 chỗ khác solution: `handleGitSubmoduleStatus`'s filter/some callback trên `Record<string, unknown>[]` cần cast `(e) => ... (e as {path:string}).path` thay vì narrow tham số callback trực tiếp thành `{path: string}` — TS7 beta từ chối overload đó (không sound). `tsc --noEmit` sạch hoàn toàn cho cả 2 file. Verification bash 2 lệnh của task (export list + case list) đều khớp đủ 8/8.)
**Bug refs:** BUG-BE-HLD-018
**Solution ref:** [SOLUTION-remote-git-ui-exact.md](../solutions/SOLUTION-remote-git-ui-exact.md) — Mục 0, Mục 2a
**Depends on:** None

---

## Mục tiêu

Có **HAI** bộ RPC handler git khác nhau ở phía Agent — SSH relay (`git-handler.ts`, đầy đủ method) và Dev Server WS (`agent-rpc-dispatch.ts`, chỉ có `git.exec`/`git.execStream`/`git.pr.create`/`git.worktree.*`). `DevServerGitProvider` (backend) gọi các RPC method như `git.history`, `git.branchCompare`, ... qua `DevServerRelayConnection.call(...)`, nhưng dispatcher Dev Server WS trả `Method not found` vì các method đó chưa được đăng ký ở phía Agent.

Toàn bộ logic nghiệp vụ đã tồn tại sẵn dưới dạng **hàm thuần nhận executor `git(args, cwd) => {stdout, stderr}`**, tách rời khỏi class `GitHandler` (dùng cho SSH relay). Task này **tái dùng nguyên các hàm thuần đó**, KHÔNG viết lại logic — chỉ tạo module handler mới cho Dev Server WS dispatcher và đăng ký 8 RPC method còn thiếu:

`git.history`, `git.branchCompare`, `git.commitCompare`, `git.branchDiff`, `git.commitDiff`, `git.checkIgnored`, `git.forkSync`, `git.submoduleStatus`.

Đây là **prerequisite bắt buộc** cho TASK-HLD-030 (backend wiring) — nếu chỉ làm TASK-HLD-030 mà không có task này, backend sẽ gọi RPC method không tồn tại ở Agent và tiếp tục nhận lỗi `Method not found`.

⚠️ **Rủi ro triển khai (ghi chú, không phải phần code của task này):** các Dev Server đang chạy Agent binary cũ (chưa có `agent-git-handler-extended.ts`) sẽ tiếp tục nhận `Method not found` cho tới khi Agent được cập nhật/redeploy trên host đó — cần rollout theo cặp (backend + agent) hoặc kiểm tra `agentVersion` từ handshake trước khi UI cho phép bật các thao tác này (xem TASK-HLD-032 — cơ chế version-mismatch check có thể tái dùng để cảnh báo).

## File cần sửa/tạo

```
agent/src/relay/agent-git-handler-extended.ts   (NEW — toàn bộ file)
agent/src/relay/agent-rpc-dispatch.ts           (thêm 8 case mới sau case 'git.execStream', dòng ~320)
```

## Thay đổi cụ thể

### 1. File mới: `agent/src/relay/agent-git-handler-extended.ts`

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

### 2. Đăng ký 8 case mới trong `agent/src/relay/agent-rpc-dispatch.ts` (sau case `'git.execStream'`, dòng ~320)

Mẫu cho `git.history` (7 case còn lại theo đúng khuôn mẫu: dynamic import + gọi handler tương ứng + bọc try/catch trả `AgentErrorCode.ServerError`):

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

// ── v5.0: git.branchCompare ─────────────────────────────────────────────
case 'git.branchCompare': {
  try {
    const { handleGitBranchCompare } = await import('./agent-git-handler-extended')
    return (await handleGitBranchCompare(rpc.id, rpc.params ?? {})) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.branchCompare unavailable: ${msg}`)
  }
}

// ── v5.0: git.commitCompare ─────────────────────────────────────────────
case 'git.commitCompare': {
  try {
    const { handleGitCommitCompare } = await import('./agent-git-handler-extended')
    return (await handleGitCommitCompare(rpc.id, rpc.params ?? {})) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.commitCompare unavailable: ${msg}`)
  }
}

// ── v5.0: git.branchDiff ────────────────────────────────────────────────
case 'git.branchDiff': {
  try {
    const { handleGitBranchDiff } = await import('./agent-git-handler-extended')
    return (await handleGitBranchDiff(rpc.id, rpc.params ?? {})) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.branchDiff unavailable: ${msg}`)
  }
}

// ── v5.0: git.commitDiff ────────────────────────────────────────────────
case 'git.commitDiff': {
  try {
    const { handleGitCommitDiff } = await import('./agent-git-handler-extended')
    return (await handleGitCommitDiff(rpc.id, rpc.params ?? {})) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.commitDiff unavailable: ${msg}`)
  }
}

// ── v5.0: git.checkIgnored ──────────────────────────────────────────────
case 'git.checkIgnored': {
  try {
    const { handleGitCheckIgnored } = await import('./agent-git-handler-extended')
    return (await handleGitCheckIgnored(rpc.id, rpc.params ?? {})) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.checkIgnored unavailable: ${msg}`)
  }
}

// ── v5.0: git.forkSync ──────────────────────────────────────────────────
case 'git.forkSync': {
  try {
    const { handleGitForkSync } = await import('./agent-git-handler-extended')
    return (await handleGitForkSync(rpc.id, rpc.params ?? {})) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.forkSync unavailable: ${msg}`)
  }
}

// ── v5.0: git.submoduleStatus ───────────────────────────────────────────
case 'git.submoduleStatus': {
  try {
    const { handleGitSubmoduleStatus } = await import('./agent-git-handler-extended')
    return (await handleGitSubmoduleStatus(rpc.id, rpc.params ?? {})) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.submoduleStatus unavailable: ${msg}`)
  }
}
```

## Verification

```bash
# 1. File mới tồn tại và export đủ 8 handler
grep -n "^export async function handle" agent/src/relay/agent-git-handler-extended.ts
# Expected: 8 dòng — handleGitHistory, handleGitBranchCompare, handleGitCommitCompare,
# handleGitBranchDiff, handleGitCommitDiff, handleGitCheckIgnored, handleGitForkSync,
# handleGitSubmoduleStatus

# 2. Cả 8 case đã đăng ký trong dispatcher
grep -n "case 'git\.\(history\|branchCompare\|commitCompare\|branchDiff\|commitDiff\|checkIgnored\|forkSync\|submoduleStatus\)'" agent/src/relay/agent-rpc-dispatch.ts
# Expected: 8 dòng khớp

# 3. Type-check phía agent
pnpm --filter agent tsc --noEmit

# 4. Unit test: gọi từng RPC method qua dispatcher trên một fixture repo thật
#    (staged changes, >=2 branch, >=1 submodule nếu có fixture) và so sánh kết quả
#    với output tương ứng từ GitHandler (SSH relay) trên cùng fixture — phải khớp
#    schema/shape vì cả hai tái dùng chung các hàm ops thuần.
pnpm --filter agent test -- agent-git-handler-extended
```
