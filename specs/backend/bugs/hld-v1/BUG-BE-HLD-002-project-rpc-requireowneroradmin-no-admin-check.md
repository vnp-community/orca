# BUG-BE-HLD-002 — `requireOwnerOrAdmin` không hề check admin (dead code trong tên hàm); `project.create` không giới hạn quyền

**Mức độ:** 🟠 HIGH (Security — RBAC broken by design)
**Status:** 🔴 Open
**Module:** `backend/src/main/project/project-rpc-handler.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.13/F34)

---

## Mô tả

`docs/features/F34-project-dev-server-binding.md` mô tả: chỉ **Lead/Admin** được tạo project và bind dev server; global `admin` (role hệ thống theo F32) phải override được quyền trên bất kỳ project nào khi cần.

Code thực tế (`backend/src/main/project/project-rpc-handler.ts:249-253`):

```typescript
function requireOwnerOrAdmin(role: ProjectRole, userId: string) {
  if (role !== 'owner') throw ...  // KHÔNG có nhánh nào check admin
}
```

- Tên hàm hứa hẹn "OrAdmin" nhưng **không có nhánh code nào kiểm tra global admin** — chỉ check `role !== 'owner'` (role ở đây là `ProjectRole = 'owner'|'member'|'viewer'`, một hệ role **hoàn toàn khác** với `'developer'|'lead'|'admin'` của F32/F33 org-level RBAC).
- `project.create` (dòng khởi tạo project + bind dev server lần đầu) chỉ yêu cầu `UNAUTHENTICATED` check (đã đăng nhập) — không giới hạn theo Lead/Admin như user-flow F34 mô tả.

## Hậu quả

- Global admin (role hệ thống) **không override được** quyền update/delete/add-member trên project mà mình không phải `owner` — mâu thuẫn trực tiếp với ý nghĩa "admin" mọi nơi khác trong hệ thống.
- Bất kỳ user đã login nào tạo được project + bind vào bất kỳ dev server nào (miễn dev server đó tồn tại) — không giới hạn Lead/Admin.
- 2 khái niệm "role" trùng tên khác domain (`ProjectRole` project-level vs org-level role F32) dễ gây nhầm lẫn khi review code hoặc viết test — rủi ro lặp lại bug tương tự ở chỗ khác.

## Bằng chứng

- `backend/src/main/project/project-rpc-handler.ts:247` — `type ProjectRole = 'owner' | 'member' | 'viewer'`.
- `backend/src/main/project/project-rpc-handler.ts:249-253` — implementation `requireOwnerOrAdmin`, không có nhánh admin.
- `backend/src/main/project/ProjectService.ts:82-136` — `create()` không có role/permission check ngoài `UNAUTHENTICATED`.

## Đề xuất fix

1. Sửa `requireOwnerOrAdmin` để nhận thêm `globalRole` (role hệ thống từ `ctx`, đã cần bổ sung cho [BUG-BE-HLD-001](./BUG-BE-HLD-001-profile-rpc-handler-requireadmin-stub-no-role-check.md)) và cho phép bypass nếu `globalRole === 'admin'`.
2. Giới hạn `project.create` theo role Lead/Admin nếu đúng ý định thiết kế F34, hoặc cập nhật lại F34 để phản ánh đúng "mọi user đều tạo được project" nếu đó là chủ đích sản phẩm (làm rõ với PO trước khi sửa code).
3. Đổi tên `ProjectRole` thành tên rõ ràng hơn (vd `ProjectMemberRole`) để tránh nhầm với org-level role — theo đúng quy tắc đặt tên trong AGENTS.md ("Never use vague names").

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.13 (F34), §6 mục 3 (Top 10)
- Doc gốc: `docs/features/F34-project-dev-server-binding.md`
- Liên quan: [BUG-BE-HLD-001](./BUG-BE-HLD-001-profile-rpc-handler-requireadmin-stub-no-role-check.md), [BUG-BE-HLD-003](./BUG-BE-HLD-003-rbac-fragmented-no-policy-table.md), [BUG-BE-HLD-020](./BUG-BE-HLD-020-project-devserver-binding-not-rebindable.md)
