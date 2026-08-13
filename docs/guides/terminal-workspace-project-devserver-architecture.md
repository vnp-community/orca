# Kiến trúc hiện tại: Dev Server, Repo, Project, Worktree/Workspace, Terminal

**Cập nhật:** 2026-08-13

> Tài liệu này mô tả **kiến trúc đang chạy thật** trong code (`frontend/src/shared/types.ts`,
> `frontend/src/shared/dev-server-types.ts`, `frontend/src/renderer/src/store/slices/repos.ts`,
> `worktrees.ts`) — **không phải** mô hình `OrcaProject` được đặc tả trong
> [F34 — Project-Dev Server Binding](../features/F34-project-dev-server-binding.md) và
> [F38 — Project Workspace](../features/F38-project-workspace.md), vốn vẫn đang ở trạng thái
> "🚧 Phát triển" (v5.0+, chưa release). Xem mục [Lưu ý: 2 mô hình song song](#lưu-ý-2-mô-hình-song-song-hiện-tại-vs-f34f38) ở cuối.

## 1. Dev Server (host chạy)

Máy vật lý/ảo **thực thi mọi thứ** — nơi git repo thực sự nằm trên đĩa, nơi PTY/shell thực sự
chạy, nơi agent (Claude/Codex...) thực sự thực thi lệnh.

```typescript
// frontend/src/shared/dev-server-types.ts
DevServer {
  id: string                 // 'ds-<uuid>'
  connectionType: 'relay-ssh' | 'relay-websocket' | 'direct-websocket'
  status, platform, arch, capabilities   // populated sau handshake
}
```

Có 4 "loại host" trong hệ thống, phân biệt qua `Repo.executionHostId`:

| Giá trị | Ý nghĩa |
|---|---|
| `'local'` | Máy chạy Orca Desktop — **không tồn tại** ở chế độ web (paired web client) |
| `` ssh:<id> `` | Một SSH target thủ công |
| `` devServer:<id> `` | Một Dev Server đã pair (ví dụ `dev-01`, `dev-ai`, `test-01`) |
| `` runtime:<envId> `` | Một "runtime environment" đang active — với web client, đây thường là `session-auth`, tức kết nối riêng của chính client đó |

Mọi Repo/Worktree/Terminal đều phải "thuộc về" đúng **1 host cụ thể** — đây là bất biến quan
trọng nhất của toàn hệ thống.

## 2. Project (nhóm logic cấp cao, không gắn 1 host cụ thể)

Gom nhiều bản sao của "cùng một dự án" trên các host khác nhau thành **1 identity duy nhất** —
để UI hiển thị 1 card project dù cùng 1 repo tồn tại trên cả local lẫn nhiều dev-server.

```typescript
// frontend/src/shared/types.ts
Project {
  id: string
  displayName, badgeColor
  sourceRepoIds: string[]    // các Repo.id thuộc project này (có thể trải nhiều host)
}

ProjectHostSetup {           // cầu nối Project ↔ Repo ↔ Host cụ thể
  projectId, repoId, hostId  // "trên host X, project Y được setup bằng repo Z"
  setupState: ...
}
```

## 3. Repo (1 git repository cụ thể, trên 1 host)

Bản ghi "repo này nằm ở path nào, trên host nào".

```typescript
Repo {
  id: string
  path: string
  executionHostId?: 'local' | `ssh:${string}` | `runtime:${string}` | `devServer:${string}` | null
  connectionId?: string | null   // SSH target nếu remote
}
```

Mỗi cặp `(id, executionHostId)` phải là **duy nhất**. Khi cùng 1 `id` xuất hiện với 2
`executionHostId` khác nhau cho cùng 1 dữ liệu server-side thật, mọi thao tác xoá/quản lý bắt
đầu không biết nên chọn dòng nào → lỗi *"Workspace identity is ambiguous across hosts"*, và
"Remove Project" trông như không có tác dụng vì chỉ 1 trong 2 dòng ảo bị xoá.

## 4. Worktree — hay "Workspace" (1 checkout/branch cụ thể trong 1 Repo)

Mỗi Worktree = 1 git worktree thật (1 branch, 1 thư mục checkout) bên trong 1 Repo. Đây là đơn
vị người dùng thực sự "làm việc" trên — mỗi worktree có file explorer, git panel, terminal
riêng.

```typescript
Worktree {
  id: `${repoId}::${path}`
  repoId, projectId, hostId
  isMainWorktree: boolean
}
```

Ngoài ra còn **FolderWorkspace** — biến thể không cần git repo (chỉ 1 thư mục thường), thuộc về
1 Project Group thay vì 1 Repo. Dùng khi muốn Orca quản lý 1 folder không phải git.

## 5. Terminal (PTY session)

Tầng thấp nhất, thực thi thật — 1 shell/PTY chạy **trên chính Dev Server sở hữu Worktree đó**,
gắn với 1 Worktree cụ thể qua tab.

```
Tab { worktreeId } → PTY chạy trên hostId của worktree đó → Dev Server thực thi lệnh thật
```

## Sơ đồ quan hệ tổng thể

```
Dev Server (nơi chạy thật)
   └─ Repo (git repo, gắn đúng 1 host qua executionHostId)
        └─ Worktree (1 checkout/branch trong repo đó — "workspace" người dùng làm việc)
             └─ Terminal/PTY (chạy trên host của worktree đó)

Project (nhóm logic, KHÔNG gắn 1 host cụ thể)
   └─ ProjectHostSetup (cầu nối: "trên host X, project này = repo Y")
        └─ liên kết tới đúng 1 Repo trên đúng 1 host
```

**Điểm mấu chốt:** **Project** là khái niệm đa-host (1 project có thể tồn tại trên nhiều
dev-server cùng lúc), còn **Repo/Worktree/Terminal** luôn gắn chặt với **đúng 1 host**. Vi phạm
bất biến này (2 dòng Repo trùng `id` khác `executionHostId` cho cùng 1 dữ liệu thật) là nguồn
gốc của toàn bộ lớp bug "ambiguous host" gặp phải khi xoá project/workspace ở chế độ paired web
client — xem commit `fix(frontend): stop paired web clients from double-fetching
repos/groups/folder-workspaces`.

## Lưu ý: 2 mô hình song song (hiện tại vs. `OrcaProject`/F34/F38)

Codebase đang mang **2 mô hình project khác nhau cùng lúc**, và mô hình thứ hai đã được xây
xong phần lớn (không chỉ là ý tưởng trên giấy):

1. **Mô hình hiện tại (mô tả ở trên)** — `Repo` + `Project` + `ProjectHostSetup`, sống trong
   `frontend/src/renderer/src/store/slices/repos.ts`. Đa-host (1 project → N repo trên N host).
   Đây là mô hình **duy nhất có UI thật** — sidebar, Settings, mọi thứ người dùng thấy.

2. **Mô hình `OrcaProject`** (F34/F38, v5.0, TDD-15) — 1 project gắn **cứng đúng 1 Dev Server**
   (`OrcaProject.devServerId` + `repoPath`), có thêm `ProjectMember`/role (`owner`/`member`/
   `viewer`) và `visibility` (`private`/`team`/`company`) mà mô hình hiện tại chưa có.
   - **Backend đã xong và đang chạy thật**: `frontend/src/main/project/ProjectService.ts`
     (SQLite-backed, đủ CRUD + membership) expose qua RPC `project.*` trong
     `ProjectServerRouter.ts`. Đây chính là nơi phát sinh log `UNAUTHENTICATED` gặp trong lúc
     điều tra bug Remove Project — service có thật, có nhận traffic.
   - **Frontend đã xây gần đủ nhưng không có lối vào UI**: `WorkspaceContext.tsx`
     (`WorkspaceProvider`) được mount app-wide thật (`main.tsx` + `web/main-web-bootstrap.tsx`),
     nhưng `project` state khởi tạo `null` và chỉ đổi khi có nơi gọi `switchProject()`. Nơi duy
     nhất gọi `switchProject()` là `ProjectSwitcher.tsx`, và nơi tiêu thụ `useWorkspace()` là
     `WorkspaceLayout.tsx`/`ProjectSettings.tsx` — nhưng **cả 3 component này không được
     import/render ở bất kỳ đâu trong cây UI thật**, chỉ tồn tại trong file test của chính
     chúng. Kết quả: provider sống nhưng luôn ở trạng thái "chờ", không ai từng kích hoạt.
   - `workspace-slice.ts` (đã gỡ khỏi `store/index.ts` — xem commit `fix(frontend): stop
     legacy workspace-slice from shadowing removeProject/updateProject`) là phần store-side
     scaffolding của tính năng này; nó khai báo trùng tên `removeProject`/`updateProject`/
     `projects` với mô hình hiện tại nên âm thầm đè lên bản thật.

**2 rủi ro cần biết trước khi nối lại `ProjectSwitcher`/`WorkspaceLayout` vào layout thật:**
- `frontend/src/renderer/src/types/workspace-types.ts` có 1 bản `OrcaProject` **lệch field**
  so với bản thật trong `frontend/src/shared/project-types.ts` (ví dụ `visibility` khác giá
  trị enum, timestamp `number` vs `Date`) — cần hợp nhất về 1 nguồn trước khi dùng lại.
- Đừng tái sử dụng tên key trùng (`projects`, `removeProject`, `updateProject`...) khi wire
  `workspace-slice.ts` (hoặc phiên bản thay thế) vào store — đó chính xác là bug vừa fix trong
  phiên làm việc 2026-08-13.
