# TASK-FE-017 — Tạo `SessionsPage.tsx` + Tests

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md) §4.1
**Depends on:** TASK-FE-013, TASK-FE-014
**Blocks:** TASK-FE-020
**Effort:** M (~30 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo trang quản lý sessions đang active. Admin có thể kill từng session hoặc kill tất cả.

---

## Files cần tạo

### `src/renderer/src/components/admin/SessionsPage.tsx` [NEW]

Features:
- Fetch sessions từ `fetchAdminSessions()`
- Table: User Email, IP Address, Started (relative time), Last Seen (relative time), Action
- [Kill] button mỗi row → `killAdminSession(sessionId)` → remove khỏi list
- [Kill All] button header → kill tất cả sessions một lượt
- Optimistic update: remove session khỏi UI ngay khi click (không chờ API)

```typescript
// relative time helper
function formatRelative(ts: number): string {
  const diff = Date.now() - ts
  const h = Math.floor(diff / 3_600_000)
  const m = Math.floor((diff % 3_600_000) / 60_000)
  if (h > 0) return `${h}h ago`
  return `${m}m ago`
}
```

### `src/renderer/src/components/admin/__tests__/SessionsPage.test.tsx` [NEW]

Test cases (3 tests):
- Renders sessions table với user emails
- Kill button gọi killAdminSession với đúng sessionId
- Sau kill → session bị remove khỏi list UI

---

## Verify

```bash
npx vitest run src/renderer/src/components/admin/__tests__/SessionsPage.test.tsx
# Expected: 3 pass
```
