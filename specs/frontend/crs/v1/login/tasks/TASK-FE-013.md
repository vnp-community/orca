# TASK-FE-013 — Tạo `admin-api-client.ts` + Tests

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md) §4.1, §3.1
**Depends on:** TASK-FE-001
**Blocks:** TASK-FE-014..TASK-FE-019
**Effort:** S (~30 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo HTTP client wrapper đầy đủ cho tất cả `/admin/api/*` endpoints.
Tất cả functions đều dùng `credentials: 'include'` và throw descriptive errors khi 401/403.

---

## Files cần tạo

### `src/renderer/src/components/admin/admin-api-client.ts` [NEW]

Implement đầy đủ theo spec tại [SOL-FE-LG-003 §4.1](../solutions/SOL-FE-LG-003-admin-panel.md).

Các types:
```typescript
export type AdminUser = { id, email, name, role, provider, isActive, lastLoginAt }
export type AdminStats = { totalUsers, activeSessions, sshConnections, pairedDevices }
export type AdminSession = { sessionId, userId, userEmail, ipAddress, userAgent?, createdAt, lastSeenAt }
export type AdminPolicy = { id, name, teams, roles, allowedServers, canCreateWorktrees, canAccessProduction }
export type AuditEntry = { id, createdAt, userId?, userEmail?, action, detail?, ipAddress? }
```

Các functions:
- `fetchAdminStats()` → GET /admin/api/stats
- `fetchAdminUsers()` → GET /admin/api/users
- `createAdminUser(data)` → POST /admin/api/users
- `updateAdminUser(id, data)` → PATCH /admin/api/users/:id
- `deactivateAdminUser(id)` → DELETE /admin/api/users/:id
- `fetchAdminSessions()` → GET /admin/api/sessions
- `killAdminSession(sessionId)` → DELETE /admin/api/sessions/:id
- `fetchAdminPolicies()` → GET /admin/api/policies
- `createAdminPolicy(data)` → POST /admin/api/policies
- `updateAdminPolicy(id, data)` → PATCH /admin/api/policies/:id
- `deleteAdminPolicy(id)` → DELETE /admin/api/policies/:id
- `fetchAdminAudit(filter?)` → GET /admin/api/audit?from=&to=&action=

### `src/renderer/src/components/admin/__tests__/admin-api-client.test.ts` [NEW]

Sao chép test spec từ [SOL-FE-LG-003 §3.1](../solutions/SOL-FE-LG-003-admin-panel.md).

Test cases (5 tests):
- `fetchAdminStats` trả về stats
- `fetchAdminStats` throw trên 403
- `fetchAdminUsers` gọi đúng endpoint với credentials
- `createAdminUser` gọi POST với body
- `killAdminSession` gọi DELETE
- `fetchAdminAudit` truyền filter params đúng

---

## Verify

```bash
npx vitest run src/renderer/src/components/admin/__tests__/admin-api-client.test.ts
# Expected: 5+ pass
```
