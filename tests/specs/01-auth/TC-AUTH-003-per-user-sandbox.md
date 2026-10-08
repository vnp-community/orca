# TC-AUTH-003 — Per-User Process Sandbox

**BL Reference:** BL-AUTH-03  
**Flow Reference:** docs/flows/logic/auth.md#BL-AUTH-03  
**Priority:** P0  
**Type:** Integration + Security  
**Actor:** All users

---

## TC-AUTH-003-01: Sandbox initialization — per-user data paths

**Priority:** P0

### Steps
1. User A login → child process fork
2. Kiểm tra data paths được tạo

### Expected Results
- `~/.orca/users/{userA.id}/orca.sock` tồn tại
- `~/.orca/users/{userA.id}/orca.db` được tạo
- `~/.orca/users/{userA.id}/worktrees/` directory tồn tại

### Assertions
```
userDataPath = `~/.orca/users/${userA.id}`
assert fs.existsSync(`${userDataPath}/orca.sock`)
assert fs.existsSync(`${userDataPath}/orca.db`)
assert fs.existsSync(`${userDataPath}/worktrees`)
```

---

## TC-AUTH-003-02: Memory isolation — separate V8 heaps

**Priority:** P0

### Steps
1. User A và User B connect → 2 child processes
2. Ghi variable vào User A heap
3. Kiểm tra User B không thấy variable đó

### Expected Results
- Fork = copy-on-write → hoàn toàn độc lập sau write
- Global state trong child A không visible trong child B

---

## TC-AUTH-003-03: File system scoping — worktree ops scoped

**Priority:** P0  
**Security:** CRITICAL

### Steps
1. User A tạo worktree tại `/worktrees/wt-A`
2. Thử truy cập từ session của User B
3. Kiểm tra User B bị từ chối

### Expected Results
- User B không thể `worktree.list` và thấy wt-A
- Mọi file ops của User A scoped trong `~/.orca/users/{userA.id}/`

### Assertions
```
// Create worktree as User A
rpcCall(sessionA, 'worktree.create', { name: 'wt-A', ... })

// Try to access from User B
worktreesB = rpcCall(sessionB, 'worktree.list')
assert !worktreesB.find(wt => wt.name === 'wt-A')
```

---

## TC-AUTH-003-04: SSH connection isolation per child

**Priority:** P1

### Steps
1. User A connect SSH to server-1
2. User B connect SSH to server-2
3. Kiểm tra SSH connections không shared

### Expected Results
- User A's SSH pool isolated in child process A
- User B's SSH pool isolated in child process B
- User A cannot send commands to User B's SSH session

---

## TC-AUTH-003-05: Database isolation — per-user SQLite

**Priority:** P0

### Steps
1. User A tạo automation
2. Kiểm tra automation chỉ tồn tại trong User A's `orca.db`
3. Kiểm tra User B's `orca.db` không có automation này

### Assertions
```
rpcCall(sessionA, 'automation.create', { name: 'test-auto' })
dbA = openSQLite(`~/.orca/users/${userA.id}/orca.db`)
dbB = openSQLite(`~/.orca/users/${userB.id}/orca.db`)
assert dbA.query('SELECT * FROM automations WHERE name = ?', ['test-auto']).length === 1
assert dbB.query('SELECT * FROM automations WHERE name = ?', ['test-auto']).length === 0
```

---

## TC-AUTH-003-06: Environment isolation — USER_ID và DATA_PATH

**Priority:** P0

### Steps
1. Fork child process cho User A
2. Kiểm tra env vars trong child process

### Expected Results
- `USER_ID` = userA.id
- `DATA_PATH` = `~/.orca/users/{userA.id}/`
- Không có env vars của User B

### Assertions
```
childEnv = SessionManager.getProcess(userA.id).env
assert childEnv.USER_ID === userA.id
assert childEnv.DATA_PATH === `~/.orca/users/${userA.id}/`
```

---

*TC-AUTH-003 — Orca v5.0 — 2026-08-01*
