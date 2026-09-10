# FE-SOL-001: Chuẩn hoá `git.status` response + phân biệt trạng thái branch + gắn nhãn repo

> **✅ ĐÃ IMPLEMENT (2026-09-06)** — 4 file sản xuất + 2 file test sửa đúng như kế hoạch dưới, cộng
> 1 file test không nằm trong bảng "Files cần sửa" ban đầu:
> `frontend/src/renderer/src/context/__tests__/WorkspaceContext.test.tsx` (2 test case cũ giả định
> `git.status` trả nguyên `{branch: 'feature'}` và mong `ctxValue.gitStatus` bằng đúng object đó —
> phải cập nhật theo shape chuẩn hoá mới, cộng 1 test case mới cho `gitStatusError`).
> `toWorkspaceGitStatus()` cũng thêm `result.entries ?? []` (không có trong bản nháp) vì test cũ
> mock response tối giản không có `entries` — bản thật từ `status.ts` luôn có `entries: []`, nhưng
> hàm map cần an toàn với input tối giản của test. `npx tsc --noEmit -p tsconfig.json`: so sánh
> before/after xác nhận **0 lỗi type mới** ngoài 1 dòng đổi tên field trong message lỗi đã có sẵn
> từ trước (`ProjectSwitcher.test.tsx`, không do solution này gây ra — file đó đã lỗi từ trước).
> `gitnexus detect_changes`: risk **low**, 0 execution flow bị ảnh hưởng, đúng 20 symbol trong 9
> file khớp phạm vi sửa. 38/38 test pass.

## CR Reference
- **CR:** [CR-PW-001](../../../../../../docs/crs/v3/project-workspace/CR-PW-001-git-status-shape-mismatch.md), [CR-PW-002](../../../../../../docs/crs/v3/project-workspace/CR-PW-002-multi-repo-workspace-repo-label.md)
- **Mức độ:** 🔴 P0 (CR-PW-001) + 🟡 P1 (CR-PW-002)
- **Impact analysis (gitnexus, chạy trước khi sửa):** `refreshGitStatus` — risk LOW, 4 symbol bị
  ảnh hưởng, 0 execution flow; `GitPanel` — risk LOW, 0 caller bị ảnh hưởng trực tiếp (component
  lá, không ai gọi trực tiếp ngoài React render tree).

---

## Root Cause

Xem chi tiết đầy đủ ở CR-PW-001. Tóm tắt: `WorkspaceContext.refreshGitStatus()` ép kiểu response
RPC thật (`GitStatusResult`) sang 1 type tự khai sai shape (`GitStatus`) mà không map field nào —
branch không được strip `refs/heads/`, `aheadBy`/`behindBy` đọc nhầm field không tồn tại (luôn
`undefined`→hiện `0`), và không phân biệt được "chưa chọn worktree" / "detached HEAD" / "git
status thất bại trên host dev-server".

## Giải pháp

### Bước 1 — Sửa type `GitStatus` (khớp đúng cái GitPanel thực sự cần, không khai field chết)

**File:** `frontend/src/renderer/src/types/workspace-types.ts` (MODIFY)

```typescript
// GitPanel chỉ đọc branch/aheadBy/behindBy (xác nhận bằng grep — staged/unstaged/hasUncommitted
// không có consumer thật, StagingArea/CommitForm dùng store.stagedFiles/unstagedFiles qua
// useGit() thay vì gitStatus). branch optional: undefined là trạng thái hợp lệ (detached HEAD
// hoặc `git status` thất bại trên host) — GitPanel phải phân biệt 2 case này qua branchUnavailable.
export type GitStatus = {
  branch?: string
  branchUnavailable?: 'detached-head' | 'status-unavailable'
  aheadBy: number
  behindBy: number
  hasUncommitted: boolean
  staged: number
  unstaged: number
}
```

### Bước 2 — Map `GitStatusResult` → `GitStatus` đúng field trong `WorkspaceContext.tsx`

**File:** `frontend/src/renderer/src/context/WorkspaceContext.tsx` (MODIFY)

```typescript
import type { GitStatusResult } from '../../../shared/git-status-types'
import { branchName } from '../lib/git-utils'

// Why (CR-PW-001): git.status trả GitStatusResult thật (entries/head/branch raw ref/
// upstreamStatus), khác hẳn shape GitStatus cũ mà GitPanel đọc — map tường minh ở đây thay vì
// ép kiểu, để branch được strip refs/heads/, ahead/behind lấy đúng từ upstreamStatus.ahead/behind,
// và phân biệt detached HEAD (result.head có giá trị) với git-status-thất-bại (cả head lẫn branch
// đều rỗng — status.ts vẫn return "thành công" khi gitStreamStdout throw, xem status.ts:291-308).
function toWorkspaceGitStatus(result: GitStatusResult): GitStatus {
  const staged = result.entries.filter((e) => e.area === 'staged').length
  const unstaged = result.entries.filter((e) => e.area === 'unstaged').length
  return {
    branch: result.branch ? branchName(result.branch) : undefined,
    branchUnavailable: result.branch
      ? undefined
      : result.head
        ? 'detached-head'
        : 'status-unavailable',
    aheadBy: result.upstreamStatus?.ahead ?? 0,
    behindBy: result.upstreamStatus?.behind ?? 0,
    hasUncommitted: result.entries.length > 0,
    staged,
    unstaged,
  }
}
```

