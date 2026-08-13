# User, Profile, Team, Department, và RBAC cho Project/OrcaProject

**Cập nhật:** 2026-08-13

> Tài liệu này mô tả identity/tổ chức hiện có trong code, chỉ ra khoảng trống, và đề xuất mô
> hình RBAC cho `Project`/`OrcaProject` — nối tiếp
> [terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md).
> Kế hoạch thực thi (thứ tự, phụ thuộc) xem
> [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md).

## 1. Những gì đã có trong code hôm nay

### `OrcaUser` — identity + role toàn cục (đang chạy thật)

```typescript
// frontend/src/renderer/src/store/slices/auth.ts
OrcaUser {
  id, email, name, avatarUrl
  role: 'developer' | 'lead' | 'admin'   // role TOÀN CỤC — không theo từng project
  teams: string[]                          // chuỗi tag tự do, KHÔNG phải FK tới 1 entity Team
  projects: string[]                       // chuỗi tag tự do, KHÔNG phải FK tới 1 entity Project
}
```

Đã có 1 pattern RBAC dùng field này — `selectSshTargetsForCurrentUser` (`store/selectors.ts`):

```typescript
if (!user || user.role === 'admin') return all        // admin/chưa login → thấy hết
return all.filter(target => {
  if (target.project && !user.projects.includes(target.project)) return false
  if (target.team && !user.teams.includes(target.team)) return false
  return true
})
```

`SshTarget.project?: string` / `SshTarget.team?: string` cũng chỉ là **chuỗi tag tự do** — so
khớp string với string, không có bảng `Team`/`Department` thật đứng sau. Đây là kiểu RBAC "nhẹ",
đủ dùng cho SSH targets nhưng không đủ chặt cho Project/OrcaProject (không phân biệt được role
theo từng project, không hỗ trợ department dạng cây).

### 2 hệ thống Profile song song — **lệch nhau**, giống hệt vấn đề gặp với `OrcaProject`

| | `frontend/src/main/profile/OrcaProfile.ts` (TDD-14) | `frontend/src/shared/profile-types.ts` (TDD-FE-11) |
|---|---|---|
| `trustPreset` | `'minimal'\|'standard'\|'full'` | `'strict'\|'standard'\|'relaxed'\|'custom'` |
| Field chặn lệnh | `disallowedCommands` | `disallowedCmds` |
| Có `Department` entity? | Không (chỉ merge theo `deptId: string`) | **Có** — `Department { id, name, parentId }` (dạng cây) |
| Nguồn ghi field | `_sources: Record<string, 'company'\|'dept'\|'user'>` | `_sources: Record<string, ProfileSource>` (`'company'\|'dept'\|'user'\|'concat'`) |

Backend thật (`frontend/src/main/profile/ProfileService.ts`, migration `0006`, bảng
`orca_companies`/`orca_departments`/`orca_user_profiles`) import từ `OrcaProfile.ts` (TDD-14).
`createProfileSlice` **đã được wire vào store thật** (`store/index.ts`, khác với
`workspace-slice.ts` đã gỡ) — nghĩa là hệ thống profile 3 lớp (Company → Department → User) khả
năng cao **đang chạy thật**, không phải scaffolding chết như `OrcaProject`. Nhưng việc có 2 bản
type lệch nhau cùng mô tả 1 khái niệm là landmine y hệt đã gặp — cần hợp nhất trước khi mở rộng
thêm Team.

### Chưa có: entity `Team`

Không tìm thấy bảng/type `Team` thật ở đâu — `OrcaUser.teams`/`SshTarget.team` chỉ là chuỗi tự
do. `Department` thì đã có entity thật (`profile-types.ts`, dạng cây qua `parentId`).

## 2. Mô hình tổ chức đề xuất

Theo xác nhận: **Company → Department (cây) → Team (cắt ngang) → User**, và **1 Team có thể gồm
thành viên từ nhiều Department khác nhau** — nghĩa là Team **không phải con của Department**
trong quan hệ cây, mà là 1 chiều nhóm riêng, cắt ngang qua nhiều Department.

```
Company
 │
 ├─ Department (cây, đã có entity thật qua parentId)
 │    └─ User  (1 user thuộc ĐÚNG 1 department — setUserDepartment(userId, deptId) đã có)
 │
 └─ Team (nhóm cắt ngang — KHÔNG lồng dưới Department)
      └─ TeamMember (userId, có thể đến từ NHIỀU department khác nhau)
```

**Schema mới cần thêm** (Team chưa tồn tại, phải tạo mới hoàn toàn):

```typescript
Team {
  id: string
  name: string
  // KHÔNG có departmentId/parentId — Team không thuộc về 1 department cụ thể
}

TeamMember {
  teamId: string
  userId: string
}

// OrcaUser cần thêm 1 field mới — hiện chỉ có teams: string[] (tag tự do)
OrcaUser {
  ...
  departmentId: string | null   // MỚI — 1 user thuộc đúng 1 department (cây)
  // teams: string[] giữ nguyên cho tương thích ngược (SshTarget vẫn dùng),
  // nhưng nguồn sự thật chuyển sang TeamMember khi cần RBAC chặt cho Project/OrcaProject
}
```

