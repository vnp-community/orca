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

## Lưu ý: 2 mô hình song song (hiện tại vs. F34/F38)

Codebase đang mang **2 mô hình project khác nhau cùng lúc**:

1. **Mô hình hiện tại (mô tả ở trên)** — `Repo` + `Project` + `ProjectHostSetup`, sống trong
   `frontend/src/renderer/src/store/slices/repos.ts`. Đây là mô hình **đang thực sự chạy**.
2. **Mô hình tương lai `OrcaProject`** — đặc tả trong F34/F38 (1 Project gắn cứng 1 Dev Server
   duy nhất, không có khái niệm multi-host). Đã có scaffolding dở dang trong
   `frontend/src/renderer/src/store/slices/workspace-slice.ts` +
   `frontend/src/renderer/src/components/project/ProjectSettings.tsx`, nhưng **không được
   wire vào store** (đã gỡ khỏi `store/index.ts` — xem commit `fix(frontend): stop legacy
   workspace-slice from shadowing removeProject/updateProject`) vì nó khai báo trùng tên
   `removeProject`/`updateProject`/`projects` với mô hình hiện tại, âm thầm đè lên bản thật.

→ Khi triển khai tiếp F34/F38, cần dọn/migrate mô hình hiện tại thay vì để 2 mô hình cùng tồn
tại — tái sử dụng key trùng tên (`projects`, `removeProject`...) sẽ tái diễn đúng lớp bug vừa
fix trong phiên làm việc 2026-08-13.