Trong `refreshGitStatus`, thay:

```typescript
const status = await callRuntimeRpc<GitStatus>(target, 'git.status', {
  worktree: toRuntimeWorktreeSelector(currentWorktree.id),
})
setGitStatus(status)
```

bằng:

```typescript
const status = await callRuntimeRpc<GitStatusResult>(target, 'git.status', {
  worktree: toRuntimeWorktreeSelector(currentWorktree.id),
})
setGitStatus(toWorkspaceGitStatus(status))
setGitStatusError(false)
```

và trong `catch`, thay comment-only bằng:

```typescript
} catch {
  // CR-PW-001: không còn nuốt lỗi hoàn toàn im lặng — GitPanel cần phân biệt "RPC thật sự lỗi"
  // (network/relay) khỏi "status thành công nhưng không có branch" (status-unavailable ở trên).
  setGitStatusError(true)
}
```

Thêm state `gitStatusError` (giống pattern `isOffline` đã có) vào `WorkspaceContextValue` +
provider, reset về `false` khi `currentWorktree` đổi hoặc `switchProject` chạy (cùng chỗ đang
reset `gitStatus` về `null`).

### Bước 3 — Gate tab Git đằng sau `currentWorktree`, tái dùng `NoWorktreeSelected`

**File:** `frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx` (MODIFY)

```tsx
{activeTab === 'git' && (
  currentWorktree ? <GitPanel /> : <NoWorktreeSelected />
)}
```

(Y hệt pattern đã có sẵn cho tab `'agent'` ngay dưới — không phải code mới, chỉ áp dụng lại.)

### Bước 4 — `GitPanel.tsx`: hiển thị đúng trạng thái + nhãn repo (CR-PW-002)

**File:** `frontend/src/renderer/src/components/workspace/git/GitPanel.tsx` (MODIFY)

- Bỏ fallback cứng `'(no branch)'`; thay bằng hàm nhỏ phân biệt 3 trạng thái
  (`branch` có giá trị / `branchUnavailable === 'detached-head'` / `'status-unavailable'`), cộng
  `gitStatusError` từ context (RPC tự throw).
- Vì Bước 3 đã đảm bảo `GitPanel` chỉ render khi có `currentWorktree`, không cần tự xử lý case
  "chưa chọn worktree" nữa trong component này.
- Thêm nhãn tên repo cạnh branch chip, tra từ `useAppStore().repos` qua
  `getRepoIdFromWorktreeId(currentWorktree.id)` (helper có sẵn ở `frontend/src/shared/worktree-id.ts`)
  — **không** thêm selector/dropdown mới (CR-PW-002 cố tình giữ nguyên quyết định #8: sidebar là
  bộ chọn worktree duy nhất).
- `handleSync`'s `gitStatus.branch ?? 'main'` giữ nguyên (branch giờ đã đúng — sẽ không còn ép
  push nhầm nhánh `main` khi thực ra đang ở nhánh khác mà chỉ vì trước đó branch bị đọc sai field).

## Files cần sửa

| File | Action |
|------|--------|
| `frontend/src/renderer/src/types/workspace-types.ts` | MODIFY — `GitStatus.branch` optional + `branchUnavailable` |
| `frontend/src/renderer/src/context/WorkspaceContext.tsx` | MODIFY — `toWorkspaceGitStatus()`, `gitStatusError` state |
| `frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx` | MODIFY — gate Git tab |
| `frontend/src/renderer/src/components/workspace/git/GitPanel.tsx` | MODIFY — trạng thái branch + nhãn repo |
| `frontend/src/renderer/src/components/workspace/git/__tests__/GitPanel.test.tsx` | MODIFY — mock theo shape mới, +5 test case (3 trạng thái branch, 2 nhãn repo) |
| `frontend/src/renderer/src/context/__tests__/WorkspaceContext.test.tsx` | MODIFY — 2 test case cập nhật theo shape chuẩn hoá, +1 test case `gitStatusError` |
| `frontend/src/renderer/src/components/workspace/__tests__/WorkspaceLayout.test.tsx` | MODIFY — test tab Git cũ giả định render vô điều kiện, tách thành 2 case (có/không `currentWorktree`) |

## Task breakdown

- [FE-TASK-001](../tasks/FE-TASK-001-normalize-git-status-response.md)
- [FE-TASK-002](../tasks/FE-TASK-002-gitpanel-status-states-and-repo-label.md)
- [FE-TASK-003](../tasks/FE-TASK-003-gate-git-tab-behind-worktree.md)

## Verification

```bash
cd frontend && npx vitest run src/renderer/src/components/workspace/git/__tests__/GitPanel.test.tsx
cd frontend && npx tsc --noEmit -p .
```

## Không làm ở solution này

- Selector chọn repo/worktree riêng trong Workspace (xem CR-PW-002 — quyết định #8 giữ nguyên).
- Sửa `useGit.ts`/`StagingArea`/`CommitForm` — chúng đã dùng đúng `GitStatusResult` thật, không
  đi qua field sai của `GitStatus`.
