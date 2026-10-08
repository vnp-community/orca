# TC-AUTH-004 — Admin User CRUD & Session Kill

**BL Reference:** BL-AUTH-04  
**Flow Reference:** docs/flows/logic/auth.md#BL-AUTH-04  
**Priority:** P0  
**Type:** Integration + Security  
**Actor:** Admin

---

## Preconditions
- Admin account: `admin@test.com` / `Admin123!` (role='admin')
- Normal user: `user@test.com` / `User123!` (role='user')
- `ORCA_MULTI_USER=1`

---

## TC-AUTH-004-01: Admin tạo user mới

**Priority:** P0

### Steps
1. Login với admin credentials
2. `POST /admin/api/users` với body:
   ```json
   { "email": "new@test.com", "name": "New User", "role": "user", "password": "NewPass123!" }
   ```

### Expected Results
- HTTP 201 Created
- User được INSERT vào `orca_users`
- Password được hash với bcrypt 12 rounds
- audit_log: `user.create`

### Assertions
```
assert response.status === 201
assert response.body.email === 'new@test.com'
user = db.users.find({ email: 'new@test.com' })
assert user !== null
assert await bcrypt.compare('NewPass123!', user.passwordHash)
assert db.auditLog.lastEntry.action === 'user.create'
```

---

## TC-AUTH-004-02: requireAdmin guard — user thường bị 403

**Priority:** P0  
**Security:** CRITICAL

### Steps
1. Login với user thường (role='user')
2. `POST /admin/api/users` với body hợp lệ

### Expected Results
- HTTP 403 Forbidden
- Body: `{ "error": "forbidden" }` hoặc tương đương

### Assertions
```
loginAsUser(userSession)
response = POST '/admin/api/users' { ... }
assert response.status === 403
```

---

## TC-AUTH-004-03: requireAdmin guard — unauthenticated bị 401

**Priority:** P0

### Steps
1. `POST /admin/api/users` KHÔNG có cookie

### Expected Results
- HTTP 401 Unauthorized

### Assertions
```
response = POST '/admin/api/users' { body, noCookie: true }
assert response.status === 401
```

---

## TC-AUTH-004-04: Admin deactivate user

**Priority:** P0

### Steps
1. Login admin
2. `PATCH /admin/api/users/{userId}` với `{ "is_active": false }`
3. Thử login với account bị deactivate

### Expected Results
- User deactivated: `is_active = 0`
- Tất cả sessions của user bị DELETE
- Child process bị SIGTERM
- audit_log: `user.deactivate`
- Login sau đó: 403 account_inactive

### Assertions
```
PATCH '/admin/api/users/{userId}' { is_active: false }
assert db.users.find({ id: userId }).isActive === false
assert db.sessions.count({ userId }) === 0
assert SessionManager.getProcess(userId) === null
// Try login
response = POST '/auth/local' { email: targetUser.email, password }
assert response.status === 403
assert response.body.error === 'account_inactive'
```

---

## TC-AUTH-004-05: Admin kill specific session

**Priority:** P0

### Steps
1. User A login → 2 sessions (2 tabs)
2. Admin: `DELETE /admin/api/sessions/{sessionId}` (session 1)
3. Kiểm tra session 1 bị terminate, session 2 vẫn active

### Expected Results
- Session 1: deleted từ DB
- WS connection của session 1: dropped
- audit_log: `session.kill`
- Session 2: vẫn hoạt động

### Assertions
```
// Delete session 1
DELETE `/admin/api/sessions/${session1.id}`
assert db.sessions.count({ id: session1.id }) === 0
assert ws1.readyState === WebSocket.CLOSED
assert db.sessions.count({ id: session2.id }) === 1
assert db.auditLog.lastEntry.action === 'session.kill'
```

---

## TC-AUTH-004-06: Admin list users — pagination + filter

**Priority:** P1

### Steps
1. Login admin
2. `GET /admin/api/users?page=1&limit=10&role=user&status=active`

### Expected Results
- HTTP 200 với paginated list
- Chỉ users có role='user' và is_active=1
- Pagination metadata: `{ total, page, limit, pages }`

### Assertions
```
response = GET '/admin/api/users?role=user&status=active'
assert response.status === 200
response.body.data.forEach(u => {
  assert u.role === 'user'
  assert u.isActive === true
})
assert response.body.pagination.page === 1
```

---

## TC-AUTH-004-07: Email uniqueness constraint

**Priority:** P1

### Steps
1. Admin tạo user với email `existing@test.com`
2. Admin tạo user thứ hai với cùng email `existing@test.com`

### Expected Results
- Request đầu: 201 Created
- Request thứ hai: 409 Conflict

### Assertions
```
POST '/admin/api/users' { email: 'dup@test.com', ... } // first: 201
POST '/admin/api/users' { email: 'dup@test.com', ... } // second: 409
assert response2.status === 409
```

---

## TC-AUTH-004-08: First-run — auto-create admin user

**Priority:** P1

### Steps
1. Start Orca Server với empty database (no users)
2. Kiểm tra stdout

### Expected Results
- Admin user tự động được tạo
- Credentials in ra stdout: `Admin email: admin@... | Password: <generated>`
- User có role='admin', is_active=1

### Assertions
```
stdout = captureOutput(startOrcaServer())
assert stdout.includes('Admin email:')
assert stdout.includes('Password:')
adminUser = db.users.find({ role: 'admin' })
assert adminUser !== null
```

---

*TC-AUTH-004 — Orca v5.0 — 2026-08-01*
