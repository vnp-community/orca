# User, Profile, Team, Department, và RBAC cho Project/OrcaProject

**Cập nhật:** 2026-08-13 (viết lại toàn bộ sau khi 4 subagent kiểm chứng lại bằng `backend/src/`
thay vì `frontend/src/main/` — xem ghi chú đính chính ở đầu
[terminal-workspace-project-devserver-architecture.md](../project-workspace/terminal-workspace-project-devserver-architecture.md))

> Tài liệu này mô tả identity/tổ chức hiện có trong code, chỉ ra khoảng trống, và đề xuất mô
> hình RBAC cho `Project`/`OrcaProject`. Chi tiết audit đầy đủ (file:line) xem
> [audit-backend-agent-2026-08-13.md](../planning/audit-backend-agent-2026-08-13.md) mục A. Giải pháp
> từng bug xem [fix-proposals-per-issue.md](../planning/fix-proposals-per-issue.md) nhóm A. Kế hoạch thực
> thi xem [roadmap-orca-project-task-rbac.md](../planning/roadmap-orca-project-task-rbac.md).

## 1. `OrcaUser` — identity + role toàn cục (đang chạy thật)

```typescript
// backend/src/shared/rbac-types.ts
OrcaUser {
  id, email, name, avatarUrl
  role: 'developer' | 'lead' | 'admin'   // role TOÀN CỤC — không theo từng project
  teams: string[]                          // chuỗi tag tự do, KHÔNG phải FK tới 1 entity Team
}
```

Frontend có 1 pattern RBAC dùng field này — `selectSshTargetsForCurrentUser` (`store/selectors.ts`):

```typescript
if (!user || user.role === 'admin') return all        // admin/chưa login → thấy hết
return all.filter(target => {
  if (target.project && !user.projects.includes(target.project)) return false
  if (target.team && !user.teams.includes(target.team)) return false
  return true
})
```

`SshTarget.project?: string` / `SshTarget.team?: string` cũng chỉ là **chuỗi tag tự do** — so
khớp string với string, không có bảng `Team`/`Department` thật đứng sau (`backend/src/shared/
rbac-types.ts`'s `OrcaAccessPolicy` cũng dùng đúng kiểu tag tự do này cho SSH/fleet, không phải
profile hierarchy). Đây là kiểu RBAC "nhẹ", đủ dùng cho SSH targets nhưng không đủ chặt cho
Project/OrcaProject.

## 2. Profile system — backend THẬT, đang chạy — nhưng UI thật đang dùng bản type lệch xa nhất

### Backend (`backend/src/main/profile/`) — REAL & LIVE, kiểm chứng qua `server-bootstrap.ts`

- **`ProfileService.ts`** (200 dòng) — CRUD thật trên 3 bảng SQL từ migration `0006_company_dept.ts`:
  `orca_companies`, `orca_departments`, `orca_user_profiles`, cột `department_id` trên `orca_users`.
  `createCompany`/`getCompanyProfile`/`setCompanyProfile`/`createDepartment`/`getDeptProfile`/
  `setDeptProfile`/`getUserProfile`/`setUserProfile`/`getCompanyProfileForUser`/
  `getDeptProfileForUser`/`setUserDepartment` — đều thật, có JOIN thật.
- **`ProfileResolver.ts`** (321 dòng) — merge 3 lớp Company→Dept→User, cache TTL 60s.
  `security` bị company-lock hoàn toàn (không override được); `agent`/`editor` merge theo field;
  `shell` merge phức hợp (`pathAdditions` nối, `envVars` merge); `mcp.servers` dedupe theo tên.
- **`profile-rpc-handler.ts`** — đăng ký đúng các method: `profile.getResolved`,
  `profile.getUserProfile`, `profile.updateUser`, `profile.getCompany`, `profile.updateCompany`,
  `profile.updateDept`, `profile.invalidate`, `profile.setUserDept`, `profile.createCompany`,
  `profile.createDept`. Method sửa company/dept đều gọi `requireAdmin()` — kiểm tra role thật,
  không phải no-op.
- Xác nhận wiring qua `server-bootstrap.ts`: `ProfileService`/`ProfileResolver` được khởi tạo và
  `rpcServer.addMethods(createProfileMethods(...))` chạy thật lúc server boot.

### 🐛 2 bug thật đang tồn tại trong production — UI gọi sai tên RPC method

- `frontend/src/renderer/src/hooks/useProfile.ts` gọi **`profile.getUser`** — **method này không
  tồn tại** trên backend (tên đúng là `profile.getUserProfile`). Lỗi bị che giấu vì test của hook
  này mock RPC layer và tự chế response cho đúng tên sai đó, nên không bao giờ chạm backend thật
  để lộ ra.
- `CompanyProfileAdmin.tsx`/`DeptProfileAdmin.tsx` gọi **`profile.listDepts`** — **method này
  cũng không tồn tại** ở bất kỳ đâu trong `backend/src`.

### 🐛 Bug thật thứ 3 — trang Admin build ra nhưng không bao giờ serve được

