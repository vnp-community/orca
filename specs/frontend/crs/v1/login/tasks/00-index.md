# Frontend Tasks — Login CRs (v1)
## Index

**Version:** 1.0
**Date:** 2026-07-24
**Solutions:** [specs/frontend/crs/v1/login/solutions/](../solutions/)
**Backend Tasks:** [specs/backend/crs/v1/login/tasks/](../../../../backend/crs/v1/login/tasks/) _(nếu có)_

---

## Tóm tắt

Bộ tasks này phân rã 4 frontend solutions thành **25 tác vụ atomic** để AI thực thi tuần tự.
Mỗi task có: mô tả rõ, files cần tạo/sửa, test commands để verify.

---

## Danh sách Tasks

| Task ID | Phase | Tên | Solution | Depends On |
|---------|-------|-----|----------|------------|
| [TASK-FE-001](./TASK-FE-001.md) | 1 | Tạo `auth-types.ts` | SOL-FE-LG-001 | — |
| [TASK-FE-002](./TASK-FE-002.md) | 1 | Tạo `auth-api-client.ts` + tests | SOL-FE-LG-001 | TASK-FE-001 |
| [TASK-FE-003](./TASK-FE-003.md) | 1 | Tạo `store/slices/auth.ts` | SOL-FE-LG-001 | TASK-FE-002 |
| [TASK-FE-004](./TASK-FE-004.md) | 1 | Tạo `LoginForm.tsx` + tests | SOL-FE-LG-001 | TASK-FE-001 |
| [TASK-FE-005](./TASK-FE-005.md) | 1 | Tạo `SsoButton.tsx` + `PairCodeFallback.tsx` + tests | SOL-FE-LG-001 | TASK-FE-001 |
| [TASK-FE-006](./TASK-FE-006.md) | 1 | Tạo `LoginPage.tsx` + tests | SOL-FE-LG-001 | TASK-FE-003, TASK-FE-004, TASK-FE-005 |
| [TASK-FE-007](./TASK-FE-007.md) | 1 | Modify `main-web-bootstrap.tsx` — auth routing | SOL-FE-LG-001 | TASK-FE-006 |
| [TASK-FE-008](./TASK-FE-008.md) | 1 | Modify `store/index.ts` — register AuthSlice | SOL-FE-LG-001 | TASK-FE-003 |
| [TASK-FE-009](./TASK-FE-009.md) | 2 | Tạo `useAuthSession.ts` + `useLogout.ts` + tests | SOL-FE-LG-002 | TASK-FE-003 |
| [TASK-FE-010](./TASK-FE-010.md) | 2 | Tạo `UserRoleBadge.tsx` + tests | SOL-FE-LG-002 | — |
| [TASK-FE-011](./TASK-FE-011.md) | 2 | Tạo `UserAvatarMenu.tsx` + tests | SOL-FE-LG-002 | TASK-FE-009, TASK-FE-010 |
| [TASK-FE-012](./TASK-FE-012.md) | 2 | Modify `OrcaProfileSwitcher.tsx` — web auth display | SOL-FE-LG-002 | TASK-FE-011 |
| [TASK-FE-013](./TASK-FE-013.md) | 3 | Tạo `admin-api-client.ts` + tests | SOL-FE-LG-003 | TASK-FE-001 |
| [TASK-FE-014](./TASK-FE-014.md) | 3 | Tạo `AdminLayout.tsx` + `AdminApp.tsx` | SOL-FE-LG-003 | TASK-FE-009, TASK-FE-013 |
| [TASK-FE-015](./TASK-FE-015.md) | 3 | Tạo `useAdminStats.ts` + `AdminDashboard.tsx` + tests | SOL-FE-LG-003 | TASK-FE-013, TASK-FE-014 |
| [TASK-FE-016](./TASK-FE-016.md) | 3 | Tạo `UsersPage.tsx` + `UserForm.tsx` + tests | SOL-FE-LG-003 | TASK-FE-013, TASK-FE-014 |
| [TASK-FE-017](./TASK-FE-017.md) | 3 | Tạo `SessionsPage.tsx` + tests | SOL-FE-LG-003 | TASK-FE-013, TASK-FE-014 |
| [TASK-FE-018](./TASK-FE-018.md) | 3 | Tạo `PoliciesPage.tsx` + `PolicyForm.tsx` | SOL-FE-LG-003 | TASK-FE-013, TASK-FE-014 |
| [TASK-FE-019](./TASK-FE-019.md) | 3 | Tạo `AuditPage.tsx` + tests | SOL-FE-LG-003 | TASK-FE-013, TASK-FE-014 |
| [TASK-FE-020](./TASK-FE-020.md) | 3 | Tạo `admin-main.tsx` + `admin-index.html` + vite config | SOL-FE-LG-003 | TASK-FE-014..TASK-FE-019 |
| [TASK-FE-021](./TASK-FE-021.md) | 4 | Tạo `auth-utils.ts` (`toLinuxUsername`) + tests | SOL-FE-LG-004 | TASK-FE-001 |
| [TASK-FE-022](./TASK-FE-022.md) | 4 | Extend `store/slices/ssh.ts` — user accounts state | SOL-FE-LG-004 | TASK-FE-003 |
| [TASK-FE-023](./TASK-FE-023.md) | 4 | Tạo `SshProvisioningProgress.tsx` + `SshUserIndicator.tsx` + tests | SOL-FE-LG-004 | TASK-FE-022 |
| [TASK-FE-024](./TASK-FE-024.md) | 4 | Tạo `useSshUserAccount.ts` + `useSshProvisioning.ts` | SOL-FE-LG-004 | TASK-FE-022, TASK-FE-023 |
| [TASK-FE-025](./TASK-FE-025.md) | 4 | Integrate SSH User Indicator vào Sidebar | SOL-FE-LG-004 | TASK-FE-024 |

