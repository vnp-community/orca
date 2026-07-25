# TASK-FE-018 — Tạo `PoliciesPage.tsx` + `PolicyForm.tsx`

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md)
**Depends on:** TASK-FE-013, TASK-FE-014
**Blocks:** TASK-FE-020
**Effort:** M (~35 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo access policy management pages. Policies là RBAC rules xác định ai được SSH vào server nào.

---

## Files cần tạo

### `src/renderer/src/components/admin/PoliciesPage.tsx` [NEW]

Features:
- Fetch policies từ `fetchAdminPolicies()`
- Hiển thị mỗi policy dạng card:
  - Policy name
  - Applies to: teams=[], roles=[]
  - Allowed servers
  - Permissions: canCreateWorktrees, canAccessProduction
- [Edit] button → navigate `/admin/policies/:id/edit`
- [Delete] button → `deleteAdminPolicy(id)` → remove từ list
- [+ New Policy] button → navigate `/admin/policies/new`

### `src/renderer/src/components/admin/PolicyForm.tsx` [NEW]

Fields:
- Name (text)
- Teams (comma-separated, hoặc tag input)
- Roles (multi-select: developer, lead, admin)
- Allowed Servers: "* (all)" hoặc danh sách server IDs
- Permissions:
  - ☑ Can create worktrees
  - ☑ Can delete worktrees
  - ☐ Can access production

Submit:
- Create: `createAdminPolicy(data)` → redirect `/admin/policies`
- Edit: `updateAdminPolicy(id, data)` → redirect `/admin/policies`

---

## Verify

```bash
npx tsc --noEmit
# Compile check
```
