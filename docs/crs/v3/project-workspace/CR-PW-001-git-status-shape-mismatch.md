# CR-PW-001 — Project Workspace Git tab: response-shape mismatch makes branch/ahead/behind unreliable

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-001 |
| **Tên** | Git tab (`GitPanel`) hiển thị sai/ẩn trạng thái branch vì cast nhầm shape RPC |
| **Loại** | Bug Fix |
| **Priority** | 🔴 P0 — thông tin git hiển thị sai cho mọi user, mọi lúc |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-06 |
| **Trạng thái** | ✅ Implemented — xem [FE-SOL-001](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-001-normalize-git-status-and-branch-display.md) |
| **Tác giả** | Investigation từ báo cáo user (b15.openledger.vn → Project Workspace → "Vnp-asm" → tab Git hiện "(no branch)") |
| **Tác động HLD** | F38 — Project Workspace |
| **Tác động Features** | Git tab (branch/ahead-behind indicator), Sync button |

---

## Bối cảnh & Vấn đề

User report: mở Project Workspace cho project "Vnp-asm" trên deployment b15.openledger.vn, tab
Git hiện branch = **"(no branch)"** dù repo có branch bình thường.

### Root cause #1 — `WorkspaceContext.refreshGitStatus()` cast sai kiểu, không map field

`git.status` RPC trả về `GitStatusResult` thật
(`backend/src/shared/git-status-types.ts:55-72`):

```typescript
export type GitStatusResult = {
  entries: GitStatusEntry[]
  conflictOperation: GitConflictOperation
  head?: string
  branch?: string           // RAW ref, ví dụ "refs/heads/main" — hoặc undefined
  upstreamStatus?: GitUpstreamStatus   // { hasUpstream, ahead, behind, ... }
  ...
}
```

Nhưng `frontend/src/renderer/src/context/WorkspaceContext.tsx:160-163` gọi:

```typescript
const status = await callRuntimeRpc<GitStatus>(target, 'git.status', {...})
setGitStatus(status)
```

ép kiểu thẳng sang `GitStatus` — 1 type **hoàn toàn khác shape**, tự khai ở
`frontend/src/renderer/src/types/workspace-types.ts:84-91`:

```typescript
export type GitStatus = {
  branch: string        // ← thực tế BE trả "refs/heads/main" (raw), không phải "main"
  aheadBy: number        // ← field này KHÔNG TỒN TẠI trên GitStatusResult
  behindBy: number        // ← field này KHÔNG TỒN TẠI trên GitStatusResult
  hasUncommitted: boolean
  staged: number
  unstaged: number
}
```

`callRuntimeRpc<GitStatus>` chỉ là type assertion phía TypeScript — không có runtime mapping nào
ở giữa. Hậu quả cụ thể, xác nhận bằng code:

1. **Khi có branch thật**: `GitPanel.tsx:62` hiển thị nguyên `gitStatus.branch` — ra
   `"refs/heads/main"` chứ không phải `"main"` (không dùng `branchName()` đã có sẵn ở
   `frontend/src/renderer/src/lib/git-utils.ts`).
2. **Ahead/behind luôn 0**: `GitPanel.tsx:65` đọc `gitStatus.aheadBy`/`gitStatus.behindBy` —
   field không tồn tại trên response thật (số ahead/behind thật nằm ở
   `gitStatus.upstreamStatus.ahead/.behind`) → Sync indicator luôn hiện `↑0 ↓0` bất kể trạng thái
   thật.
3. **"(no branch)" xuất hiện trong ít nhất 2 trường hợp khác nhau, không phân biệt được**:
   - Detached HEAD thật (`branch` hợp lệ là `undefined`, `status.ts:313`).
   - `git status` chạy thất bại trên host (thiếu `.git`, git không có trên dev-server, SSH/relay
     lỗi) — hàm vẫn return "thành công" với `branch: undefined` thay vì throw
     (`backend/src/main/git/status.ts:291-308`, `backend/src/main/providers/dev-server-git-provider.ts:109-172`)
     — case thực tế nhiều khả năng nhất cho b15 (hosted, multi-repo qua dev-server relay).
4. **Khi RPC tự throw** (network/relay lỗi thật), `WorkspaceContext.tsx:164-166` nuốt lỗi hoàn
   toàn im lặng (`catch { /* Silently fail */ }`) — không có cách nào phân biệt với case 3.

### Root cause #2 — Git tab được render kể cả khi chưa có worktree nào được chọn

`WorkspaceLayout.tsx:78` render `<GitPanel />` không điều kiện cho `activeTab === 'git'`, trong
khi tab "Agent" liền kề (`:81-85`) đã có sẵn pattern đúng
(`currentWorktree ? <AgentPanel .../> : <NoWorktreeSelected />`). `currentWorktree` chỉ được set
khi sidebar đồng bộ lựa chọn (`WorktreeList.tsx:5102-5109`, "quyết định #8" —
xem `docs/guides/project-workspace/project-workspace-f38-doc-vs-code.md` §3/§4 bước 3); ngay sau
`switchProject()`, `gitStatus`/`currentWorktree` có thể chưa có gì — `GitPanel` vẫn render và rơi
vào fallback "(no branch)" thay vì trạng thái "chưa chọn worktree" đã có sẵn UI cho nó.

---

## Giải pháp (tóm tắt — chi tiết ở FE-SOL-001)

1. Chuẩn hoá `GitStatusResult` → `GitStatus` đúng field (branch đã strip `refs/heads/`,
   `aheadBy`/`behindBy` từ `upstreamStatus`), phân biệt rõ "chưa chọn worktree" / "detached HEAD" /
   "git status thất bại trên host" — không còn gộp chung vào 1 chuỗi "(no branch)".
2. Gate tab Git đằng sau `currentWorktree` giống tab Agent, tái dùng `NoWorktreeSelected` đã có.

## Không thuộc phạm vi CR này

- Sửa `s.executions`/`window.api` event bridge (khác domain, xem CR-PW-003).
- Xây lại toàn bộ Source Control sidebar (đã đúng, dùng làm tham chiếu ở FE-SOL-001).
- Backend/backend-go: **không cần đổi gì** — `git.status` RPC đã trả đúng dữ liệu thật; lỗi hoàn
  toàn ở tầng frontend (map/cast sai kiểu). Không có agent-side (`agent/`) impact.

## Liên quan

- `docs/guides/project-workspace/project-workspace-f38-doc-vs-code.md` — ghi nhận trước đó việc
  `WorkspaceContext` gọi sai contract `git.status` (đã fix phần tham số `{worktree}` từ trước khi
  CR này viết); CR này là 1 lớp lỗi khác, sâu hơn (shape response, không phải tham số request).
- [FE-SOL-001](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-001-normalize-git-status-and-branch-display.md)
