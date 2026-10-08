# TC-AUTH-001 — Local Login (email + password)

**BL Reference:** BL-AUTH-01  
**Flow Reference:** docs/flows/logic/auth.md#BL-AUTH-01  
**Priority:** P0  
**Type:** Integration  
**Actor:** All users (Web Server mode)

---

## Preconditions

- `ORCA_MULTI_USER=1` env var được set
- Migration 0005 đã applied (`orca_users`, `orca_sessions` tables tồn tại)
- Test user accounts được seed:
  - `active@test.com` / `Password123!` — is_active=1, role='user'
  - `admin@test.com` / `Admin123!` — is_active=1, role='admin'
  - `inactive@test.com` / `Pass123!` — is_active=0

---

## Test Data

| Field | Value |
|-------|-------|
| Valid email | active@test.com |
| Valid password | Password123! |
| Invalid email | notexist@test.com |
| Wrong password | WrongPass! |
| Inactive account | inactive@test.com |
| Bcrypt rounds | 12 |
| Session TTL | 8 hours |

---

## TC-AUTH-001-01: Login thành công — Happy Path

**Type:** Integration  
**Priority:** P0

### Steps
1. `POST /auth/local` với body `{ "email": "active@test.com", "password": "Password123!" }`
2. Kiểm tra response code
3. Kiểm tra Set-Cookie header
4. Kiểm tra response body
5. Kiểm tra DB state

### Expected Results
- HTTP 200 OK
- `Set-Cookie: orca_session=<token>; HttpOnly; SameSite=Strict; Path=/`
- Response body: `{ "id": "<uuid>", "email": "active@test.com", "name": "...", "role": "user" }`
- DB: `SELECT COUNT(*) FROM orca_sessions WHERE userId = <userId>` → 1
- DB: `SELECT action FROM orca_audit_log ORDER BY created_at DESC LIMIT 1` → `login.success`

### Assertions
```
assert response.status === 200
assert response.headers['set-cookie'] matches /orca_session=\w+; HttpOnly/
assert response.body.email === 'active@test.com'
assert db.sessions.count({ userId }) === 1
assert db.auditLog.lastEntry.action === 'login.success'
```

---

## TC-AUTH-001-02: Login thất bại — Email không tồn tại

**Type:** Integration  
**Priority:** P0  
**Security:** Phải CÙNG error message với wrong password (không tiết lộ)

### Steps
1. `POST /auth/local` với `{ "email": "notexist@test.com", "password": "AnyPass123!" }`

### Expected Results
- HTTP 401 Unauthorized
- Body: `{ "error": "invalid_credentials" }`
- KHÔNG có Set-Cookie header
- DB: audit_log ghi `login.fail`

### Assertions
```
assert response.status === 401
assert response.body.error === 'invalid_credentials'
assert response.headers['set-cookie'] === undefined
assert db.auditLog.lastEntry.action === 'login.fail'
```

---

## TC-AUTH-001-03: Login thất bại — Password sai

**Type:** Integration  
**Priority:** P0  
**Security:** CÙNG error với email không tồn tại

### Steps
1. `POST /auth/local` với `{ "email": "active@test.com", "password": "WrongPass!" }`

### Expected Results
- HTTP 401 Unauthorized
- Body: `{ "error": "invalid_credentials" }` (giống TC-AUTH-001-02!)
- DB: audit_log ghi `login.fail`

### Assertions
```
assert response.status === 401
assert response.body.error === 'invalid_credentials'
// Security: same response as "email not found"
assert TC_AUTH_001_02.body === TC_AUTH_001_03.body
```

---

## TC-AUTH-001-04: Login thất bại — Account deactivated

**Type:** Integration  
**Priority:** P0

### Steps
1. `POST /auth/local` với `{ "email": "inactive@test.com", "password": "Pass123!" }`

### Expected Results
- HTTP 403 Forbidden
- Body: `{ "error": "account_inactive" }`