`frontend/vite.config.ts` build thật 1 entry point thứ 2, `admin-index.html` (khác
`web-index.html`) — file này tồn tại thật trong `frontend/out/web/admin-index.html`. Nhưng
`backend/src/server/http-server.ts` có đoạn:
```typescript
if (path.startsWith('/auth') || path.startsWith('/admin')) {
  app(req, res)   // route thẳng vào Express app TRƯỚC khi tới static-file fallback
  return
}
```
Express app chỉ mount `/admin/api/*` (REST thật, có `requireAdmin`, hoạt động đúng) — **không có
route nào serve `/admin` hay `/admin-index.html`**. Mọi request `GET /admin`/`/admin-index.html`
rơi vào 404 mặc định của Express. **Trang Admin SPA không bao giờ mở được qua HTTP**, bất kể đã
đăng nhập admin hay chưa — và cũng không có link/route nào trong app chính dẫn tới đó. Tầng
API `/admin/api/*` thì thật và hoạt động đúng — chỉ có phần HTML shell là không serve được.

Ngoài ra `DeptProfileAdmin.tsx` không được `AdminApp.tsx` import vào router của nó (chỉ có
`CompanyProfileAdmin` cho route `/profile`) — mồ côi kể cả bên trong chính app Admin.

## 3. `Team` — KHÔNG tồn tại như entity thật, chỉ có 1 bảng rỗng chưa ai dùng

- **Không có bảng `orca_teams`** ở bất kỳ migration nào.
- **Có bảng `orca_team_members`** (tạo bởi `0010_tasks.ts`, cùng migration tạo hệ Task):
  `(team_id TEXT, user_id TEXT, role TEXT, added_at)`, PK `(team_id, user_id)` — nhưng `team_id`
  là chuỗi trơn, **không có bảng metadata nào** lưu tên/mô tả team.
  - Người dùng duy nhất: `TaskGrantService.ts` — SELECT read-only để resolve grant scope
    `'team'` cho Task (xem [task-automation-orchestration-integration.md](../task-automation/task-automation-orchestration-integration.md)).
  - **Không có method/route nào từng `INSERT` vào bảng này** — không có RPC `team.*` để tạo team
    hay thêm thành viên. **Nhánh grant `'team'` trong `TaskGrantService` hiện là dead code** —
    永远 không match được ai vì bảng luôn rỗng.

→ Kết luận (khớp với suy luận ban đầu, dù trước đó tra sai file): **Department có entity thật
(dạng cây), Team thì chưa** — chỉ có 1 bảng rỗng cho tương lai.

## 4. Có **5 bản type Profile/OrcaProfile khác nhau** trong repo, không phải 2

| # | File | Trạng thái |
|---|---|---|
| 1 | `backend/src/main/profile/OrcaProfile.ts` | **Chuẩn/thật** — backend dùng thật |
| 2 | `frontend/src/main/profile/OrcaProfile.ts` | Bản sao y hệt #1 nhưng **chết** — nằm trong cây Electron-main-process vestigial, không thuộc runtime deploy |
| 3 | `agent/src/main/profile/OrcaProfile.ts` | Bản sao y hệt #1, cũng **chết** — 0 importer trong `agent/src` |
| 4 | `frontend/src/shared/profile-types.ts` | Bản tách biệt, ít field hơn, **0 importer ở bất kỳ đâu** — mồ côi hoàn toàn |
| 5 | `frontend/src/renderer/src/types/profile-types.ts` | **UI thật đang dùng bản này** — và đây là bản **lệch xa backend nhất** |

**Bảng lệch chi tiết giữa #5 (UI thật dùng) và #1 (backend thật)**:

