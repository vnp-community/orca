# F24 — Per-User Process Sandbox

| Trường | Giá trị |
|--------|---------|
| **ID** | F24 |
| **Tên** | Per-User Process Sandbox |
| **Ưu tiên** | P0 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [login/CR-LOGIN-002](../crs/v1/login/CR-LOGIN-002-sandbox.md) |
| **TDD** | [TDD-04: Session Management](../specs/backend/tdd/04-rpc-server.md) |
| **Phiên bản** | v4.0+ |
| **ADR References** | ADR-003 |
| **HLD References** | C3.1 |

---

## Mô tả

Khi `ORCA_MULTI_USER=1`, mỗi user được cô lập trong **Node.js process riêng** (`fork()`). User A không thể đọc data của user B. Process isolation đảm bảo crash của 1 user không ảnh hưởng các user khác.

---

## Vấn đề cần giải quyết

Orca Server cũ là **single-process**: mọi user share cùng runtime, cùng SQLite database, cùng filesystem view. Rủi ro:
- Data leak giữa users
- Process crash ảnh hưởng tất cả
- Không audit được hoạt động từng user

---

## Tính năng chi tiết

### Process Isolation

```
Supervisor process (main server-bootstrap)
  └── fork() per user → user-process-entry.ts
        ├── env: ORCA_USER_ID=<uuid>
        ├── env: ORCA_SOCKET_PATH=userData/users/<userId>/orca.sock
        ├── env: ORCA_USER_DATA_PATH=userData/users/<userId>/
        └── OrcaRuntimeRpcServer (Unix socket only — không expose ra ngoài)
```

### SessionManager
- `getOrSpawnUserProcess(userId)` — idempotent, reuse existing process
- Fork timeout: 30s (fail fast)
- Idle shutdown: 4h (configurable via `SESSION_IDLE_TIMEOUT_MS`)
- Max respawn: 3 attempts sau crash

### WsSessionRouter
- WS request → resolve `userId` từ `req.orcaSession`
- `SessionManager.getOrSpawn(userId)` → Unix socket path
- Proxy WS ↔ Unix socket (transparent)

### Data Isolation
```
~/.orca/users/
  ├── user-a-uuid/
  │   ├── orca.sock        ← Unix socket
  │   ├── orca.db          ← Per-user SQLite
  │   └── worktrees/       ← Per-user worktrees
  └── user-b-uuid/
      ├── orca.sock
      └── ...
```

---

## Tiêu chí chấp nhận

- [x] Mỗi user login tạo 1 isolated Node.js process với `fork()`
- [x] User process có `userDataPath` riêng biệt `/data/orca/users/{userId}/`
- [x] WS connection của user A không thể đọc data của user B
- [x] User process tự shutdown sau 4h idle (configurable)
- [x] Process crash tự respawn (max 3 attempts)
- [x] SSH connection store isolated per user
- [x] Supervisor process vẫn healthy khi user process crash
- [x] `GET /health/ready` reflect status của supervisor

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Session types | `src/main/session/session-types.ts` |
| Session manager | `src/main/session/session-manager.ts` |
| WS session router | `src/main/session/ws-session-router.ts` |
| User process entry | `src/main/session/user-process-entry.ts` |

**Tests:** 21 tests | **Env flag:** `ORCA_MULTI_USER=1`
