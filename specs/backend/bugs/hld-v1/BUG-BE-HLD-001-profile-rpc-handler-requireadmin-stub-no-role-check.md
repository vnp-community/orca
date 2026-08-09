# BUG-BE-HLD-001 — `requireAdmin(ctx)` trong `profile-rpc-handler.ts` chỉ check đã login, KHÔNG check role admin

**Mức độ:** 🔴 CRITICAL (Security — permission bypass)
**Status:** 🔴 Open
**Module:** `backend/src/main/profile/profile-rpc-handler.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.11/F32, §5.12/F33)

---

## Mô tả

`docs/features/F32-team-rbac.md` yêu cầu company/department profile chỉ được sửa bởi role `admin`. RPC method `profile.updateCompany`, `profile.updateDept`, `profile.createCompany`, `profile.createDept`, `profile.setUserDept`, `profile.getCompany`, `profile.invalidate` đều gọi một helper `requireAdmin(ctx)` bên trong handler (`backend/src/main/profile/profile-rpc-handler.ts:282-293`).

Hàm này **chỉ kiểm tra `ctx.userId` tồn tại** (nghĩa là "đã đăng nhập"), **không hề kiểm tra `ctx.userRole === 'admin'`**. Comment ngay trong code tự thừa nhận đây là việc chưa làm:

```
// TODO: when AuthManager exposes ctx.userRole...
// Admin enforcement deferred to AuthManager middleware in http-server.ts
// for routes decorated with requireAdmin flag. In-process RPC callers
// must pass role validation upstream.
```

Nhưng các RPC method trên được gọi qua kênh **in-process/WS RPC nội bộ** (không phải HTTP route `/admin/api/*` có `requireAdmin` middleware thật ở `admin-middleware.ts`) — nên "validation upstream" mà comment giả định **không tồn tại**.

## Hậu quả

- **Bất kỳ user đã đăng nhập nào** (không cần role `admin`) gọi được `profile.updateCompany`/`updateDept`/`createCompany`/`createDept`/`setUserDept` — sửa/tạo chính sách công ty, phòng ban, và gán department cho user khác.
- Trực tiếp phá vỡ acceptance criterion "security fields chỉ company-level set được" của F33 (User Profile Hierarchy) — vì chính field `security` (approvedModels, maxSessionHours...) nằm trong `profile.updateCompany` payload mà `requireAdmin` không chặn được user thường.
- Vi phạm KPI "0 permission bypass (P0)" mà F32 tự đặt ra.

## Bằng chứng

- `backend/src/main/profile/profile-rpc-handler.ts:282-293` — implementation của `requireAdmin(ctx)`, chỉ check `ctx.userId`.
- Các call site dùng `requireAdmin`: `profile.updateCompany`, `profile.updateDept`, `profile.createCompany`, `profile.createDept`, `profile.setUserDept`, `profile.getCompany`, `profile.invalidate` trong cùng file.
- So sánh với cơ chế đúng: `backend/src/main/admin/admin-middleware.ts:21-43` — `requireAdmin` cho route HTTP `/admin/api/*` có check `session.role !== 'admin'` thật (403 nếu sai role) — chứng minh pattern đúng đã tồn tại ở nơi khác nhưng không được áp dụng cho RPC layer.

## Đề xuất fix

1. Sửa `requireAdmin(ctx)` trong `profile-rpc-handler.ts` để đọc `ctx.userRole` (cần đảm bảo `RpcExecutionContext`/`ctx` mang theo role từ `AuthManager`/session — nếu chưa có, bổ sung field này khi build RPC context) và trả lỗi `FORBIDDEN`/403 nếu `role !== 'admin'`.
2. Viết test cho từng RPC method ở trên với user role `developer`/`lead` gọi và assert bị từ chối.
3. Rà soát toàn bộ codebase tìm các chỗ khác gọi `requireAdmin`-style helper tương tự chỉ check login mà không check role (xem [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md) — cùng pattern lỗi ở `project-rpc-handler.ts`).

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.11 (F32), §5.12 (F33), §6 mục 1 (Top 10)
- Doc gốc: `docs/features/F32-team-rbac.md`, `docs/features/F33-user-profile-hierarchy.md`
- Liên quan: [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md), [BUG-BE-HLD-003](./BUG-BE-HLD-003-rbac-fragmented-no-policy-table.md)
