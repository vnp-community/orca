# TASK-HLD-028: Implement `getStagedCommitContext` trong `DevServerGitProvider`

**Priority:** 🟡 MEDIUM — quick win, không chặn merge, cải thiện UX AI commit-message trên Dev Server
**Effort:** ~15 phút
**Status:** ✅ DONE — 2026-08-09 (thay stub `NOT_SUPPORTED` bằng implementation đúng theo solution: `branch --show-current` + `diff --cached --name-status` chạy song song qua `Promise.all`, trả `null` nếu không có staged change, thử lấy `diff --cached --patch` đầy đủ với fallback degrade về summary-only khi overflow buffer (dùng `isMaxBufferOverflowError`/`describeMaxBufferOverflowError` từ `../git/max-buffer-overflow`, đã tồn tại sẵn). Thêm import mới. `tsc --noEmit` sạch hoàn toàn cho `dev-server-git-provider.ts`. ⚠️ Phát hiện `desktop/src/main/providers/dev-server-git-provider.ts` có stub `NOT_SUPPORTED` giống hệt, KHÔNG nằm trong phạm vi file list của task này — chưa sửa, ghi nhận làm gap cần task riêng nếu cần đồng bộ desktop/.)
**Bug refs:** BUG-BE-HLD-018
**Solution ref:** [SOLUTION-remote-git-ui-exact.md](../solutions/SOLUTION-remote-git-ui-exact.md) — Mục 1
**Depends on:** None

---

## Mục tiêu

`DevServerGitProvider.getStagedCommitContext()` hiện chỉ `throw NOT_SUPPORTED('AI commit-message context')`, khiến tính năng gợi ý commit message bằng AI không hoạt động khi repo được host trên Dev Server (nhưng hoạt động bình thường trên SSH provider). Đây là 1/9 method thiếu của `DevServerGitProvider` được liệt kê trong BUG-BE-HLD-018, nhưng là method **duy nhất không cần sửa Agent trước** — `diff --cached` và `branch --show-current` đều đã nằm trong `ALLOWED_GIT_SUBCOMMANDS` hiện có của `agent-git-handler.ts`, nên compose thẳng qua `this.exec()` sẵn có trong `DevServerGitProvider` là đủ.

Mirror logic từ `SshGitProvider.getStagedCommitContext` (`ssh-git-provider.ts:176-212`): lấy branch hiện tại + staged file summary (`diff --cached --name-status`) song song, rồi thử lấy staged patch đầy đủ (`diff --cached --patch --minimal --no-color --no-ext-diff`), degrade về chỉ có summary nếu patch quá lớn gây tràn buffer.

## File cần sửa/tạo

```
backend/src/main/providers/dev-server-git-provider.ts   (thay thế method stub dòng 186-188 + thêm import)
```

## Thay đổi cụ thể

### 1. Thay thế method stub (dòng 186-188)

Code sai hiện tại:
```typescript
async getStagedCommitContext(): Promise<CommitMessageDraftContext | null> {
  throw NOT_SUPPORTED('AI commit-message context')
}
```

Fix:
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

### 2. Thêm import ở đầu file (cùng nhóm với `isBinaryBuffer`)

```typescript
// backend/src/main/providers/dev-server-git-provider.ts — thêm vào import block đầu file:
import {
  describeMaxBufferOverflowError,
  isMaxBufferOverflowError
} from '../git/max-buffer-overflow'
```

## Verification

```bash
# 1. Xác nhận stub NOT_SUPPORTED đã bị loại bỏ
grep -n "NOT_SUPPORTED('AI commit-message context')" backend/src/main/providers/dev-server-git-provider.ts
# Expected: không có kết quả

# 2. Xác nhận import mới đã có
grep -n "isMaxBufferOverflowError\|describeMaxBufferOverflowError" backend/src/main/providers/dev-server-git-provider.ts
# Expected: >= 3 dòng khớp (import + 2 lần dùng trong method)

# 3. Type-check
pnpm --filter backend tsc --noEmit

# 4. Test thủ công / unit test (nếu có test file cho dev-server-git-provider):
# - Repo không có staged change → getStagedCommitContext trả về null
# - Repo có staged change nhỏ → trả về { branch, stagedSummary, stagedPatch } đầy đủ
# - Repo có staged diff cực lớn (giả lập overflow) → trả về stagedPatch rỗng, stagedSummary vẫn có,
#   không throw
```
