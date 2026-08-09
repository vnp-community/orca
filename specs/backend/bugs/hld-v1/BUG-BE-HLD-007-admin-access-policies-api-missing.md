# BUG-BE-HLD-007 — Toàn bộ backend API cho Access Policies (PoliciesPage) không tồn tại dù DB schema đã sẵn sàng

**Mức độ:** 🟠 HIGH (Feature gap — Admin Panel / RBAC)
**Status:** 🔴 Open
**Module:** `backend/src/main/admin/` (thiếu file), `backend/src/main/db/migrations/0005_add_auth_schema.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.4/F25, §5.11/F32)

---

## Mô tả

`docs/features/F25-admin-panel.md` liệt kê **PoliciesPage** ("RBAC access policy CRUD") là tính năng đã Phát hành, chỉ note phần *enforcement* SSH của policy là "DEFERRED Phase 3" — nghĩa là CRUD/API của policy được kỳ vọng đã có.

Thực tế:
- Bảng DB `orca_access_policies` đã tồn tại đầy đủ field (`backend/src/main/db/migrations/0005_add_auth_schema.ts:82-98`).
- Type `PolicyInput` đã định nghĩa (`backend/src/main/admin/admin-types.ts:41-52`).
- **Không có route `/admin/api/policies*` nào** trong `admin-router.ts` (`backend/src/main/admin/admin-router.ts:23-45` chỉ mount stats/users/sessions/audit).
- **Không tồn tại file `admin-policy-handlers.ts`** trong `backend/src/main/admin/`.

Đây là gap vượt xa phần doc tự nhận là "deferred" — CRUD cơ bản (không phải enforcement) hoàn toàn thiếu, không chỉ enforcement.

## Hậu quả

- PoliciesPage trên Admin SPA không có API để gọi — UI (nếu tồn tại) sẽ crash hoặc hiển thị lỗi khi user vào trang này.
- Không có cách nào tạo/sửa/xoá `OrcaAccessPolicy` (dùng bởi `resolveUserPermissions()`, xem [BUG-BE-HLD-003](./BUG-BE-HLD-003-rbac-fragmented-no-policy-table.md)) qua Admin Panel — phải thao tác trực tiếp DB.

## Bằng chứng

- `backend/src/main/admin/admin-router.ts:23-45` — không có `/policies` route.
- `find backend/src/main/admin -iname "*policy*"` → không có kết quả.
- `backend/src/main/db/migrations/0005_add_auth_schema.ts:82-98` — bảng đã sẵn sàng, chưa dùng.
- `backend/src/main/admin/admin-types.ts:41-52` — `PolicyInput` type đã có, chưa có handler dùng nó.

## Đề xuất fix

1. Thêm `admin-policy-handlers.ts` theo đúng pattern của `admin-user-handlers.ts`/`admin-session-handlers.ts` (list/create/update/delete).
2. Mount route `/admin/api/policies` trong `admin-router.ts` với `requireAdmin` guard.
3. Kết nối `resolveUserPermissions()` để đọc policy vừa tạo (đảm bảo round-trip CRUD → effect thật trên RBAC).

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.4 (F25), §6 mục 5 (Top 10)
- Doc gốc: `docs/features/F25-admin-panel.md`
- Liên quan: [BUG-BE-HLD-003](./BUG-BE-HLD-003-rbac-fragmented-no-policy-table.md), [BUG-BE-HLD-006](./BUG-BE-HLD-006-admin-sessions-list-stub-empty.md)
