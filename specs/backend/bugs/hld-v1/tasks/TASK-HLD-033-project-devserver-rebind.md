# TASK-HLD-033: Thêm `devServerId` vào `UpdateProjectParams` — cho phép rebind Project sang Dev Server khác

**Priority:** 🟡 MEDIUM — feature gap, project hiện không thể đổi Dev Server sau khi tạo
**Effort:** ~1 giờ
**Status:** ✅ DONE — 2026-08-09 (xác nhận nội dung thật của cả 3 file khớp với dòng task nêu trước khi sửa. Thêm `devServerId?: string` vào `UpdateProjectParams`; thêm `devServerId: z.string().min(1).optional()` vào `UpdateProjectParam.patch` zod schema (handler `project.update` không cần sửa — đã forward nguyên `patch`, xác nhận qua đọc code); `ProjectService.update()` thêm khối validate `devServerId` qua `DevServerManager.get()` mirror đúng `create()`, kèm TODO business-decision y nguyên văn cho việc chặn rebind khi có execution đang chạy (KHÔNG tự ý implement, đúng yêu cầu). `tsc --noEmit`: 0 lỗi mới cho `project-types.ts`/`project-rpc-handler.ts`; `ProjectService.ts` chỉ còn lỗi baseline có sẵn (`TS2558`/`TS2740`/`TS2345`/`TS2739` từ `db.query<T>` generic — cùng pattern đã xác nhận nhiều lần trong phiên này, không liên quan devServerId). Dependency TASK-HLD-002/003 đã DONE trước đó trong phiên này nên chính sách RBAC "owner hoặc admin" cho rebind đã có hiệu lực đầy đủ ngay từ bây giờ, không còn ở trạng thái tạm "chỉ owner" như task mô tả cho trường hợp merge riêng lẻ. ⚠️ Chưa viết test case — effort budget. **Đây là task cuối cùng trong batch 33 task — 32/33 DONE (chỉ còn TASK-HLD-005 pending có chủ đích, follow-up thiết kế PermissionService phase 2, ưu tiên thấp).**)
**Bug refs:** BUG-BE-HLD-020
**Solution ref:** [SOLUTION-project-devserver-rebind-exact.md](../solutions/SOLUTION-project-devserver-rebind-exact.md) — Mục 1, 2, 3
**Depends on:** TASK-HLD-002, TASK-HLD-003 — fix RBAC (`requireOwnerOrAdmin` hiện chỉ check `role !== 'owner'`, tham số `_userId` bị bỏ qua, không có nhánh admin thật) phải merge trước để giới hạn quyền rebind đúng theo chính sách "Lead/Admin". Chi tiết fix RBAC xem [SOLUTION-rbac-exact.md](../solutions/SOLUTION-rbac-exact.md) — **không lặp lại nội dung đó ở đây.**

---

## Mục tiêu

`Project.devServerId` hiện chỉ set được lúc `create()`, không thể rebind (đổi Dev Server đích) sau khi project đã tồn tại — `UpdateProjectParams` không có field `devServerId`, và `ProjectService.update()` không xử lý field này dù có trong DB (cột `dev_server_id`).

Task này thêm hỗ trợ rebind ở 3 lớp:

1. `UpdateProjectParams` (shared type) — thêm field `devServerId?: string`.
2. Zod schema RPC `UpdateProjectParam` (`project-rpc-handler.ts`) — thêm validate cho field mới. Handler `project.update` không cần sửa gì thêm — đã forward toàn bộ `patch` nguyên trạng.
3. `ProjectService.update()` — validate `devServerId` mới tồn tại qua `DevServerManager.get()` (mirror đúng validation đã có trong `create()`), rồi mới persist vào cột `dev_server_id`.

