# BL-AUTH-05: Audit Log Ghi nhận Action

**Domain:** Authentication & User Management  
**Priority:** P1  
**Actor chính:** Admin  
**Tham chiếu:** FR-13.2, UR-113, F25

---

## Mô tả

Hệ thống ghi lại mọi action quan trọng vào `orca_audit_log`. Audit log là append-only, không có DELETE API. Admin có thể xem, filter và export.

## Events được ghi

| Action | Trigger | Metadata |
|--------|---------|----------|
| `login.success` | POST /auth/local success | `{ ip, userAgent }` |
| `login.fail` | Wrong password | `{ ip, email }` |
| `user.create` | Admin tạo user | `{ targetEmail, role }` |
| `user.deactivate` | Admin vô hiệu hóa | `{ targetId }` |
| `user.role_change` | Admin đổi role | `{ from, to }` |
| `session.kill` | Admin kill session | `{ sessionId, targetUserId }` |
| `ssh.connect` | User kết nối SSH host | `{ host, linuxUser }` |

## Schema

```sql
CREATE TABLE orca_audit_log (
  id          TEXT PRIMARY KEY,      -- UUID v4
  actor_id    TEXT,                  -- userId của người thực hiện (NULL = system)
  action      TEXT NOT NULL,         -- "login.success", "user.create", etc.
  target_type TEXT,                  -- "user", "session", "ssh_host"
  target_id   TEXT,                  -- ID của target entity
  metadata    TEXT,                  -- JSON string (extra context)
  ip_address  TEXT,                  -- IP của request
  created_at  TEXT NOT NULL          -- ISO 8601 UTC timestamp
);
CREATE INDEX idx_audit_actor ON orca_audit_log(actor_id);
CREATE INDEX idx_audit_action ON orca_audit_log(action);
CREATE INDEX idx_audit_created ON orca_audit_log(created_at);
```

## Admin Query API

```
GET /admin/api/audit?page=1&limit=50&action=login.fail&userId=xxx&from=2026-07-01&to=2026-07-31
```

Response: paginated list of audit entries.

## Export

```
GET /admin/api/audit/export?format=csv
```

Returns CSV stream với headers: id, actor_email, action, target_type, target_id, metadata, ip_address, created_at.

## Source References

- `src/main/auth/audit-log.ts` — writeAudit() helper
- `src/main/server/admin-router.ts` — GET /admin/api/audit
- `src/main/db/migrations/0005-auth-schema.ts` — orca_audit_log DDL
