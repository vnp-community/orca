# SOLUTION: BUG-BE-HLD-020 — Project–DevServer binding không thể rebind — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế
**Files nguồn đã đọc:**
`backend/src/main/project/ProjectService.ts` (toàn bộ, 295 dòng),
`backend/src/main/project/project-rpc-handler.ts` (toàn bộ, 254 dòng),
`backend/src/main/dev-server/dev-server-manager.ts` (`DevServerManager.get()`, dòng 120-124),
`backend/src/shared/project-types.ts` (`UpdateProjectParams`, `CreateProjectParams`),
`specs/backend/tdd/v5/15-project-binding.md`.

**Dependency quan trọng:** Fix này giả định [BUG-BE-HLD-002](../BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md) đã được fix trước. `requireOwnerOrAdmin()` hiện tại (`project-rpc-handler.ts:249-253`) chỉ check `role !== 'owner'` — KHÔNG có nhánh admin thật (tham số `_userId` không dùng), nghĩa là quyền rebind mô tả dưới đây (dùng lại `requireOwnerOrAdmin` sẵn có trên `project.update`) hiện đang **chặt hơn cần thiết** (chỉ owner, không phải admin nào cũng rebind được) cho tới khi BUG-BE-HLD-002 được vá. Solution này **không lặp lại** chi tiết fix RBAC đó — xem `SOLUTION-rbac-exact.md` (nếu tồn tại trong cùng thư mục `solutions/`) cho phần đó. Sau khi BUG-BE-HLD-002 được fix, `requireOwnerOrAdmin` tự động áp dụng đúng cho cả `project.update` (bao gồm rebind) mà không cần sửa gì thêm ở file này.

---

## 1. Thêm `devServerId` vào `UpdateProjectParams` (shared type)

**File:** `backend/src/shared/project-types.ts`
**Lines:** 65-70

### Code thiếu hiện tại:
```typescript
export interface UpdateProjectParams {
  name?: string
  description?: string
  defaultBranch?: string
  visibility?: ProjectVisibility
}
```

### Fix:
```typescript
// backend/src/shared/project-types.ts — thay thế dòng 65-70:
export interface UpdateProjectParams {
  name?: string
  description?: string
  defaultBranch?: string
  visibility?: ProjectVisibility
  /** Rebind project to a different Dev Server. Validated against
   *  DevServerManager before being persisted — see ProjectService.update(). */
  devServerId?: string
}
```

---

## 2. Thêm `devServerId` vào `UpdateProjectParam` (zod schema RPC)

**File:** `backend/src/main/project/project-rpc-handler.ts`
**Lines:** 40-48

### Code thiếu hiện tại:
```typescript
const UpdateProjectParam = z.object({
  projectId: z.string().min(1),
  patch: z.object({
    name: z.string().min(1).optional(),
    description: z.string().optional(),
    defaultBranch: z.string().optional(),
    visibility: z.enum(['private', 'team', 'company']).optional(),
  }),
})
```

### Fix:
```typescript
// backend/src/main/project/project-rpc-handler.ts — thay thế dòng 40-48:
const UpdateProjectParam = z.object({
  projectId: z.string().min(1),
  patch: z.object({
    name: z.string().min(1).optional(),
    description: z.string().optional(),
    defaultBranch: z.string().optional(),
    visibility: z.enum(['private', 'team', 'company']).optional(),
    devServerId: z.string().min(1).optional(),
  }),
})
```

Handler `project.update` (dòng 125-136) không cần sửa — đã gọi `projectService.update(params.projectId, params.patch, userId)` với toàn bộ `patch`, field mới tự động đi qua nguyên trạng.

---

## 3. Validate dev server mới tồn tại + persist `devServerId` — `ProjectService.update()`

**File:** `backend/src/main/project/ProjectService.ts`
**Lines:** 186-200 (thay thế)

### Code thiếu hiện tại:
```typescript
  /** Update project fields (partial patch). */
  async update(projectId: string, patch: UpdateProjectParams, _updatedBy: string): Promise<void> {
    const now = Date.now()
    const sets: string[] = ['updated_at = ?']
    const values: unknown[] = [now]

    if (patch.name !== undefined) { sets.push('name = ?'); values.push(patch.name) }
    if (patch.description !== undefined) { sets.push('description = ?'); values.push(patch.description) }
    if (patch.defaultBranch !== undefined) { sets.push('default_branch = ?'); values.push(patch.defaultBranch) }
    if (patch.visibility !== undefined) { sets.push('visibility = ?'); values.push(patch.visibility) }

    values.push(projectId)
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_v5_projects SET ${sets.join(', ')} WHERE id = ?`, values)
    )
  }
