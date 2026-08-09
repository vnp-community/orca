# TASK-HLD-030: Wire `DevServerGitProvider` gọi 8 method RPC mới thay vì `NOT_SUPPORTED`

**Priority:** 🔴 HIGH — hoàn thiện parity git giữa Dev Server và SSH provider (8/9 method còn lại của BUG-BE-HLD-018)
**Effort:** ~45 phút
**Status:** ✅ DONE — 2026-08-09 (đúng theo solution — xác nhận vị trí thật của từng stub trước khi sửa (line number lệch so với task vì đã shift do TASK-HLD-028): `getSubmoduleStatus` sửa chữ ký từ sai `()` thành đúng 3 tham số `(worktreePath, submodulePath, area)` theo `IGitProvider` interface thật, `checkIgnoredPaths`, `getHistory` (+ import `GitHistoryOptions`), `getBranchDiff`, `getCommitDiff`, `getBranchCompare`, `getCommitCompare`, `syncForkDefaultBranch` (+ import `GitStagingArea`) — đủ 8/8, tất cả gọi đúng `this.relay.call<T>('git.<method>', {...})`. Sau khi thay hết, `NOT_SUPPORTED` helper không còn caller nào trong file (đã dùng hết ở cả 9/9 method BUG-BE-HLD-018, kể cả `getStagedCommitContext` từ TASK-HLD-028) → xoá định nghĩa để tránh lỗi `TS6133 unused` (không phải baseline — do chính task này gây ra, đã fix). `tsc --noEmit` sạch hoàn toàn. Verification bash 2 lệnh của task đều khớp: 0 `NOT_SUPPORTED(` call, 8/8 `this.relay.call` đúng method name. Cùng gap đã ghi nhận ở TASK-HLD-028: `desktop/src/main/providers/dev-server-git-provider.ts` có bản sao NOT_SUPPORTED stub tương tự, KHÔNG thuộc phạm vi task này, chưa sửa.)
**Bug refs:** BUG-BE-HLD-018
**Solution ref:** [SOLUTION-remote-git-ui-exact.md](../solutions/SOLUTION-remote-git-ui-exact.md) — Mục 2b, 3, 4, 5, 6, 7
**Depends on:** TASK-HLD-029 (Agent phải có sẵn 8 RPC method `git.history`/`git.branchCompare`/`git.commitCompare`/`git.branchDiff`/`git.commitDiff`/`git.checkIgnored`/`git.forkSync`/`git.submoduleStatus` trước khi backend gọi, nếu không sẽ tiếp tục nhận `Method not found`)

---

## Mục tiêu

Sau khi TASK-HLD-029 đăng ký đủ 8 RPC method còn thiếu ở phía Agent Dev Server WS, sửa `DevServerGitProvider` (backend) để gọi `this.relay.call(...)` tới các method mới này thay vì các stub `NOT_SUPPORTED` hiện tại. Cùng với TASK-HLD-028 (`getStagedCommitContext`, không phụ thuộc Agent), task này khép kín toàn bộ 9/9 method thiếu của BUG-BE-HLD-018.

Không dùng `requestGitStreamable` (cơ chế chunk response qua SSH mux của `SshGitProvider`) — `DevServerRelayConnection` không có API tương đương, nên diff lớn trên Dev Server đi qua JSON-RPC response bình thường, chấp nhận giới hạn `MAX_MESSAGE_SIZE` (16MB) của khung WS. Streaming thật cho Dev Server là hạng mục riêng, ngoài phạm vi bug này.

## File cần sửa/tạo

```
backend/src/main/providers/dev-server-git-provider.ts
```

## Thay đổi cụ thể

### 1. `getHistory` — thay thế dòng 182-184

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

Sửa import dòng 32 để có thêm `GitHistoryOptions` (hiện file chỉ import `GitHistoryResult`):
```typescript
// backend/src/main/providers/dev-server-git-provider.ts — sửa import dòng 32:
import type { GitHistoryOptions, GitHistoryResult } from '../../shared/git-history'
```

### 2. `getBranchCompare` / `getCommitCompare` — thay thế dòng 297-303

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

### 3. `getBranchDiff` / `getCommitDiff` — thay thế dòng 289-295

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

### 4. `getSubmoduleStatus` — thay thế dòng 174-176

Interface thật của `IGitProvider.getSubmoduleStatus` (`providers/types.ts:317-321`) nhận 3 tham số — stub hiện tại (`async getSubmoduleStatus(): Promise<GitStatusResult>`) sai chữ ký, chỉ "khớp" nhờ TypeScript cho phép implement với ít tham số hơn.

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

### 5. `checkIgnoredPaths` — thay thế dòng 178-180

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thay thế dòng 178-180:
async checkIgnoredPaths(worktreePath: string, relativePaths: string[]): Promise<string[]> {
  return this.relay.call<string[]>('git.checkIgnored', {
    worktreePath,
    paths: relativePaths
  })
}
```

### 6. `syncForkDefaultBranch` — thay thế dòng 305-310

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

## Verification

```bash
# 1. Không còn stub NOT_SUPPORTED cho 8 method này trong dev-server-git-provider.ts
grep -n "NOT_SUPPORTED" backend/src/main/providers/dev-server-git-provider.ts
# Expected: 0 kết quả liên quan tới getHistory/getBranchCompare/getCommitCompare/
# getBranchDiff/getCommitDiff/getSubmoduleStatus/checkIgnoredPaths/syncForkDefaultBranch
# (nếu file còn stub khác ngoài phạm vi BUG-BE-HLD-018, giữ nguyên — không đụng)

# 2. Mỗi method gọi đúng relay RPC method name tương ứng
grep -n "this.relay.call.*'git\.\(history\|branchCompare\|commitCompare\|branchDiff\|commitDiff\|submoduleStatus\|checkIgnored\|forkSync\)'" backend/src/main/providers/dev-server-git-provider.ts
# Expected: 8 dòng khớp

# 3. Type-check
pnpm --filter backend tsc --noEmit

# 4. Test tích hợp end-to-end trên Dev Server thật (đã chạy Agent mới từ TASK-HLD-029):
#    mở repo qua DevServerGitProvider, gọi getHistory/getBranchCompare/getCommitCompare/
#    getBranchDiff/getCommitDiff/getSubmoduleStatus/checkIgnoredPaths/syncForkDefaultBranch
#    và xác nhận kết quả khớp với hành vi tương ứng của SshGitProvider trên cùng repo.
#
# 5. Nhắc lại rủi ro rollout: nếu Dev Server target chạy Agent binary CŨ (trước
#    TASK-HLD-029), các lời gọi này sẽ trả lỗi "Method not found" — cần đảm bảo
#    Agent đã redeploy trước khi bật các thao tác này trên UI.
```
