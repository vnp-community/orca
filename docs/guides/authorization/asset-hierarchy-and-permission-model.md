# Tổ chức tài sản (Project/Repo/Folder/Worktree/Task) & Mô hình Phân quyền trong Orca

**Tổng hợp từ:** `docs/guides/project-workspace/*`, `docs/guides/profile/user-profile-team-department-rbac.md`,
`docs/guides/task-automation/task-automation-orchestration-integration.md`, `docs/features/F32/F33/F37/F38.md`,
`docs/logic/task-graph/BL-TG-03-task-access-control.md`, `docs/crs/v1/login/CR-LOGIN-004-admin.md`,
`docs/hld/v1/security.md`.

> **Lưu ý về độ tin cậy:** Orca hiện có **2 mô hình "project" chạy song song, cả hai đều thật**
> (không phải 1 cái thật 1 cái ý tưởng) — mô hình `Repo`/`Project` cũ (per-user, đa-host, KHÔNG có
> RBAC) và mô hình `OrcaProject` mới (v5.0, SQL dùng chung, CÓ RBAC qua `ProjectMember`). Hai mô
> hình chưa được nối với nhau. Tài liệu này mô tả **cả hai**, chỉ rõ mô hình nào có phân quyền và
> mô hình nào chưa, dựa trên đối chiếu trực tiếp `backend/src/` (không phải `frontend/src/main/`,
> vốn là code Electron chết không chạy production — xem đính chính trong nguồn).

---

## 1. Các khái niệm tài sản (asset) — định nghĩa & phân cấp

### 1.1 Dev Server — host thực thi

Máy vật lý/ảo nơi git repo thật sự nằm trên đĩa, PTY/shell thật sự chạy, agent thật sự thực thi lệnh.

```typescript
DevServer { id, connectionType: 'relay-ssh'|'relay-websocket'|'direct-websocket', status, platform, capabilities }
```

4 loại host, phân biệt qua `Repo.executionHostId`: `'local'` (máy Desktop, không tồn tại ở web
mode), `` ssh:<id> ``, `` devServer:<id> ``, `` runtime:<envId> ``. **Bất biến quan trọng nhất**:
mọi Repo/Worktree/Terminal phải thuộc về đúng **1 host cụ thể**.

### 1.2 Repo — 1 git repository cụ thể, trên 1 host

```typescript
Repo { id, path, executionHostId?, connectionId? }
```

Cặp `(id, executionHostId)` phải duy nhất — vi phạm gây lỗi "ambiguous host" khi xoá/quản lý.

### 1.3 Project (mô hình cũ, per-user, đa-host) — **KHÔNG có RBAC**

Gom nhiều bản sao "cùng một dự án" trên nhiều host thành 1 identity để UI hiển thị 1 card. Lưu
**per-user trong file JSON** (`orca-data.json` của từng user) — không phải bảng SQL dùng chung.

```typescript
Project { id, displayName, badgeColor, sourceRepoIds: string[] }
ProjectHostSetup { projectId, repoId, hostId, setupState }  // cầu nối Project ↔ Repo ↔ Host
```

Vì lưu per-user, **không có khái niệm chia sẻ giữa user khác nhau** ở tầng này — ai tạo, chỉ
người đó (đúng hơn: chỉ tiến trình đọc file của user đó) thấy được.

### `Project` vs `OrcaProject` — vì sao tồn tại cả 2, và có nên gộp không

Hai model này **giải quyết 2 bài toán khác nhau, không phải 1 bài toán làm 2 lần**:

| | `Project` (mục 1.3) | `OrcaProject` (mục 1.7) |
|---|---|---|
| Ra đời để giải quyết | Người dùng Desktop (local, 1 người) mở cùng 1 dự án trên nhiều máy/dev-server khác nhau — cần 1 identity duy nhất để UI không hiện trùng card | Nhiều người cùng cộng tác trên **đúng 1** dev-server — cần biết ai được xem/sửa gì |
| Lưu ở đâu | File JSON per-user (`orca-data.json` trên Desktop; blob Postgres theo `(tenant_id, user_id)` trên `backend` — xem cảnh báo bên dưới) | Bảng SQL dùng chung `orca_v5_projects`/`orca_v5_project_members` |
| Phạm vi host | **Đa-host** — 1 Project có repo trên N dev-server cùng lúc (`sourceRepoIds[]`) | **Đúng 1** dev-server (`devServerId` + `repoPath`) |
| Phân quyền (RBAC) | **Không có** — ẩn với mọi user khác ngoài chủ file | **Có** — `ProjectMember.role` (owner/member/viewer) + `visibility` |
| Consumer thật | Sidebar/Settings, toàn bộ UI Desktop đang chạy | Hệ Task (`orca_tasks.project_id` FK thật); UI consumer (`WorkspaceLayout` và ~14 component) hiện chưa được `App.tsx` mount |
| Quan hệ với Repo/Worktree | Chính là cha trực tiếp (`sourceRepoIds`) | **Không có** — không FK, không tham chiếu `Repo.id`/`Worktree.id` nào |

