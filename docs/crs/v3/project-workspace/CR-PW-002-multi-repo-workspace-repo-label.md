# CR-PW-002 — Project Workspace không cho biết đang xem repo nào khi 1 Project có nhiều repo

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-002 |
| **Tên** | Gắn nhãn repo hiện tại trong Git tab + không đổi mô hình chọn worktree đã chốt |
| **Loại** | UX Fix (không phải Architectural Change) |
| **Priority** | 🟡 P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-06 |
| **Trạng thái** | ✅ Implemented — xem [FE-SOL-001](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-001-normalize-git-status-and-branch-display.md) (gộp chung 1 solution với CR-PW-001, cùng file `GitPanel.tsx`) |
| **Tác giả** | Investigation từ câu hỏi user: "nếu 1 Orca Project có nhiều legacy project (repo) → nên hiển thị thế nào?" |
| **Tác động HLD** | F38 — Project Workspace |
| **Tác động Features** | Git tab header |

---

## Bối cảnh & Vấn đề

Từ v0.4.x (xem `AssignRepoToProject` RPC, commit `a704f745b`, và "repo candidate picker" trong
Project Settings, commit `6cb05f0f3`), 1 Orca Project **có thể có nhiều repo** (`Repo.ProjectID`,
1-nhiều). Câu hỏi: Project Workspace (khu vực giữa — Explorer/Git/Tasks/Workflows) nên hiển thị
git-status thế nào khi có N repo, mỗi repo 1 branch riêng?

### Quyết định kiến trúc đã chốt (không nên đảo ngược)

`WorktreeList.tsx:5102-5109` — comment tại chỗ:

> *"Why: sync WorkspaceContext's currentWorktree to the sidebar's selection (roadmap decision #8,
> docs/guides/project-workspace-f38-doc-vs-code.md §4 step 3) — Workspace has no picker of its
> own, it reuses this one."*

Tức là: **Project Workspace không tự có bộ chọn repo/worktree riêng — sidebar bên trái (đã hỗ trợ
`WorktreeGroupBy: 'repo'`, mỗi repo 1 worktree card, mỗi card có branch/status riêng) chính là bộ
chọn.** Đây là quyết định đã chốt sau khi cân nhắc (roadmap `docs/guides/planning/
roadmap-orca-project-task-rbac.md` giai đoạn 2c) — CR này **không đề xuất đảo ngược** (không thêm
dropdown/selector mới trong `GitPanel`/`WorkspaceLayout`, tránh 2 nguồn chọn worktree cạnh tranh
nhau).

### Gap thật sự

Đúng như quyết định #8, khi user chọn 1 worktree ở sidebar, `currentWorktree` đồng bộ vào
`WorkspaceContext` — nhưng Git tab (`GitPanel.tsx`) hiện chỉ hiện **branch**, không hiện **repo
nào** đang được xem. Với project 1-repo thì không sao (ngầm hiểu); với project N-repo, người dùng
nhìn thấy 1 branch chip trơ trọi, không có cách nào (trong khu vực Workspace) biết nó thuộc repo
nào trong N repo — đặc biệt dễ nhầm khi 2 repo cùng có nhánh `main`.

## Giải pháp (tóm tắt — chi tiết ở FE-SOL-001)

Thêm nhãn tên repo (repo `displayName`, tra từ `useAppStore().repos` qua
`getRepoIdFromWorktreeId(currentWorktree.id)` — helper đã có sẵn ở
`frontend/src/shared/worktree-id.ts`) ngay cạnh branch chip trong `GitPanel.tsx` header. Không
thêm RPC mới, không thêm state chọn worktree mới — thuần hiển thị dữ liệu đã có sẵn trong store.

## Không thuộc phạm vi CR này

- Selector/dropdown chọn repo riêng trong Workspace — **cố tình không làm**, xem "quyết định đã
  chốt" ở trên.
- Backend/backend-go: **không cần đổi gì** — `Repo`/`repo.list`/`AssignRepoToProject` đã đủ dữ
  liệu ở client (store đã có `state.repos` với `projectId`). Không có agent-side impact.

## Liên quan

- `docs/guides/project-workspace/project-workspace-f38-doc-vs-code.md` §3/§4 bước 3 (quyết định #8)
- `backend-go/services/project-service/internal/usecase/assign_repo_to_project.go` (cơ chế gán
  nhiều repo vào 1 project — đã có, không cần đổi)
- [FE-SOL-001](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-001-normalize-git-status-and-branch-display.md)
