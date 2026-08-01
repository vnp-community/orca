# F25 — Admin Panel

| Trường | Giá trị |
|--------|---------|
| **ID** | F25 |
| **Tên** | Admin Panel |
| **Ưu tiên** | P1 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [login/CR-LOGIN-004](../crs/v1/login/CR-LOGIN-004-admin.md) |
| **TDD** | [TDD-11: Admin Panel](../specs/backend/tdd/11-web-server-mode.md) |
| **Phiên bản** | v4.0+ |
| **ADR References** | — |
| **HLD References** | C3.1 |

---

## Mô tả

Admin Panel là **SPA riêng biệt** (`/admin`) cho phép quản trị viên quản lý users, sessions, access policies, và xem audit log — chỉ accessible bởi role `admin`.

---

## Tính năng chi tiết

### Backend API (`/admin/api/*`)
Tất cả routes bảo vệ bởi `requireAdmin` middleware (role === 'admin', 403 nếu không):

| Endpoint | Mô tả |
|----------|-------|
| `GET /admin/api/stats` | Total users, active sessions, paired devices |
| `GET /admin/api/users` | Danh sách tất cả users |
| `POST /admin/api/users` | Tạo user mới |
| `DELETE /admin/api/users/:id` | Deactivate user + kill tất cả sessions |
| `DELETE /admin/api/sessions/:id` | Kill 1 session cụ thể |
| `DELETE /admin/api/users/:id/sessions` | Kill tất cả sessions của user |
| `GET /admin/api/audit` | Audit log với filter `?userId=&action=&from=&to=` |

### Audit Log
Mọi admin action được ghi sync vào `orca_audit_log`:
```
login.success / login.failure / logout
user.create / user.deactivate
session.kill / sessions.kill_all
ssh.connect / ssh.disconnect
```

### Admin SPA (React)
- **AdminDashboard** — stats, quick actions
- **UsersPage** — search, filter role/status, deactivate
- **UserForm** — create/edit user
- **SessionsPage** — xem tất cả active sessions, kill
- **AuditPage** — audit log với pagination + filter
- **PoliciesPage** — RBAC access policy CRUD

### First-Run Setup
```
server-bootstrap → ensureFirstAdminUser():
  Nếu không có admin → tạo admin với random password
  → In ra stdout (one-time):
     Email:    admin@orca.local
     Password: a1b2c3d4e5f6... (auto-generated)
```

---

## Tiêu chí chấp nhận

- [x] `/admin/` chỉ accessible bởi role `admin` (403 otherwise)
- [x] List, create, deactivate users hoạt động
- [x] Deactivate user → kill tất cả sessions ngay lập tức
- [x] Kill session từ admin → WS disconnect ngay lập tức
- [x] Audit log ghi đầy đủ actions
- [x] First-run: in admin credentials ra stdout khi không có admin
- [x] Admin SPA có search/filter user hoạt động
- [x] `GET /admin/api/stats` trả về total users, active sessions
- [ ] Access Policy SSH enforcement — DEFERRED Phase 3

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Admin middleware | `src/main/admin/admin-middleware.ts` |
| Admin user handlers | `src/main/admin/admin-user-handlers.ts` |
| Admin session handlers | `src/main/admin/admin-session-handlers.ts` |
| Admin stats | `src/main/admin/admin-stats-handler.ts` |
| Audit handlers | `src/main/admin/admin-audit-handlers.ts` |
| Admin router | `src/main/admin/admin-router.ts` |
| Audit logger | `src/main/admin/audit-logger.ts` |
| First run setup | `src/main/admin/first-run-setup.ts` |
| Admin SPA entry | `src/renderer/admin-index.html` + `src/renderer/src/admin/admin-main.tsx` |
| Admin components | `src/renderer/src/components/admin/` |

**Tests:** 44 backend + 43 frontend = **87 tests**