**Vì sao không gộp làm 1:** `Project` được thiết kế cho use case "local-first, 1 người, nhiều máy" —
không cần identity dùng chung, không cần server, không cần RBAC. `OrcaProject` được thiết kế cho
use case ngược lại: "1 host cố định, nhiều người, cần kiểm soát ai làm gì" — bản chất đòi hỏi 1
bảng SQL dùng chung mà nhiều tiến trình/nhiều user cùng đọc-ghi an toàn. Gộp phẳng 2 model nghĩa
là phải hy sinh 1 trong 2 thuộc tính cốt lõi: hoặc mất khả năng đa-host của `Project` (di chuyển
toàn bộ dữ liệu per-user sang SQL), hoặc mất RBAC thật của `OrcaProject` (hạ nó xuống per-user
JSON không chia sẻ được). Đây là lý do mục 5 (đề xuất `OrcaProjectSourceProject`) chọn hướng
**nối 2 model** (1 bảng cầu nối mỏng) thay vì gộp — giữ nguyên cả 2 thuộc tính, chỉ thêm 1 tầng
chia sẻ ở trên.

### 1.4 Worktree ("Workspace") — 1 checkout/branch cụ thể trong 1 Repo

Đơn vị người dùng thực sự làm việc — mỗi worktree có file explorer, git panel, terminal riêng.

```typescript
Worktree { id: `${repoId}::${path}`, repoId, projectId, hostId, isMainWorktree: boolean }
```

### 1.5 FolderWorkspace — biến thể không cần git

Chỉ 1 thư mục thường (không phải git repo), thuộc về 1 **Project Group** thay vì 1 Repo. Dùng khi
muốn Orca quản lý 1 folder không phải git.

### 1.6 Terminal (PTY session) — tầng thấp nhất

```
Tab { worktreeId } → PTY chạy trên hostId của worktree đó → Dev Server thực thi lệnh thật
```

### 1.7 OrcaProject (mô hình mới, v5.0, SQL dùng chung) — **CÓ RBAC thật**

Gắn **cứng đúng 1 Dev Server** (`devServerId` + `repoPath`), khác hẳn `Project` cũ (đa-host).
Backend hoàn chỉnh, đang chạy thật: `ProjectService.ts` + `ProjectServerRouter.ts`, bảng SQL thật
`orca_v5_projects`/`orca_v5_project_members`, migration `0007_projects.ts`, có `assertAccess()`
kiểm tra quyền thật trước mọi thao tác.

```typescript
OrcaProject { id, devServerId, repoPath, visibility: 'private'|'team'|'company', ... }
ProjectMember { projectId, userId, role: 'owner'|'member'|'viewer' }
```

**⚠️ Đính chính (2026-09-01):** Ghi chú trước đó ("KHÔNG có quan hệ với Repo") chỉ còn đúng với
`Repo` cũ (mục 1.2, per-user JSON). Thực tế `OrcaProject` giờ **có Repo riêng của nó** qua 1
entity thứ 3, hoàn toàn khác: Go proto `orca.project.v1.Repo { id, projectId, url, displayName,
position }` (`backend-go/services/project-service`), FK thật `projectId → OrcaProject.id`. RPC
`repo.add`/`repo.list({projectId})` khi `projectId` là 1 `OrcaProject.id` sẽ tạo/đọc đúng Repo này
— **không đụng gì tới `Repo` cũ (mục 1.2) hay `orca_projects`/`orca_repos` (mục 7 cảnh báo)**. Tạo
OrcaProject rồi gọi `repo.add` **không** tự động tạo hay liên kết 1 `Project` cũ nào — 2 luồng độc
lập hoàn toàn (xem flow thật trong `CreateProjectDialog.tsx`: `project.create` →
`project.rebindDevServer` → `repo.add`, cả 3 đều không chạm `Project`/`Repo` cũ).

FK thật còn lại vào `orca_v5_projects` đến từ hệ Task (`orca_tasks.project_id`). Việc **nối với
`Project` cũ (đa-host, per-user JSON)** vẫn là 1 cơ chế tách biệt, có chủ đích, không tự động —
xem mục 5 (`OrcaProjectSourceProject`, **đã implement ở backend**, chưa có UI).

### 1.8 Task (Task Graph) — cây công việc, gắn `OrcaProject`

```typescript
OrcaTask { id, projectId?, parentId?, title, type, status, priority, labels, visibility,
           reporterId?, assigneeId?, estimatedHours?, progressPercent, aiContext?,
           promptTemplate?, dueDate? }
```

Task **có FK thật** `project_id → orca_v5_projects.id`. Task có hệ phân quyền riêng
(`orca_task_grants`), tách biệt hoàn toàn khỏi `ProjectMember` — xem mục 4.4. Task có cây quan
hệ cha-con (`parentId`) + cạnh phụ thuộc (`orca_task_edges`), và có thể sinh ra Worktree/Agent
session khi thực thi.

