# TC-AUTH-002 — Session Management & Isolation

**BL Reference:** BL-AUTH-02  
**Flow Reference:** docs/flows/logic/auth.md#BL-AUTH-02  
**Priority:** P0  
**Type:** Integration + Security  
**Actor:** All users

---

## Preconditions

- `ORCA_MULTI_USER=1`
- 2 active users: userA@test.com, userB@test.com
- Mỗi user đã login và có session cookie

---

## TC-AUTH-002-01: WebSocket auth — valid session

**Priority:** P0

### Steps
1. Login để lấy `orca_session` cookie
2. Connect WebSocket `ws://orca:6768` với cookie
3. Kiểm tra connection established

### Expected Results
- WS connection accepted
- `orca_sessions.lastSeenAt` được update
- Child process được fork cho userId

### Assertions
```
wsClient.connect({ cookie: sessionCookie })
assert wsClient.readyState === WebSocket.OPEN
assert db.sessions.find({ token }).lastSeenAt > previousLastSeenAt
```

---

## TC-AUTH-002-02: WebSocket auth — invalid/expired session

**Priority:** P0

### Steps
1. Connect WebSocket với forged cookie `orca_session=aaaabbbbcccc...`
2. Kiểm tra connection bị reject

### Expected Results
- WS connection closed với code 401
- Không có child process được fork

### Assertions
```
wsClient.connect({ cookie: 'orca_session=fakefakefake' })
assert wsClient.closeCode === 401
```

---

## TC-AUTH-002-03: Session TTL — 8 giờ

**Priority:** P0

### Steps
1. Login và lấy session token
2. Manually update DB: `UPDATE orca_sessions SET expires_at = datetime('now', '-1 second') WHERE token = ?`
3. Connect WebSocket với expired token

### Expected Results
- WS connection closed với code 401 (expired session)

### Assertions
```
db.sessions.update({ token }, { expiresAt: new Date(Date.now() - 1000) })
wsClient.connect({ cookie: `orca_session=${token}` })
assert wsClient.closeCode === 401
```

---

## TC-AUTH-002-04: Session isolation — User A không access User B data

**Priority:** P0  
**Security:** CRITICAL

### Steps
1. User A login, nhận session A
2. User B login, nhận session B
3. Dùng session A gọi RPC: lấy danh sách worktrees
4. Dùng session B gọi RPC: lấy danh sách worktrees
5. Kiểm tra User A không thấy worktrees của User B

### Expected Results
- User A chỉ thấy worktrees của mình
- User B chỉ thấy worktrees của mình
- Không có data leakage

### Assertions
```
worktreesA = rpcCall(sessionA, 'worktree.list')
worktreesB = rpcCall(sessionB, 'worktree.list')
worktreesA.forEach(wt => assert wt.userId === userA.id)
worktreesB.forEach(wt => assert wt.userId === userB.id)
// No overlap
assert !worktreesA.some(wt => worktreesB.includes(wt))
```

---

## TC-AUTH-002-05: Child process fork — per user isolation

**Priority:** P0

### Steps
1. User A connect WebSocket → child process A fork
2. User B connect WebSocket → child process B fork
3. Kiểm tra 2 process riêng biệt

### Expected Results
- 2 child processes với PID khác nhau
- Mỗi process có Unix socket riêng:
  - `~/.orca/users/{userA}/orca.sock`
  - `~/.orca/users/{userB}/orca.sock`

### Assertions
```
processA = SessionManager.getProcess(userA.id)
processB = SessionManager.getProcess(userB.id)
assert processA.pid !== processB.pid
assert processA.socketPath !== processB.socketPath
```

---

## TC-AUTH-002-06: Child process respawn — crash recovery

**Priority:** P1

### Steps
1. User A connect WebSocket → child process A fork
2. Simulate crash: `process.kill(childA.pid, 'SIGKILL')`
3. Gửi message qua WS trong 5s
4. Kiểm tra child process được respawn

### Expected Results
- Child process bị kill
- SessionManager detect exit → respawn (max 3 attempts)
- WS connection vẫn connected (seamless recovery)
- Sau 3 lần crash: session marked broken, admin alert

### Assertions
```
childPidBefore = processA.pid
killProcess(processA.pid)
await delay(2000)
childPidAfter = SessionManager.getProcess(userA.id).pid
assert childPidAfter !== childPidBefore // new pid = respawned
assert SessionManager.respawnCount(userA.id) === 1
```

---

## TC-AUTH-002-07: Idle timeout — 4 giờ

**Priority:** P1

### Steps
1. User A connect WebSocket
2. Simulate 4h idle (advance timer)
3. Kiểm tra child process được gracefully killed

### Expected Results
- Sau 4h không có activity → child process SIGTERM
- Unix socket removed
- Session NOT deleted (user có thể login lại)

### Assertions
```
advanceTime(4 * 60 * 60 * 1000) // 4 hours
await delay(100)
assert SessionManager.getProcess(userA.id) === null
assert !fs.existsSync(userA.socketPath)
```

---

## TC-AUTH-002-08: Session lastSeenAt update per request

**Priority:** P1

### Steps
1. Login, lấy session
2. Gọi `GET /auth/me`
3. Kiểm tra `lastSeenAt` được update

### Expected Results
- `orca_sessions.last_seen_at` được update mỗi request

### Assertions
```
before = db.sessions.find({ token }).lastSeenAt
await delay(1000)
await fetch('/auth/me', { headers: { Cookie: sessionCookie } })
after = db.sessions.find({ token }).lastSeenAt
assert after > before
```

---

## TC-AUTH-002-09: GET /auth/me — trả đúng user info

**Priority:** P1

### Steps
1. Login với valid credentials
2. `GET /auth/me` với session cookie

### Expected Results
- HTTP 200
- Body: `{ id, email, name, role }`

### TC-AUTH-002-10: POST /auth/logout

**Priority:** P1

### Steps
1. Login → nhận session
2. `POST /auth/logout`
3. Thử connect WS với token cũ

### Expected Results
- Session deleted từ DB
- WS connect với token cũ → 401

### Assertions
```
await fetch('/auth/logout', { method: 'POST', headers: { Cookie } })
assert db.sessions.count({ token }) === 0
// Try old token
wsClient.connect({ cookie: `orca_session=${token}` })
assert wsClient.closeCode === 401
```

---

*TC-AUTH-002 — Orca v5.0 Test Cases — 2026-08-01*
