# BUG-BE-HLD-003 — Không có policy table role×resource×action (`hasPermission()`); RBAC phân mảnh 4 cơ chế không tương thích

**Mức độ:** 🟠 HIGH (Architecture gap — security consistency)
**Status:** 🔴 Open
**Module:** `backend/src/shared/rbac-types.ts`, `backend/src/main/admin/admin-middleware.ts`, `backend/src/main/profile/profile-rpc-handler.ts`, `backend/src/main/project/project-rpc-handler.ts`
**Phát hiện:** 2026-08-08/09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §2.6, §5.11/F32)

---

## Mô tả

`docs/hld/backend-server-architecture.md` §5 mô tả `hasPermission()` (Role → resource → action policy table) như cơ chế RBAC trung tâm. `docs/features/F32-team-rbac.md` cũng yêu cầu một ma trận permission thống nhất theo role×resource×action.

Hàm `hasPermission()` **không tồn tại ở đâu trong codebase** (đã grep xác nhận). Thay vào đó, RBAC bị phân mảnh thành **4 cơ chế độc lập, không tương thích nhau**:

1. `resolveUserPermissions()` (`backend/src/shared/rbac-types.ts:73-119`) — merge union các `OrcaAccessPolicy[]` ra `{allowedServerIds, allowedProjects, agentTrust}`, phục vụ fleet/server access — không có khái niệm resource/action rời rạc.
2. `requireAdmin` HTTP-route middleware (`backend/src/main/admin/admin-middleware.ts:21-43`) — nhị phân đúng (401/403 theo `role==='admin'`), nhưng chỉ áp dụng cho `/admin/api/*`.
3. `requireAdmin` RPC-handler stub (`backend/src/main/profile/profile-rpc-handler.ts:282-293`) — **lỗi**, xem [BUG-BE-HLD-001](./BUG-BE-HLD-001-profile-rpc-handler-requireadmin-stub-no-role-check.md).
4. `requireOwnerOrAdmin` project-level (`backend/src/main/project/project-rpc-handler.ts:249-253`) — **dead code trong tên hàm**, xem [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md).

Không cơ chế nào implement đúng "Role → resource → action" như tài liệu mô tả; mỗi domain tự chế cơ chế riêng.

## Hậu quả

- Không có single-source-of-truth cho "ai được làm gì" — mỗi domain (Auth, Profile, Project, Task Graph) tự quyết cách check quyền, dễ tạo lỗ hổng như BUG-BE-HLD-001/002.
- Audit/compliance khó thực hiện (không thể liệt kê "user X có quyền gì" bằng 1 query duy nhất).
- Thêm domain mới (vd F35 AI Provider, F36 Workflow) rất dễ lặp lại lỗi tương tự vì không có pattern chuẩn để tái sử dụng.

## Bằng chứng

- Grep `hasPermission` toàn `backend/src`: 0 kết quả (ngoài `hasPermissionSuffix()` trong `agent-title-owner.ts`, không liên quan RBAC).
- 4 vị trí nêu trên với 4 shape dữ liệu khác nhau, không import lẫn nhau.
- `TaskGrantService.resolvePermission()` (`backend/src/main/task/TaskGrantService.ts:112-157`) là RBAC riêng thứ 5 cho task graph (BFS ancestor + apply_tree) — thêm 1 cơ chế nữa ngoài 4 cái trên, càng khẳng định tình trạng phân mảnh.

## Đề xuất fix

1. Thiết kế 1 `PermissionService.hasPermission(userId, resource, action, context?)` trung tâm, dùng policy table hoặc rule engine thống nhất.
2. Migrate dần từng domain (Admin route → Profile RPC → Project RPC) sang gọi `PermissionService`, giữ `TaskGrantService` là đặc thù hợp lý (task graph cần grant-tree riêng) nhưng expose qua cùng interface `hasPermission`.
3. Ưu tiên fix trước 2 lỗ hổng cụ thể [BUG-BE-HLD-001](./BUG-BE-HLD-001-profile-rpc-handler-requireadmin-stub-no-role-check.md) và [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md) trước khi làm refactor lớn.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §2.6, §5.11, §6 mục 1
- Doc gốc: `docs/hld/backend-server-architecture.md` §5, `docs/features/F32-team-rbac.md`
- Liên quan: [BUG-BE-HLD-001](./BUG-BE-HLD-001-profile-rpc-handler-requireadmin-stub-no-role-check.md), [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md)