### Sơ đồ 2 mô hình song song

```
── Mô hình cũ (per-user JSON, đa-host, KHÔNG RBAC) ──
Dev Server ── Repo (executionHostId) ── Worktree (repoId) ── Terminal/PTY
                  ▲
                  │ sourceRepoIds[]
              Project (per-user, đa-host, KHÔNG chia sẻ được)

── Mô hình mới (SQL dùng chung, 1 project = 1 dev-server, CÓ RBAC) ──
OrcaProject (devServerId, repoPath, visibility) ──ProjectMember(owner/member/viewer)──► User
     ▲
     │ project_id FK (thật)
   OrcaTask (parentId cây, orca_task_edges phụ thuộc) ──orca_task_grants──► User/Team/Company

  ⚠️ KHÔNG có cầu nối giữa 2 mô hình — OrcaProject không biết gì về Repo/Worktree thật
```

---

## 2. Tổ chức người dùng (Identity hierarchy)

```
Company (root — entity thật, bảng orca_companies)
  │  security section LOCKED — Dept/User không override được
  └── Department (entity thật, bảng orca_departments, cây)
        │  team_lead_id, override AI model/fleet tags/shared env
        └── Team (⚠️ CHƯA xây — chỉ có bảng nối rỗng orca_team_members,
            │      chưa có bảng metadata tên/mô tả, KHÔNG có RPC team.* để tạo/thêm thành viên,
            │      cắt ngang qua Department, 1 user có thể ở nhiều Team)
            └── User (OrcaUser — role TOÀN CỤC: developer|lead|admin, KHÔNG theo từng project)
```

**Trạng thái thật từng tầng:**

| Tầng | Trạng thái | Ghi chú |
|---|---|---|
| Company | ✅ Entity thật (`orca_companies`) | `ProfileService.createCompany/getCompanyProfile/setCompanyProfile` |
| Department | ✅ Entity thật (`orca_departments`, cây), cột `department_id` trên `orca_users` | `createDepartment/getDeptProfile/setDeptProfile/setUserDepartment` |
| Team | ❌ Chưa có bảng metadata — chỉ bảng nối `orca_team_members(team_id, user_id, role)` rỗng, không ai `INSERT` vào | Nhánh grant `'team'` trong `TaskGrantService` hiện là **dead code** vì bảng luôn rỗng |
| User (role toàn cục) | ✅ `orca_users.role`: `developer`\|`lead`\|`admin` | Không phải role theo từng project — xem mục 3 |

**Profile inheritance** (không phải quyền truy cập tài sản, mà là cấu hình mặc định kế thừa):
`resolveProfile(userId)` deep-merge Company → Department → User; field `security` bị
company-lock hoàn toàn (user/dept không override được dù merge).

---

## 3. Các hệ RBAC độc lập tồn tại song song — bảng tổng hợp

**⚠️ Gap đã biết** (`docs/hld/v1/security.md` §8.3): RBAC phân mảnh trên **~4-5 cơ chế độc lập,
không đồng nhất**, không có 1 hàm `hasPermission(role, resource, action)` duy nhất làm nguồn chân
lý (BUG-BE-HLD-003, còn mở):

| # | Hệ RBAC | Áp dụng cho tài sản | Đơn vị cấp quyền | Trạng thái |
|---|---|---|---|---|
| 1 | `RolePolicy`/`hasPermission()` tag-based (F32) | SSH host, Fleet, `orca_access_policies` | Role toàn cục (`developer`/`lead`/`admin`) + string tag `project`/`team` tự do trên `SshTarget` | ✅ Phase 1 implemented |
| 2 | HTTP `requireAdmin` middleware | `/admin/api/*` (user CRUD, session kill, audit) | Role toàn cục `admin` | ✅ Implemented, đúng từ đầu |
| 3 | RPC `requireAdmin(ctx)` | `profile.updateCompany/updateDept/createCompany/createDept/setUserDept` | Role toàn cục `admin` | ✅ Đã vá lỗi bypass 2026-08-09 (trước đó chỉ check đã login) |
| 4 | RPC `requireOwnerOrAdmin(...)` | `OrcaProject` ownership check (mô hình mới) | Chủ sở hữu project hoặc admin | ✅ Đã vá cùng đợt 2026-08-09 |
| 5 | `ProjectMember.role` + `assertAccess()` | `OrcaProject` (mô hình mới, mục 1.7) | `owner`/`member`/`viewer` **theo từng project** | ✅ Backend thật, đang chạy |
| 6 | `TaskGrantService`/`orca_task_grants` | `OrcaTask` (mục 1.8) | `view<comment<edit<execute<manage`, scope `user`/`team`/`company`/`public_link` | ✅ Backend thật, đang chạy — **độc lập hoàn toàn** với #5 |
| — | Project/Repo/Worktree (mô hình cũ, mục 1.2–1.4) | Repo/Worktree per-user | **Không có RBAC** — file JSON per-user, ai sở hữu tiến trình mới đọc được | Chưa có cơ chế share |

