# TASK-FE-016 — Tạo `UsersPage.tsx` + `UserForm.tsx` + Tests

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md) §3.3
**Depends on:** TASK-FE-013, TASK-FE-014
**Blocks:** TASK-FE-020
**Effort:** L (~60 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo user management pages:
- `UsersPage`: danh sách users với search + role filter + deactivate action
- `UserForm`: form tạo/sửa user

---

## Files cần tạo

### `src/renderer/src/components/admin/UsersPage.tsx` [NEW]

Features:
- Fetch users từ `fetchAdminUsers()`
- Search input (`role="searchbox"`) → filter theo email/name
- Role filter dropdown (label `Role`) → filter theo role
- Table columns: Email, Name, Role, Provider, Status (🟢/🔴)
- Edit button → navigate `/admin/users/:id/edit`
- Deactivate button → call `deactivateAdminUser(id)` → remove từ list
- Create User link → `/admin/users/new`

### `src/renderer/src/components/admin/UserForm.tsx` [NEW]

- Route params: `id` (edit mode) hoặc không có (create mode)
- Fields: Email, Name, Role (select), Provider (select), Password (chỉ local provider), Confirm Password
- Teams + Projects (multi-value input, optional)
- Cancel → navigate back
- Submit:
  - Create mode: `createAdminUser(data)` → redirect `/admin/users`
  - Edit mode: `updateAdminUser(id, data)` → redirect `/admin/users`
- Validation: email format, password match (create mode)

### `src/renderer/src/components/admin/__tests__/UsersPage.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-003 §3.3](../solutions/SOL-FE-LG-003-admin-panel.md).

Test cases (5 tests):
- Renders user list sau load
- Filter bằng role select
- Filter bằng search text
- Call deactivateAdminUser khi click Deactivate
- Create User button hiển thị

---

## Verify

```bash
npx vitest run src/renderer/src/components/admin/__tests__/UsersPage.test.tsx
# Expected: 5 pass
```