| Field | UI (#5) | Backend (#1) | Hậu quả |
|---|---|---|---|
| `agent.trustPreset` | `'strict'\|'standard'\|'relaxed'\|'custom'` | `'minimal'\|'standard'\|'full'` | lệch enum |
| `agent.approvedModels` | có | không có ở `agent` (nằm dưới `security`) | field đặt sai chỗ |
| `editor.fontSize/fontFamily/keybindings` | có | **hoàn toàn không tồn tại** trên backend | ghi vào, không bao giờ đọc lại được |
| `integrations.*` (githubOrg/linearWorkspace/prTemplate) | có | **section không tồn tại** | `ProfileResolver.merge()` không duyệt qua section này — **lưu được nhưng chết vĩnh viễn**, không bao giờ trả về trong `profile.getResolved` — ⏳ **còn mở**, chưa quyết định (xem [decisions-needed.md](../planning/decisions-needed.md) mục 6) |
| `fleet.*` | có | **section không tồn tại** | tương tự — chết vĩnh viễn nếu ghi — ⏳ **còn mở**, chưa quyết định |
| `security.disallowedCmds` | tên này | `disallowedCommands` | lệch tên field |
| `security.require2FA` | có | **không có field này** | ✅ Quyết định: thêm vào backend `SecurityProfileSection` — xem [decisions-needed.md](../planning/decisions-needed.md) mục 6 |
| `security.sessionTimeoutHours` | tên này | `maxSessionHours` | lệch tên field |

May mắn: `ProfileEditor.tsx` (component thật, được render) chỉ động tới `agent.preferredModel`
và `security.approvedModels` — 2 field DUY NHẤT trong bảng trên khớp đúng cả 2 bên — nên phần
lệch chưa gây lỗi hiển thị cho user, nhưng đã "bake" sẵn vào type contract cho code tương lai.

## 5. Đề xuất — cập nhật lại theo đúng thực tế đã xác nhận

### 5.1 Việc CẦN SỬA NGAY (bug thật, không phải thiết kế mới)

1. Sửa `useProfile.ts`: `profile.getUser` → `profile.getUserProfile`.
2. Thêm method `profile.listDepts` vào backend (`CompanyProfileAdmin.tsx`/`DeptProfileAdmin.tsx`
   giữ nguyên cách gọi) — xem [fix-proposals-per-issue.md](../planning/fix-proposals-per-issue.md) mục A2.
3. Sửa route `/admin` trong `http-server.ts` — tách rõ `/admin/api/*` (Express, giữ nguyên) khỏi
   `/admin`/`/admin-index.html` (cần fallback về static file serve).
4. Hợp nhất 5 bản `OrcaProfile`/profile-types — chọn `backend/src/main/profile/OrcaProfile.ts`
   làm chuẩn duy nhất, migrate `frontend/src/renderer/src/types/profile-types.ts` (bản UI thật
   đang dùng) về khớp đúng field/enum của backend, xoá 3 bản chết còn lại (#2, #3, #4).
5. Import `DeptProfileAdmin` vào `AdminApp.tsx`'s router (hiện bị bỏ sót).

### 5.2 Mô hình tổ chức đề xuất (Team — vẫn cần xây mới, xác nhận chưa có)

Company → Department (cây, đã có entity thật) → Team (cắt ngang, **cần xây mới hoàn toàn**,
không tái dùng bảng `orca_team_members` hiện có vì nó thiếu bảng metadata) → User.

```typescript
// Bảng metadata Team — MỚI, orca_team_members hiện có chỉ là bảng nối, chưa có bảng cha
Team {
  id: string
  name: string
  // KHÔNG có departmentId/parentId — Team không thuộc về 1 department cụ thể
}

// orca_team_members đã tồn tại (migration 0010) — tái dùng, chỉ cần thêm RPC team.* để ghi vào
TeamMember {
  teamId: string
  userId: string
  role: string       // đã có cột 'role' sẵn trong bảng thật
}

OrcaUser {
  ...
  departmentId: string | null   // MỚI — tận dụng cột department_id đã có trên orca_users
}
```

**✅ Quyết định (2026-08-13)**: 1 user có thể thuộc nhiều Team cùng lúc — khi merge profile
cascade, Team nào thắng nếu cấu hình khác nhau? Đã chốt: thêm `priority: number` vào
`TeamMember`, số cao thắng; `_sources` ghi rõ `'team:<teamId>'` thay vì chỉ `'team'` để audit
được chính xác Team nào đã ghi đè field đó.

### 5.3 RBAC cho Project/OrcaProject

**✅ Quyết định (2026-08-13)**: `TaskGrantService` (task-level) và `ProjectMember`
(project-level) **giữ nguyên 2 hệ tách biệt, không hợp nhất** — coi đây là chủ ý kiến trúc.

Hệ quả cho `OrcaProject` sharing layer: **dùng `ProjectMember`** (đề xuất 4-tầng visibility
`private`/`team`/`department`/`company` giữ nguyên như bản gốc), **không** tái dùng
`TaskGrantService`'s model — vì quyết định #4 đã chốt giữ 2 hệ Task/Project tách biệt, việc kéo
`TaskGrantService` sang dùng cho Project sẽ phá vỡ chính sự tách biệt đó. `OrcaProject` là mở
rộng của khái niệm Project, nên đi theo `ProjectMember` là nhất quán.

**Bảng năng lực theo role** (giữ nguyên đề xuất trước, áp dụng cho bất kỳ hệ grant nào được chọn):

| Hành động | `viewer` | `member` | `owner` |
|---|:---:|:---:|:---:|
| Xem file explorer, git status, terminal output | ✅ | ✅ | ✅ |
| Tạo worktree mới, mở terminal, chạy agent | ❌ | ✅ | ✅ |
| Thêm/xoá `Project` khỏi `OrcaProject` | ❌ | ❌ | ✅ |
| Thêm/xoá member, đổi role người khác | ❌ | ❌ | ✅ |
| Đổi `visibility`, xoá `OrcaProject` | ❌ | ❌ | ✅ |

## 6. Vì sao đề xuất này không đụng vào RBAC nhẹ hiện có (`SshTarget`)

`selectSshTargetsForCurrentUser` dùng string-tag tự do — **giữ nguyên, không migrate**. RBAC cho
`Project`/`OrcaProject`/`Task` dùng entity thật vì cần độ chính xác cao hơn. Ba hệ có thể cùng
tồn tại — miễn là không tiếp tục tạo thêm hệ thứ 4 cho cùng khái niệm "quyền truy cập theo
team/project" mà không ai biết đã có 2-3 hệ trước đó rồi.