```

### Fix — validate qua `DevServerManager.get()` giống hệt `create()` (dòng 87-93), tái dùng cùng error code `DEV_SERVER_NOT_FOUND`:
```typescript
// backend/src/main/project/ProjectService.ts — thay thế dòng 186-200:

  /**
   * Update project fields (partial patch).
   * - Rebinding devServerId validates the target exists in DevServerManager,
   *   mirroring create()'s validation (ProjectService.ts:87-93).
   * @throws 'DEV_SERVER_NOT_FOUND' if patch.devServerId does not exist
   */
  async update(projectId: string, patch: UpdateProjectParams, _updatedBy: string): Promise<void> {
    const span = Tracers.profileProjectRouteFlow.start({
      op: 'update', projectId, devServerId: patch.devServerId
    })

    if (patch.devServerId !== undefined) {
      const server = this.devServerManager.get(patch.devServerId)
      span.step('validateDevServer', { devServerId: patch.devServerId })
      if (!server) {
        span.fail('DEV_SERVER_NOT_FOUND', { devServerId: patch.devServerId })
        throw new Error(`DEV_SERVER_NOT_FOUND: devServerId '${patch.devServerId}' does not exist`)
      }

      // TODO(BUG-BE-HLD-020, business decision cần xác nhận): chặn rebind nếu
      // project đang có workflow execution / task chạy dở gắn với dev server
      // CŨ (patch.devServerId khác project.devServerId hiện tại) — tránh orphan
      // execution khi worker mất kết nối tới host cũ giữa chừng. TDD-15 §7 đã
      // định nghĩa sẵn error code cho tình huống tương tự ở delete()
      // (`PROJECT_HAS_ACTIVE_WORKFLOWS` — xem specs/backend/tdd/v5/15-project-binding.md
      // dòng ~276) nhưng CHƯA được implement ở đâu trong backend/src (đã grep xác
      // nhận) — nghĩa là cả delete() lẫn rebind đều thiếu check này, không riêng
      // rebind. Cần quyết định business trước khi code:
      //   1. Có cho rebind khi có execution đang chạy không, hay luôn chặn?
      //   2. Nếu chặn, dùng WorkflowOrchestrator (backend/src/main/workflow/) để
      //      query execution theo projectId + status='running' — bảng nào, cột nào?
      //   3. Nếu cho phép, execution cũ có cần được hủy/marked orphaned tự động
      //      không, hay để nguyên và chỉ warning ở UI?
      // Không tự ý implement logic chặn ở đây cho tới khi có quyết định.
    }

    const now = Date.now()
    const sets: string[] = ['updated_at = ?']
    const values: unknown[] = [now]

    if (patch.name !== undefined) { sets.push('name = ?'); values.push(patch.name) }
    if (patch.description !== undefined) { sets.push('description = ?'); values.push(patch.description) }
    if (patch.defaultBranch !== undefined) { sets.push('default_branch = ?'); values.push(patch.defaultBranch) }
    if (patch.visibility !== undefined) { sets.push('visibility = ?'); values.push(patch.visibility) }
    if (patch.devServerId !== undefined) { sets.push('dev_server_id = ?'); values.push(patch.devServerId) }

    values.push(projectId)
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_v5_projects SET ${sets.join(', ')} WHERE id = ?`, values)
    )
    span.ok({ op: 'update', projectId })
  }
```

> `Tracers` và `DevServerManager` đã import sẵn ở đầu file (`ProjectService.ts:14-15`) — không cần thêm import mới.

---

## 4. Quyền hạn rebind (RBAC) — phụ thuộc BUG-BE-HLD-002, không lặp lại chi tiết ở đây

`project.update` RPC method (`project-rpc-handler.ts:125-136`) hiện đã gọi `requireOwnerOrAdmin(member.role, userId)` trước khi gọi `projectService.update(...)` — field `devServerId` mới đi qua đúng cùng một guard, KHÔNG cần thêm code guard riêng cho rebind. Tuy nhiên bản thân `requireOwnerOrAdmin` (dòng 249-253):

```typescript
function requireOwnerOrAdmin(role: ProjectRole, _userId: string): void {
  if (role !== 'owner') {
    throw new Error('FORBIDDEN: only project owners can perform this action')
  }
}
```

chỉ thật sự cho phép **owner**, tham số `_userId` bị bỏ qua (không có nhánh kiểm tra admin toàn hệ thống) — đây chính là nội dung của BUG-BE-HLD-002. Solution này **không sửa hàm trên** (tránh trùng lặp/đụng độ với fix RBAC riêng) — chỉ ghi nhận: sau khi BUG-BE-HLD-002 được vá đúng (thêm nhánh check admin thật dựa trên `userId`), rebind sẽ tự động thừa hưởng đúng chính sách quyền hạn "Lead/Admin" mà `docs/features/F34-project-dev-server-binding.md` mô tả, vì `project.update` đã dùng chung guard cho mọi field patch kể cả `devServerId`.

---

## Tóm tắt thay đổi

| Thay đổi | File | Lines |
|---|---|---|
| Thêm `devServerId?` vào interface | `backend/src/shared/project-types.ts` | 65-70 |
| Thêm `devServerId` vào zod schema | `backend/src/main/project/project-rpc-handler.ts` | 40-48 |
| Validate + persist `devServerId` trong `update()` | `backend/src/main/project/ProjectService.ts` | 186-200 |
| TODO business decision: chặn rebind khi có execution đang chạy | `backend/src/main/project/ProjectService.ts` | trong `update()`, trước phần persist |
| RBAC (owner/admin) cho rebind | *(không sửa ở đây)* | phụ thuộc BUG-BE-HLD-002 — xem `SOLUTION-rbac-exact.md` |

**Không đổi:** `create()` (dòng 82-136) — validate `devServerId` lúc tạo project đã đúng, dùng làm khuôn mẫu cho fix ở `update()`.