---

## Execution Order (Dependency Graph)

```
Phase 1 — Auth Foundation (SOL-FE-LG-001)
  TASK-FE-001 (types)
      ├──► TASK-FE-002 (api-client) ──► TASK-FE-003 (auth slice) ──► TASK-FE-008 (register slice)
      ├──► TASK-FE-004 (LoginForm)
      └──► TASK-FE-005 (SsoButton, PairCode)
               │
              [TASK-FE-003 + TASK-FE-004 + TASK-FE-005]
               └──► TASK-FE-006 (LoginPage) ──► TASK-FE-007 (bootstrap modify)

Phase 2 — User Identity (SOL-FE-LG-002)
  TASK-FE-003 ──► TASK-FE-009 (hooks)
  TASK-FE-010 (UserRoleBadge, no deps)
  [TASK-FE-009 + TASK-FE-010] ──► TASK-FE-011 (UserAvatarMenu) ──► TASK-FE-012 (OrcaProfileSwitcher)

Phase 3 — Admin Panel (SOL-FE-LG-003)
  TASK-FE-001 ──► TASK-FE-013 (admin-api-client)
  [TASK-FE-009 + TASK-FE-013] ──► TASK-FE-014 (AdminLayout, AdminApp)
  TASK-FE-014 ──► TASK-FE-015 (Dashboard)
             ──► TASK-FE-016 (UsersPage)
             ──► TASK-FE-017 (SessionsPage)
             ──► TASK-FE-018 (PoliciesPage)
             ──► TASK-FE-019 (AuditPage)
  [TASK-FE-015..019] ──► TASK-FE-020 (entry point + build config)

Phase 4 — SSH UI (SOL-FE-LG-004)
  TASK-FE-001 ──► TASK-FE-021 (auth-utils)
  TASK-FE-003 ──► TASK-FE-022 (ssh slice extend)
  TASK-FE-022 ──► TASK-FE-023 (SSH components) ──► TASK-FE-024 (hooks) ──► TASK-FE-025 (sidebar integrate)
```

---

## Test Commands

```bash
# Run tất cả frontend tests
cd /path/to/orca && npx vitest run

# Run từng phase:
npx vitest run src/renderer/src/auth/
npx vitest run src/renderer/src/store/slices/auth.test.ts
npx vitest run src/renderer/src/web/login/
npx vitest run src/renderer/src/components/auth/
npx vitest run src/renderer/src/components/admin/
npx vitest run src/renderer/src/components/ssh/
```

---

## Trạng thái

| Phase | Tasks | Status |
|-------|-------|--------|
| Phase 1 | TASK-FE-001~008 | ✅ Done (26/26 tests pass) |
| Phase 2 | TASK-FE-009~012 | ✅ Done (21/21 tests pass) |
| Phase 3 | TASK-FE-013~020 | ✅ In Progress (13-19 done) |
| Phase 4 | TASK-FE-021~025 | ✅ Done |
