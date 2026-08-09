# BUG-BE-HLD-006 — `GET /admin/api/sessions` là stub rỗng, không list được session thật

**Mức độ:** 🟠 HIGH (Feature gap — Admin Panel)
**Status:** 🔴 Open
**Module:** `backend/src/main/admin/admin-session-handlers.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.4/F25)

---

## Mô tả

`docs/features/F25-admin-panel.md` liệt kê **SessionsPage** ("xem tất cả active sessions") là tính năng đã Phát hành.

Handler thật (`admin-session-handlers.ts:18-22`, hàm `listAllSessions`) luôn trả về:

```typescript
{ sessions: [], total: 0, note: 'Full listing not yet implemented' }
```

bất kể DB có session thật hay không. Chỉ chức năng **kill session** (`DELETE /admin/api/sessions/:id`, `DELETE /admin/api/users/:userId/sessions`) hoạt động thật — chức năng **xem** danh sách thì không.

## Hậu quả

- Admin không thể xem danh sách session đang hoạt động trên UI — SessionsPage hiển thị trống dù có user đang online.
- Không thể chọn đúng session cần kill từ danh sách thật (phải biết trước sessionId bằng cách khác).

## Bằng chứng

- `backend/src/main/admin/admin-session-handlers.ts:18-22` — hard-code trả rỗng, comment tự thừa nhận "not yet implemented".
- Route mount đúng vị trí: `backend/src/main/admin/admin-router.ts:23-45`.
- Bảng `orca_sessions` đã tồn tại đầy đủ (migration 0005) — chỉ thiếu query.

## Đề xuất fix

1. Implement `listAllSessions()` thật: `SELECT * FROM orca_sessions WHERE expires_at > now() ORDER BY created_at DESC` (kèm pagination `limit`/`offset` giống `admin-audit-handlers.ts` đã có pattern sẵn).
2. Join thêm `orca_users` để trả `userEmail` cho UI hiển thị.
3. Viết test cho endpoint với ≥2 session thật trong DB, assert trả đúng danh sách.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.4 (F25), §6 mục 5 (Top 10)
- Doc gốc: `docs/features/F25-admin-panel.md`
- Liên quan: [BUG-BE-HLD-007](./BUG-BE-HLD-007-admin-access-policies-api-missing.md)
