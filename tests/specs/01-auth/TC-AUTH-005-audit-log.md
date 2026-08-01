# TC-AUTH-005 — Audit Log

**BL Reference:** BL-AUTH-05  
**Flow Reference:** docs/flows/logic/auth.md#BL-AUTH-05  
**Priority:** P1  
**Type:** Integration  
**Actor:** Admin

---

## TC-AUTH-005-01: Login success ghi audit log

**Priority:** P0

### Steps
1. Login thành công với `active@test.com`
2. `GET /admin/api/audit?action=login.success&limit=1`

### Expected Results
- Audit entry với:
  - `action: 'login.success'`
  - `actor_id: <userId>`
  - `target_type: 'user'`
  - `ip_address: <request IP>`
  - `created_at: <recent timestamp>`

### Assertions
```
await loginUser('active@test.com', 'Password123!')
entries = GET '/admin/api/audit?action=login.success&limit=1'
entry = entries.data[0]
assert entry.action === 'login.success'
assert entry.actorId === userId
assert entry.ipAddress !== null
```

---

## TC-AUTH-005-02: Login fail ghi audit log

**Priority:** P0

### Steps
1. Gửi request login với wrong password
2. Kiểm tra audit log

### Expected Results
- `action: 'login.fail'`
- `actor_id: null` (unknown user)
- `metadata.ip: <request IP>`

---

## TC-AUTH-005-03: User create ghi audit log

**Priority:** P0

### Steps
1. Admin tạo user mới
2. Kiểm tra audit log

### Expected Results
- `action: 'user.create'`
- `actor_id: <adminId>`
- `target_type: 'user'`
- `target_id: <newUserId>`

---

## TC-AUTH-005-04: Session kill ghi audit log

**Priority:** P0

### Steps
1. Admin kill session
2. Kiểm tra audit log

### Expected Results
- `action: 'session.kill'`
- `actor_id: <adminId>`
- `target_type: 'session'`
- `target_id: <sessionId>`

---

## TC-AUTH-005-05: User deactivate ghi audit log

**Priority:** P0

### Steps
1. Admin deactivate user
2. Kiểm tra audit log

### Expected Results
- `action: 'user.deactivate'`
- `actor_id: <adminId>`
- `target_id: <deactivatedUserId>`

---

## TC-AUTH-005-06: Audit log query — filter by action

**Priority:** P1

### Steps
1. `GET /admin/api/audit?action=login.fail&from=2026-07-01&page=1`
2. Kiểm tra results

### Expected Results
- Chỉ entries với `action='login.fail'`
- Chỉ entries từ ngày 2026-07-01 trở đi
- Paginated response

### Assertions
```
response = GET '/admin/api/audit?action=login.fail&from=2026-07-01'
response.body.data.forEach(e => {
  assert e.action === 'login.fail'
  assert new Date(e.createdAt) >= new Date('2026-07-01')
})
```

---

## TC-AUTH-005-07: Audit log export CSV

**Priority:** P1

### Steps
1. `GET /admin/api/audit/export?format=csv`
2. Kiểm tra response

### Expected Results
- `Content-Type: text/csv`
- Response body là valid CSV
- Streaming response (no memory buffering)

### Assertions
```
response = GET '/admin/api/audit/export?format=csv'
assert response.headers['content-type'].includes('text/csv')
assert isValidCsv(response.body)
```

---

## TC-AUTH-005-08: requireAdmin — user thường không thể xem audit log

**Priority:** P0

### Steps
1. Login với user thường
2. `GET /admin/api/audit`

### Expected Results
- HTTP 403 Forbidden

### Assertions
```
assert GET('/admin/api/audit', { cookie: userSession }).status === 403
```

---

*TC-AUTH-005 — Orca v5.0 — 2026-08-01*
