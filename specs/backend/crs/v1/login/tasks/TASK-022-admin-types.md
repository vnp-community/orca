# TASK-022: Tạo `src/main/admin/admin-types.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.1
**Depends on:** (không có)
**Blocks:** TASK-023, TASK-025, TASK-026, TASK-027, TASK-028, TASK-029, TASK-030

---

## Mục tiêu

Tạo types cho admin panel subsystem — AdminStats, AuditEvent, PolicyInput.

---

## File cần tạo

**Path:** `src/main/admin/admin-types.ts`

---

## Nội dung

```typescript
// src/main/admin/admin-types.ts

export type AdminStats = {
  totalUsers:     number   // Tổng số users (kể cả inactive)
  activeUsers:    number   // Users có is_active = 1
  activeSessions: number   // Sessions chưa hết hạn
  pairedDevices:  number   // DeviceRegistry count (stub: 0)
}

export type AuditEvent = {
  id:         number
  createdAt:  number
  userId:     string | null
  userEmail:  string | null
  action:     string
  detail:     Record<string, unknown> | null
  ipAddress:  string | null
}

export type AuditLogInput = {
  userId?:    string
  userEmail?: string
  action:     string
  ipAddress?: string
  detail?:    Record<string, unknown>
}

export type AuditQueryFilter = {
  userId?:   string
  action?:   string
  from?:     number   // timestamp ms
  to?:       number   // timestamp ms
  limit?:    number   // default 100
  offset?:   number
}

export type PolicyInput = {
  name:                  string
  teams?:                string[]
  roles?:                string[]
  users?:                string[]
  allowedServers?:       string | string[]   // '*' hoặc array of server IDs
  allowedProjects?:      string | string[]
  agentTrust?:           'minimal' | 'standard' | 'full'
  canCreateWorktrees?:   boolean
  canDeleteWorktrees?:   boolean
  canAccessProduction?:  boolean
}

// Known audit action constants (không phải enum, dùng string để extensible)
export const AUDIT_ACTIONS = {
  LOGIN_SUCCESS:      'login.success',
  LOGIN_FAILURE:      'login.failure',
  LOGOUT:             'logout',
  SSO_LOGIN:          'sso.login',
  USER_CREATE:        'user.create',
  USER_DEACTIVATE:    'user.deactivate',
  SESSION_KILL:       'session.kill',
  SESSION_KILL_ALL:   'session.kill_all',
  SSH_CONNECT:        'ssh.connect',
  SSH_DISCONNECT:     'ssh.disconnect',
  SERVER_START:       'server.start',
  SERVER_STOP:        'server.stop',
} as const
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] Export: `AdminStats`, `AuditEvent`, `AuditLogInput`, `AuditQueryFilter`, `PolicyInput`, `AUDIT_ACTIONS`
- [x] `AUDIT_ACTIONS` là `const` object (không dùng enum) để values là string literals