**Về quyền hạn (RBAC):** `project.update` RPC method đã gọi `requireOwnerOrAdmin(member.role, userId)` trước khi vào `ProjectService.update(...)` — field `devServerId` mới tự động đi qua đúng guard này, không cần thêm code guard riêng cho rebind. Nhưng bản thân `requireOwnerOrAdmin` hiện tại (`project-rpc-handler.ts:249-253`) chỉ thật sự cho phép **owner** (tham số `_userId` bị bỏ qua, không có nhánh check admin toàn hệ thống) — đây là nội dung của BUG-BE-HLD-002 / TASK-HLD-002+003. Cho tới khi 2 task đó merge, quyền rebind mô tả ở task này **chặt hơn cần thiết** (chỉ owner rebind được, admin không tự động rebind được) — đây KHÔNG phải bug của task này, chỉ là hệ quả tạm thời của thứ tự merge. Sau khi TASK-HLD-002/003 merge, rebind tự động thừa hưởng đúng chính sách "owner hoặc admin" mà không cần sửa gì thêm ở các file dưới đây.

**Về chặn rebind khi có execution đang chạy:** đây là quyết định business CHƯA được xác nhận — xem TODO rõ ràng trong mục 3 bên dưới. KHÔNG tự ý implement logic chặn.

## File cần sửa/tạo

```
backend/src/shared/project-types.ts                (thêm devServerId? vào UpdateProjectParams, dòng 65-70)
backend/src/main/project/project-rpc-handler.ts     (thêm devServerId vào zod schema, dòng 40-48)
backend/src/main/project/ProjectService.ts          (validate + persist devServerId trong update(), dòng 186-200)
```

## Thay đổi cụ thể

### 1. `UpdateProjectParams` — `backend/src/shared/project-types.ts` (thay thế dòng 65-70)

Code thiếu hiện tại:
```typescript
export interface UpdateProjectParams {
  name?: string
  description?: string
  defaultBranch?: string
  visibility?: ProjectVisibility
}
```

Fix:
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

### 2. Zod schema RPC — `backend/src/main/project/project-rpc-handler.ts` (thay thế dòng 40-48)

Code thiếu hiện tại:
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

Fix:
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

### 3. Validate + persist trong `ProjectService.update()` — `backend/src/main/project/ProjectService.ts` (thay thế dòng 186-200)

Code thiếu hiện tại:
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

Fix — validate qua `DevServerManager.get()` giống hệt `create()` (dòng 87-93), tái dùng cùng error code `DEV_SERVER_NOT_FOUND`:
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

## Verification

```bash
# 1. Field devServerId đã có trong type + schema
grep -n "devServerId" backend/src/shared/project-types.ts
grep -n "devServerId" backend/src/main/project/project-rpc-handler.ts
# Expected: mỗi file có ít nhất 1 dòng khớp

# 2. ProjectService.update() đã validate qua DevServerManager
grep -n "DEV_SERVER_NOT_FOUND" backend/src/main/project/ProjectService.ts
# Expected: xuất hiện cả ở create() (đã có từ trước) và update() (mới thêm)

# 3. Type-check
pnpm --filter backend tsc --noEmit

# 4. Unit test ProjectService.update():
#    - patch.devServerId trỏ tới dev server không tồn tại → throw chứa 'DEV_SERVER_NOT_FOUND'
#    - patch.devServerId hợp lệ → cột dev_server_id trong DB được cập nhật đúng
#    - patch không có devServerId → hành vi cũ (các field khác) không đổi, dev_server_id
#      giữ nguyên giá trị cũ (không bị ghi đè thành NULL/undefined)

# 5. Integration test RPC project.update qua project-rpc-handler:
#    - User role 'owner' → rebind thành công
#    - User role khác 'owner' (developer/lead) trước khi TASK-HLD-002/003 merge →
#      bị FORBIDDEN (hành vi tạm thời, chặt hơn thiết kế cuối — không phải bug của task này)
#    - Sau khi TASK-HLD-002/003 merge → xác nhận lại test case admin rebind theo
#      đúng chính sách RBAC cuối cùng (xem SOLUTION-rbac-exact.md)
pnpm --filter backend test -- ProjectService project-rpc-handler

# 6. Không tự ý thêm logic chặn "rebind khi có execution đang chạy" — nếu review
#    thấy code đã có logic đó xuất hiện ngoài TODO comment, đó là lỗi phạm vi, cần
#    quay lại xác nhận business decision trước khi merge.
```
