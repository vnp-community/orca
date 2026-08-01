# BL-AUTH-04: Admin User CRUD & Session Kill

**Domain:** Authentication & User Management  
**Priority:** P0  
**Actor chính:** Admin  
**Tham chiếu:** FR-13.1, UR-112, F25

---

## Mô tả

Admin có thể tạo, xem, sửa, vô hiệu hóa users và kill sessions đang active thông qua Admin Panel và REST API.

## User CRUD

### Tạo User
```
POST /admin/api/users
Body: { email, name, role: 'developer'|'lead'|'admin', password }

1. requireAdmin() guard
2. Validate input (Zod schema)
3. Check email uniqueness
4. Hash password: bcrypt(password, 12)
5. INSERT orca_users
6. Log: audit_log { action: "user.create", target: userId }
7. Return 201 { id, email, name, role }
```

### Vô hiệu hóa User (không xóa — audit trail)
```
PATCH /admin/api/users/:id
Body: { is_active: false }

1. requireAdmin() guard
2. UPDATE orca_users SET is_active=0 WHERE id=?
3. DELETE FROM orca_sessions WHERE userId=?  ← kick all sessions
4. SIGTERM child process của userId (nếu đang chạy)
5. Log: audit_log { action: "user.deactivate", target: userId }
```

### Kill Session
```
DELETE /admin/api/sessions/:sessionId

1. requireAdmin() guard
2. DELETE FROM orca_sessions WHERE id=?
3. Notify WsSessionRouter: drop WS connection
4. Log: audit_log { action: "session.kill", target: sessionId }
```

## Admin Dashboard Endpoints

| Endpoint | Response |
|----------|----------|
| `GET /admin/api/users` | List all users (paginated, filter by role/status) |
| `GET /admin/api/users/:id` | User detail |
| `PATCH /admin/api/users/:id` | Update name/role/is_active |
| `GET /admin/api/sessions` | Active sessions (userId, last_seen_at, IP) |
| `DELETE /admin/api/sessions/:id` | Kill session |
| `GET /admin/api/stats` | { totalUsers, activeSessions, uptime } |

## First-Run Admin Creation

Khi `orca_users` table trống khi startup:
```
1. Generate random password (16 chars, URL-safe)
2. INSERT admin user { email: "admin@localhost", role: "admin" }
3. Print to stdout: "First run — Admin credentials: admin@localhost / <password>"
4. Log warning (never to file, only stdout)
```

## Source References

- `src/main/server/admin-router.ts`
- `src/main/auth/auth-manager.ts` — createUser(), deactivateUser()
- `src/renderer/src/web/admin/UsersPage.tsx`
- `src/renderer/src/web/admin/SessionsPage.tsx`