### Assertions
```
assert response.status === 403
assert response.body.error === 'account_inactive'
```

---

## TC-AUTH-001-05: Login thất bại — Validation error

**Type:** Unit  
**Priority:** P1

### Steps (a) Invalid email format
1. `POST /auth/local` với `{ "email": "not-an-email", "password": "Pass123!" }`

### Expected Results (a)
- HTTP 400 Bad Request
- Body: validation error message

### Steps (b) Password quá ngắn (< 8 chars)
1. `POST /auth/local` với `{ "email": "user@test.com", "password": "short" }`

### Expected Results (b)
- HTTP 400 Bad Request

### Assertions
```
assert response.status === 400
```

---

## TC-AUTH-001-06: Rate limiting — 10 requests/minute per IP

**Type:** Integration  
**Priority:** P0  
**Security:** Quan trọng để chống brute force

### Steps
1. Gửi 10 request login thất bại liên tiếp từ cùng IP
2. Gửi request thứ 11

### Expected Results
- Request 1-10: 401 Unauthorized
- Request 11: 429 Too Many Requests
- Body: `{ "error": "too_many_attempts" }`

### Assertions
```
responses[0..9].every(r => r.status === 401)
assert responses[10].status === 429
assert responses[10].body.error === 'too_many_attempts'
```

---

## TC-AUTH-001-07: ORCA_MULTI_USER=0 — Endpoint disabled

**Type:** Integration  
**Priority:** P1

### Preconditions
- `ORCA_MULTI_USER=0`

### Steps
1. `POST /auth/local` với valid credentials

### Expected Results
- HTTP 404 Not Found

### Assertions
```
assert response.status === 404
```

---

## TC-AUTH-001-08: Cookie security properties

**Type:** Security  
**Priority:** P0

### Steps
1. Login thành công
2. Kiểm tra chi tiết cookie attributes

### Expected Results
- `HttpOnly` flag: present (JavaScript không thể đọc)
- `SameSite=Strict` hoặc `SameSite=Lax`: present
- `Path=/`: present
- Cookie value: hex string 64 chars (32 bytes random)

### Assertions
```
cookie = parseCookie(response.headers['set-cookie'])
assert cookie.httpOnly === true
assert ['Strict','Lax'].includes(cookie.sameSite)
assert cookie.path === '/'
assert /^[a-f0-9]{64}$/.test(cookie.value)
```

---

## TC-AUTH-001-09: Timing attack prevention — bcrypt 12 rounds

**Type:** Security + Performance  
**Priority:** P1

### Steps
1. Đo thời gian response khi email tồn tại nhưng password sai
2. Đo thời gian response khi email không tồn tại

### Expected Results
- Cả 2 trường hợp có timing tương tự (~ 300ms, không thể distinguish qua timing)
- Độ lệch < 50ms

### Assertions
```
timingDiff = Math.abs(timeWrongPass - timeNoEmail)
assert timingDiff < 50 // ms
```

---

## TC-AUTH-001-10: Session token — cryptographic randomness

**Type:** Security  
**Priority:** P0

### Steps
1. Login 3 lần với cùng credentials
2. So sánh session tokens

### Expected Results
- 3 tokens đều khác nhau
- Mỗi token là 64 hex chars (crypto.randomBytes(32))

### Assertions
```
tokens = [token1, token2, token3]
assert new Set(tokens).size === 3 // all unique
tokens.forEach(t => assert /^[a-f0-9]{64}$/.test(t))
```

---

## Cleanup

Sau mỗi test:
```sql
DELETE FROM orca_sessions WHERE userId IN (SELECT id FROM orca_users WHERE email LIKE '%@test.com');
DELETE FROM orca_audit_log WHERE actor_id IN (SELECT id FROM orca_users WHERE email LIKE '%@test.com');
```

---

*TC-AUTH-001 — Orca v5.0 Test Cases — 2026-08-01*
