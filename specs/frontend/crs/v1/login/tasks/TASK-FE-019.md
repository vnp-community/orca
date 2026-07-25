# TASK-FE-019 — Tạo `AuditPage.tsx` + Tests

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md)
**Depends on:** TASK-FE-013, TASK-FE-014
**Blocks:** TASK-FE-020
**Effort:** M (~30 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo audit log viewer với date range filter và action filter.

---

## Files cần tạo

### `src/renderer/src/components/admin/AuditPage.tsx` [NEW]

Features:
- Filter form: From date, To date, Action filter (dropdown)
- Table: Time, User, Action, Detail, IP
- Fetch `fetchAdminAudit({ from, to, action })`
- "Export CSV" button (optional — download as CSV blob)
- Pagination: show 50 records per page

Action filter options (từ backend):
- All (blank)
- login.success
- login.fail
- logout
- ssh.connect
- ssh.disconnect
- user.create
- user.update
- user.deactivate
- agent.run
- policy.create
- policy.update

### `src/renderer/src/components/admin/__tests__/AuditPage.test.tsx` [NEW]

Test cases (3 tests):
- Renders audit table với action và user email
- Filter bằng action dropdown → gọi fetchAdminAudit với đúng action
- Filter bằng date range → truyền đúng from/to

---

## Verify

```bash
npx vitest run src/renderer/src/components/admin/__tests__/AuditPage.test.tsx
# Expected: 3 pass
```