## 3. Profile cascade — thêm lớp Team

Cascade hiện tại (`ProfileService.ts`): **Company → Department → User** (`getCompanyProfileForUser`,
`getDeptProfile`, `getUserProfile`, merge: user thắng > dept thắng > company, trừ section khoá).

Thêm Team vào giữa Department và User: **Company → Department → Team → User**.

**Vấn đề cần quyết định rõ**: 1 user có thể thuộc **nhiều Team** cùng lúc (khác với Department —
chỉ 1). Khi 2 Team user thuộc về có cấu hình khác nhau cho cùng 1 field, ai thắng? Đề xuất:
thêm `priority: number` vào `TeamMember` (mặc định theo thứ tự thêm vào, priority cao hơn ghi đè
priority thấp hơn) — tránh đoán ngầm thứ tự merge, ghi rõ trong `_sources` giống các layer khác
(vd. `'team:<teamId>'` thay vì chỉ `'team'`, để biết chính xác Team nào đã ghi đè field đó).

## 4. RBAC cho `Project` / `OrcaProject` — 4 tầng visibility + role tinh theo từng người

Mở rộng đề xuất "`OrcaProject` là lớp SỞ HỮU + CHIA SẺ" trong
[terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md)
thêm tầng `department` (trước đó chỉ có `private`/`team`/`company`):

```typescript
OrcaProject {
  ...
  visibility: 'private' | 'team' | 'department' | 'company'
  teamId?: string         // bắt buộc nếu visibility = 'team'
  departmentId?: string   // bắt buộc nếu visibility = 'department'
}
```

| `visibility` | Field bắt buộc | Ai **thấy được** project tồn tại | Role mặc định nếu chưa có `ProjectMember` |
|---|---|---|---|
| `private` | — | Chỉ user có row trong `ProjectMember` | — (không thấy) |
| `team` | `teamId` | User có `TeamMember` khớp `teamId` (bất kể thuộc department nào) | `viewer` |
| `department` | `departmentId` | User có `departmentId` khớp (bất kể thuộc team nào) | `viewer` |
| `company` | — | Mọi user đã đăng nhập | `viewer` |

`ProjectMember { projectId, userId, role }` (đã có sẵn trong `project-types.ts`) **luôn override**
role mặc định ở trên — dùng để nâng quyền 1 người cụ thể lên `member`/`owner` mà không cần cả
team/department đều có quyền đó. `OrcaUser.role === 'admin'` (role toàn cục) **luôn bypass mọi
kiểm tra** — khớp đúng hành vi `selectSshTargetsForCurrentUser` đang chạy, không phát minh quy
tắc mới.

**Bảng năng lực theo `ProjectRole`:**

| Hành động | `viewer` | `member` | `owner` |
|---|:---:|:---:|:---:|
| Xem file explorer, git status, terminal output | ✅ | ✅ | ✅ |
| Tạo worktree mới, mở terminal, chạy agent | ❌ | ✅ | ✅ |
| Thêm/xoá `Project` khỏi `OrcaProject` | ❌ | ❌ | ✅ |
| Thêm/xoá member, đổi role người khác | ❌ | ❌ | ✅ |
| Đổi `visibility`, xoá `OrcaProject` | ❌ | ❌ | ✅ |

## 5. Việc cần làm trước khi code, theo thứ tự ưu tiên

1. **Hợp nhất 2 bản `OrcaProfile`/`profile-types.ts`** (TDD-14 vs TDD-FE-11) về 1 nguồn duy
   nhất — làm trước, vì mọi thứ ở đây phụ thuộc vào đúng 1 định nghĩa `Department`.
2. **Tạo entity `Team`/`TeamMember`** mới hoàn toàn — hiện chưa tồn tại dưới dạng bảng thật,
   chỉ có string tag lỏng lẻo trên `OrcaUser.teams`.
3. **Thêm `departmentId` vào `OrcaUser`**, tận dụng `setUserDepartment()` đã có sẵn trong
   `ProfileService.ts`.
4. **Mở rộng `OrcaProject.visibility`** thêm `'department'`, thêm `teamId`/`departmentId`.
5. **Quyết định rule merge multi-team** cho profile cascade (mục 3) trước khi implement, đừng
   để ngầm định — dễ gây bug khó debug giống lớp bug "ambiguous host" gặp phải tuần này.

## 6. Vì sao đề xuất này không đụng vào RBAC nhẹ hiện có (`SshTarget`)

`selectSshTargetsForCurrentUser` dùng string-tag tự do (`user.teams.includes(target.team)`) —
**giữ nguyên, không migrate**. RBAC cho `Project`/`OrcaProject` dùng entity thật
(`TeamMember`/`ProjectMember`) vì cần độ chính xác cao hơn (role khác nhau theo từng người,
không chỉ "có trong team hay không"). Hai hệ thống có thể cùng tồn tại — SSH targets vẫn dùng
tag nhẹ, Project/OrcaProject dùng RBAC chặt — miễn là không tái diễn việc 2 khái niệm cùng tên
(`team`) trỏ tới 2 nguồn dữ liệu khác nhau mà không ai biết.