**Quyết định kiến trúc đã chốt (2026-08-13):** #5 (`ProjectMember`) và #6 (`TaskGrantService`)
**cố tình giữ tách biệt, không hợp nhất** — dù ban đầu có đề xuất dùng chung 1 hệ. Lý do: 2 hệ đã
được xây độc lập, đầy đủ, đang chạy thật trước khi có quyết định; hợp nhất tốn kém hơn lợi ích.
Hệ quả: nếu sau này xây RBAC cho tài sản mới, **phải chọn rõ đi theo hệ nào** (đơn giản kiểu
owner/member/viewer → theo #5; cần chia sẻ linh hoạt theo cây/scope/apply_tree → theo #6) — không
tạo thêm hệ RBAC thứ 7 cho cùng khái niệm "ai được làm gì trên tài sản nào".

---

## 4. Chi tiết phân quyền theo từng loại tài sản

### 4.1 SSH Host / Fleet (F32 — Phase 1)

```typescript
interface RolePolicy {
  role: 'developer' | 'lead' | 'admin'
  resource: 'ssh_host' | 'fleet' | 'worktree' | 'admin_panel' | 'credentials'
  actions: ('read' | 'write' | 'delete' | 'admin')[]
}
```

| Role | ssh_host | worktree | fleet | admin_panel |
|---|---|---|---|---|
| `developer` | read (servers của project mình) | read, write | — | — |
| `lead` | read, write | read, write, delete | read, write | read |
| `admin` | read/write/delete/admin | read/write/delete/admin | read/write/delete/admin | read/write/delete/admin |

Lọc hiển thị dựa trên `orca_access_policies(user_id, project_id, server_id, role, expires_at)` —
`project_id`/`server_id` NULL nghĩa là áp dụng cho tất cả. Mọi request đi qua RPC middleware
kiểm tra `hasPermission()` trước khi thực thi, ghi `orca_audit_log` với `outcome: 'allowed'|'denied'`.

### 4.2 Project/Repo/Worktree (mô hình cũ) — chưa có phân quyền

Vì lưu **per-user trong JSON**, không có khái niệm "chia sẻ Project X cho user Y" ở tầng này.
Đây chính là lý do mục 5 đề xuất xây 1 tầng chia sẻ riêng thay vì chờ RBAC tự nhiên xuất hiện ở
đây.

### 4.3 OrcaProject (mô hình mới, v5.0) — `ProjectMember`

| Hành động | `viewer` | `member` | `owner` |
|---|:---:|:---:|:---:|
| Xem file explorer, git status, terminal output | ✅ | ✅ | ✅ |
| Tạo worktree mới, mở terminal, chạy agent | ❌ | ✅ | ✅ |
| Thêm/xoá `Project` khỏi `OrcaProject` (sau khi tích hợp, mục 5) | ❌ | ❌ | ✅ |
| Thêm/xoá member, đổi role người khác | ❌ | ❌ | ✅ |
| Đổi `visibility`, xoá `OrcaProject` | ❌ | ❌ | ✅ |

`visibility` (`private`/`team`/`company`) kiểm soát **ai nhìn thấy project tồn tại** (trước cả
khi xét role); `ProjectMember.role` kiểm soát **được làm gì** sau khi đã thấy/được thêm vào.
`assertAccess()` (`ProjectService.ts`) là điểm kiểm tra tập trung duy nhất trước mọi RPC
`project.*`.

**Bug đã phát hiện (đã fix 2026-08-09):** `requireOwnerOrAdmin` từng chỉ kiểm tra "đã login",
không kiểm tra đúng owner/admin — cho phép override ownership check. Đã vá bằng cách resolve role
thật qua `AuthUserStore.getUserRole(ctx.userId)`.

**Bug đang mở:** `project.agentSpawn` luôn lỗi `AGENT_SPAWNER_NOT_AVAILABLE` vì
`server-bootstrap.ts` đăng ký method thiếu tham số `agentSpawner` — không liên quan RBAC, nhưng
chặn hoàn toàn việc dùng quyền `member`/`owner` để "chạy agent" qua đường Project.

### 4.4 OrcaTask — `TaskGrantService` (BL-TG-03)

**5 cấp quyền, tăng dần, cấp sau bao gồm cấp trước:**

```
view < comment < edit < execute < manage
```

| Permission | Cho phép |
|---|---|
| `view` | Đọc task, subtask, comment, activity |
| `comment` | + thêm/sửa comment của chính mình |
| `edit` | + sửa title/status/labels/estimate/assignee |
| `execute` | + chạy Agent, tạo Worktree gắn với task |
| `manage` | + grant/revoke quyền cho người khác, xoá task, share cả cây |

**Thuật toán resolve quyền** (`hasTaskAccess`):

```
1. task.ownerId === userId               → luôn full manage
2. user.role === 'admin'                 → luôn full access (toàn org)
3. Direct grant trên chính task này      (scope: user | team | company)
4. Grant kế thừa từ task tổ tiên có apply_tree=true (đi ngược parentId)
5. Lấy permission cao nhất trong các grant còn hiệu lực (chưa expiresAt)
6. So permission đó với permission yêu cầu theo thứ tự view<comment<edit<execute<manage
```

**Scope của 1 grant:** `user` (1 người cụ thể), `team` (theo `departmentId` — lưu ý: nhánh
`'team'` hiện là dead code vì bảng Team chưa có ai ghi, xem mục 2), `company` (mọi user trong
org), `public_link` (token ngẫu nhiên, ai có link cũng xem được, không cần login, chỉ quyền
`view`).

**Kế thừa theo cây (`apply_tree`):**
```
Epic [grant: company, view, apply_tree=true]
  ├── Story A [thừa hưởng: company view]
  └── Story B [owner thêm: user@x.com, execute]  ← grant bổ sung, không thay grant cũ
        └── Task B1 [thừa hưởng CẢ HAI: company view + user@x.com execute]
```

Revoke = xoá row `orca_task_grants` → có hiệu lực ngay, không cần cache invalidation.

### 4.5 Profile (Company/Department/User)

| API | Yêu cầu quyền |
|---|---|
| `profile.company.update` (`profile.updateCompany`/`createCompany`) | role `admin` |
| `profile.dept.update` (`profile.updateDept`/`createDept`) | role `admin` (tài liệu gốc ghi "admin hoặc team-lead" nhưng code thật hiện chỉ check `admin`) |
| `profile.user.update` | chính user đó, không được sửa field bị lock |
| `profile.resolve(userId)` | chính session của `userId`, hoặc admin |

Field `security` (approvedModels, disallowedCommands, require2FA, sessionTimeoutHours) bị
**company-lock hoàn toàn** — Department/User không override được dù merge, kể cả khi họ có
quyền `edit` ở tầng khác.

---

## 5. Tích hợp mô hình cũ (Project/Repo) với mô hình mới (OrcaProject) — ✅ ĐÃ IMPLEMENT ở backend, ⚠️ CHƯA có UI

**Đính chính (2026-09-01):** mục này trước đây ghi "đề xuất, chưa triển khai" — **sai, đã lỗi
thời**. Đã xác nhận bằng code thật: `backend/src/main/project/OrcaProjectSourceProjectService.ts`
+ `backend/src/main/project/orca-project-sharing-rpc-handler.ts`, có test
(`orca-project-sharing-rpc-handler.test.ts`), comment trong code ghi "wiring done by the Wave 3
integration agent" — tức đã được tích hợp vào `ALL_RPC_METHODS` lúc bootstrap. Đây KHÔNG phải
gộp phẳng 2 model — đúng như thiết kế ban đầu, `OrcaProject` là **tầng SỞ HỮU + CHIA SẺ** phía
trên, không tự lưu `sourceRepoIds` của `Project` cũ:

```
OrcaProject (SQL, dùng chung)  id, name, visibility, members[] (owner/member/viewer)
   │ 1..N — 1 OrcaProject có thể gộp nhiều Project hiện có (vd: nhiều repo của cùng 1 team)
   ▼
Project (per-user JSON — GIỮ NGUYÊN)  id, displayName, sourceRepoIds[]
   │ 1..N — đa-host, GIỮ NGUYÊN
   ▼
Repo (executionHostId)  →  Worktree (repoId)  →  Terminal/PTY
```

**Bảng nối thật đang chạy** — `orca_project_source_projects` (qua `OrcaProjectSourceProjectService`):

```typescript
// SourceProjectRef — shape trả về, khớp cột thật (orca_project_id, owner_user_id, project_id, created_at)
SourceProjectRef {
  ownerUserId: string     // user thật sự sở hữu file JSON chứa Project gốc — LUÔN = ctx.userId lúc link
  projectId: string       // FK logic -> Project.id trong JSON của ownerUserId
}
```

**4 RPC method thật** (`orca-project-sharing-rpc-handler.ts`):

| RPC | Ai gọi được | Việc làm |
|---|---|---|
| `orcaProjects.linkSourceProject({orcaProjectId, projectId})` | Bất kỳ member nào (mọi role) của `orcaProjectId` | Nối 1 `Project` **CỦA CHÍNH MÌNH** (ownerUserId luôn = `ctx.userId`, client không truyền được — chống link hộ) vào OrcaProject. **Không tự tạo Project mới** — `projectId` phải là 1 Project đã có sẵn |
| `orcaProjects.unlinkSourceProject({orcaProjectId, projectId})` | Chỉ `owner`/admin (`requireOwnerOrAdmin`) | Gỡ liên kết, idempotent (không lỗi nếu chưa từng link) |
| `orcaProjects.getProjectData({orcaProjectId, projectId})` | Bất kỳ member nào của `orcaProjectId`, **và** `projectId` phải đang được link vào đúng `orcaProjectId` đó | Đọc file JSON của `ownerUserId`, lọc đúng `Project`/`Repo`/`WorktreeMeta` thuộc `projectId` (`filterOwnerProjectData()`) — không bao giờ trả nguyên file |
| `orcaProjects.list()` | User đã login | Liệt kê mọi OrcaProject caller là member, kèm `sourceProjects` (danh sách Project đã link) của mỗi cái |

**Luồng đọc-chéo-user thật (B xem Project của A qua OrcaProject chung):**

```
1. A tạo Project P (như bình thường, vẫn trong JSON của A) — KHÔNG liên quan gì OrcaProject
2. A gọi orcaProjects.linkSourceProject({orcaProjectId, projectId: P.id})
   → ghi orca_project_source_projects(orca_project_id, owner_user_id=A, project_id=P.id)
3. A thêm B làm 'member' của OrcaProject đó (qua project.addMember, xem mục 4.3)
4. B gọi orcaProjects.list() → thấy OrcaProject, kèm sourceProjects=[{ownerUserId: A, projectId: P.id}]
5. B gọi orcaProjects.getProjectData({orcaProjectId, projectId: P.id})
   → assertOrcaProjectMemberForRead(B) → kiểm tra P.id có nằm trong sourceProjects đã link không
   → đọc file JSON của A, filterOwnerProjectData() → chỉ trả đúng Project P + repo của nó
6. B mở terminal trên worktree thuộc P → PTY vẫn chạy trên host của worktree đó (không đổi tầng
   thực thi) → có audit span (`Tracers.orcaProjectSharingFlow`) ghi orcaProjectId/actingUserId/
   ownerUserId/projectId
```

**⚠️ Chưa có ở giao diện — xác nhận bằng grep (0 kết quả) trong `frontend/src`, `desktop/src` cho
`linkSourceProject`/`unlinkSourceProject`/`orcaProjects.list`/`orcaProjects.getProjectData`.**
Không có dialog/nút "Share Project" hay "Link existing Project" nào trong app — cơ chế này hiện
chỉ gọi được qua RPC trực tiếp (API/CLI/test), giống pattern "backend xong, UI mồ côi" đã thấy ở
RBAC cho `OrcaProject`/`Task` (mục 7). Ai muốn dùng tính năng này hôm nay phải tự gọi RPC bằng
tay — chưa có cách nào từ UI để: (a) chọn 1 Project sẵn có để link vào 1 OrcaProject, (b) xem
danh sách Project đã link, hay (c) unlink.

**Lưu ý quan trọng cho Q "thêm repo vào OrcaProject có tự tạo Project không":** **Không.**
`repo.add({projectId: orcaProjectId, ...})` (mục 1.7) và `orcaProjects.linkSourceProject(...)` là
**2 cơ chế hoàn toàn độc lập, không cơ chế nào tự động gọi cơ chế kia**:
- `repo.add` tạo 1 Repo **mới**, thuộc kiểu Go-native (`orca.project.v1.Repo`), gắn thẳng
  `projectId → OrcaProject.id` — không sinh ra, không cần, không đụng tới bất kỳ `Project` cũ nào.
- `linkSourceProject` chỉ nối 1 `Project` cũ **đã tồn tại sẵn** (đa-host, per-user JSON) vào
  OrcaProject để chia sẻ — không tạo Project mới, không liên quan gì tới việc thêm Repo qua
  `repo.add`.

Vẫn đúng cảnh báo cũ: **không bao giờ trả nguyên file JSON của A** — `getProjectData` đã tuân thủ
đúng nguyên tắc lọc này qua `filterOwnerProjectData()`.

**2 bug thật đã ghi nhận từ chính 2 điểm ở trên** — xem chi tiết + đề xuất fix:
[BUG-BE-PROJECT-001](../bugs/bug-be-project-001-project-orcaproject-no-bridge-ux.md) (không có
cầu nối/gợi ý ở UI khi `repoPath` trùng 1 Repo đã có sẵn trong sidebar → dữ liệu phân mảnh không
kiểm soát được) và [BUG-BE-PROJECT-002](../bugs/bug-be-project-002-orcaprojectsourceproject-no-ui.md)
(4 RPC sharing ở trên hoàn chỉnh nhưng 0 UI nào gọi tới).

---

## 6. Bảng tổng hợp nhanh: "Ai được làm gì trên tài sản nào?"

| Tài sản | Đơn vị cấp quyền | Role/scope | Nơi enforce |
|---|---|---|---|
| SSH Host / Fleet server | Role toàn cục + tag `project`/`team` tự do | developer/lead/admin | RPC middleware `hasPermission()` |
| Project/Repo/Worktree (cũ) | — (chưa có) | — | — (per-user JSON, ẩn với user khác) |
| OrcaProject (mới) + Repo riêng của nó (`repo.add`, Go-native) | `ProjectMember` theo từng project | owner/member/viewer + `visibility` | `assertAccess()` trong `ProjectService.ts` |
| Liên kết Project cũ ↔ OrcaProject (`orcaProjects.linkSourceProject`) | `ProjectMember` (link) + chính chủ owner Project (`ownerUserId=ctx.userId`) | member trở lên để link; owner/admin để unlink | `orca-project-sharing-rpc-handler.ts`, **chưa có UI** |
| OrcaTask | `orca_task_grants` theo từng task, kế thừa cây | view<comment<edit<execute<manage, scope user/team/company/public_link | `TaskGrantService.hasTaskAccess()` |
| Company/Department profile | Role toàn cục | admin (dept: tài liệu ghi thêm team-lead, code thật chỉ check admin) | RPC `requireAdmin(ctx)` |
| User profile (cá nhân) | Chính chủ | — | So `ctx.userId === targetUserId` |
| `/admin/api/*` (user CRUD, session, audit) | Role toàn cục | admin | HTTP `requireAdmin` middleware |

---

## 7. Khoảng trống & mâu thuẫn đã ghi nhận

- **RBAC phân mảnh 4-5 hệ độc lập** (mục 3), không có `hasPermission(role, resource, action)`
  duy nhất — rủi ro thêm 1 chỗ check sai/thiếu khi mở rộng (đã từng xảy ra thật:
  BUG-BE-HLD-001/002).
- **Team chưa tồn tại như entity thật** — mọi scope `'team'` trong `TaskGrantService` hiện là
  dead code. Cần xây bảng metadata `Team{id, name}` + RPC `team.*` trước khi scope này dùng được.
- **Project/Repo/Worktree (mô hình cũ) hoàn toàn chưa chia sẻ được giữa user** — không phải thiếu
  sót nhỏ mà là giới hạn kiến trúc (lưu trữ per-user JSON). Mục 5 là hướng giải quyết đã được
  thiết kế nhưng **chưa triển khai**.
- **`ProjectMember` và `TaskGrantService` cố tình không hợp nhất** — quyết định kiến trúc đã
  chốt, không phải nợ kỹ thuật cần dọn.
- **`project.agentSpawn` luôn lỗi** (thiếu tham số khi đăng ký RPC) — chặn hành động `execute`
  cấp bởi quyền `member`/`owner` trên OrcaProject dù RBAC bản thân đúng.
- **RBAC cho `OrcaProject`/`Task` không có UI thật để cấu hình** — cụm component consumer
  (`WorkspaceLayout.tsx` và ~14 component khác) không được `App.tsx` import, nên toàn bộ luồng
  "xem/gán quyền project" phía người dùng cuối hiện chưa thể chạm tới qua UI, dù backend hoạt
  động đúng qua RPC trực tiếp.
- **⚠️ Phát hiện an toàn (2026-09-01): KHÔNG nối `SqlStateRepository` vào luồng sống nếu chưa
  thêm tenant/user scoping.** Có 1 abstraction thứ 3 (ngoài `Store`/JSON-blob và `OrcaProject`/SQL
  chuẩn) đang nằm chết: `IStateRepository`/`SqlStateRepository` (`backend/src/main/repositories/
  sql-repository.ts`) với bảng chuẩn hoá thật `orca_projects`/`orca_repos`
  (`backend/src/main/db/migrations/0004_orca_app_tables.ts`) — được tạo ở
  `server-bootstrap.ts` nhưng kết quả bị bỏ ngay (`void stateRepo`), không có consumer thật nào.
  Lý do nó vẫn nằm chết, không phải nên "hoàn thiện cho xong":
  - **`orca_projects`/`orca_repos` không có cột `tenant_id`/`user_id`** — 2 bảng toàn cục dùng
    chung. Luồng sống thật (`Store.hydrateFromPostgres()` + `PgOrcaDataStatePersistence`, bảng
    `orca_data_state_blob`) scope đúng theo `(tenant_id, user_id)` qua khoá dòng. Nối
    `SqlStateRepository` vào thay thế **sẽ khiến mọi tenant/mọi user đọc-ghi chung 1 tập row** —
    tái hiện đúng lớp bug đã vá ngày 2026-08-16 ("mọi process mặc định `userId=''`, user thật âm
    thầm ghi đè lẫn nhau"), lần này không sửa được bằng code vì thiếu hẳn cột để scope.
  - `Project` (mô hình cũ) phần lớn là **view suy ra (derived) từ `Repo`**, không phải entity độc
    lập — xem `Store.syncProjectHostSetupCompatibilityState()` (`persistence.ts:4425`) và
    `removeProject()` (`persistence.ts:4230`, xoá `state.repos` dù tên hàm nói "Project"). 2 entity
    liên quan (`ProjectHostSetup`, `ProjectGroup`) hoàn toàn chưa có bảng SQL nào.
    `SqlStateRepository` còn giả định `Repo.projectId` — field này **không tồn tại** trên type
    `Repo` thật (`backend/src/shared/types.ts`).
  - Đã được `specs/backend/models/08-postgres-microservices-target-architecture.md:48-55` ghi
    nhận là "ownership undecided, deferred to Phase 1" — tức đây là quyết định treo có chủ đích,
    không phải lỗ hổng chưa ai biết.
  - **Kết luận:** nếu sau này có ai định hoàn thiện hướng này, việc đầu tiên bắt buộc là thêm
    migration bổ sung `tenant_id`/`user_id` (+ index) vào `orca_projects`/`orca_repos` và sửa mọi
    query trong `sql-repository.ts` để luôn `WHERE tenant_id = ? AND user_id = ?` — trước khi bàn
    đến việc thay thế `Store` ở bất kỳ RPC nào.
- **`Project` (cũ) và `OrcaProject` không có cầu nối nào tới được UI** — 2 bug thật, xem
  [BUG-BE-PROJECT-001](../bugs/bug-be-project-001-project-orcaproject-no-bridge-ux.md) (tạo
  OrcaProject + `repo.add` không cảnh báo/không gợi ý link khi trùng repo đã có sẵn ở sidebar) và
  [BUG-BE-PROJECT-002](../bugs/bug-be-project-002-orcaprojectsourceproject-no-ui.md)
  (`orcaProjects.linkSourceProject`/`unlinkSourceProject`/`getProjectData`/`list` hoàn chỉnh ở
  backend, 0 UI nào gọi tới).

---

## 8. Nguồn tài liệu

| File | Nội dung |
|---|---|
| [`docs/guides/project-workspace/terminal-workspace-project-devserver-architecture.md`](../project-workspace/terminal-workspace-project-devserver-architecture.md) | Định nghĩa Dev Server/Repo/Project/Worktree/Terminal, 2 mô hình project song song |
| [`docs/guides/profile/user-profile-team-department-rbac.md`](../profile/user-profile-team-department-rbac.md) | Company/Department/Team, RBAC cho Project/OrcaProject |
| [`docs/guides/task-automation/task-automation-orchestration-integration.md`](../task-automation/task-automation-orchestration-integration.md) | `OrcaTask` shape thật, quyết định tách `ProjectMember`/`TaskGrantService` |
| [`docs/logic/task-graph/BL-TG-03-task-access-control.md`](../../logic/task-graph/BL-TG-03-task-access-control.md) | Thuật toán `hasTaskAccess`, permission levels, apply_tree |
| [`docs/features/F32-team-rbac.md`](../../features/F32-team-rbac.md) | RolePolicy/`hasPermission()` cho SSH host/Fleet |
| [`docs/features/F33-user-profile-hierarchy.md`](../../features/F33-user-profile-hierarchy.md) | Company→Department→User profile inheritance |
| [`docs/features/F37-task-graph-management.md`](../../features/F37-task-graph-management.md) | Đặc tả Task Graph (đối chiếu shape thật ở task-automation guide) |
| [`docs/features/F38-project-workspace.md`](../../features/F38-project-workspace.md) | Đặc tả Project Workspace (đối chiếu code thật ở project-workspace guide) |
| [`docs/crs/v1/login/CR-LOGIN-004-admin.md`](../../crs/v1/login/CR-LOGIN-004-admin.md) | Admin UI, Access Policy |
| [`docs/hld/v1/security.md`](../../hld/v1/security.md) §8.3 | Gap RBAC phân mảnh, bug đã vá 2026-08-09 |
| [`docs/guides/planning/audit-backend-agent-2026-08-13.md`](../planning/audit-backend-agent-2026-08-13.md) | Audit đầy đủ file:line cho toàn bộ nhận định trên |
| [`docs/guides/planning/decisions-needed.md`](../planning/decisions-needed.md) | Quyết định đã chốt: Team multi-membership, tách ProjectMember/TaskGrant |
| [`specs/backend/models/08-postgres-microservices-target-architecture.md`](../../../specs/backend/models/08-postgres-microservices-target-architecture.md) | Quyết định treo "ownership undecided" cho `orca_projects`/`orca_repos`, trạng thái cutover `Store`→Postgres qua `orca_data_state_blob` |
| [`specs/backend/models/02-sql-schema-catalog.md`](../../../specs/backend/models/02-sql-schema-catalog.md) | 3 cặp bảng "project/repo" trùng tên khác nghĩa: migration 0001 (dead), 0004 `orca_projects`/`orca_repos` (JSON blob, không tenant-scoped), 0007 `orca_v5_projects` (OrcaProject thật) |
