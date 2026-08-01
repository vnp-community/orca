# BUG-BE-TM-003: SessionManager không inject `ORCA_RPC_AUTH_TOKEN` vào child process env — chỉ nhận qua IPC

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-005  
**Note:** ws-session-router.ts: authToken validation guard  

## Mức độ: MEDIUM

## Tóm tắt

Tài liệu HLD (terminal-create-flow.md §Bước 2) mô tả RBAC check bằng `ORCA_RPC_AUTH_TOKEN` được inject vào child process. Nhưng thực tế trong `session-manager.ts`, token này **không** được truyền qua env vars. Thay vào đó, token được sinh ra bởi `OrcaRuntimeRpcServer` bên trong user-process-entry, sau đó gửi ngược về supervisor qua IPC message `{type: 'ready', rpcAuthToken}`.

Vấn đề: `WsSessionRouter` tiêm auth token vào WebSocket messages bằng cách thay thế `authToken === 'cookie-auth'` — đây là một protocol hack không có trong HLD.

## File liên quan

- [`src/main/session/session-manager.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/session-manager.ts) — Lines 87-98 (fork env)
- [`src/main/session/ws-session-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/session/ws-session-router.ts) — Lines 104-119 (token injection hack)

## Code thực tế vs HLD

**HLD mô tả:**
```
SessionManager.getOrSpawn(userId):
  → fork(user-process-entry.js, {
        ORCA_USER_ID: userId,
        ORCA_RPC_AUTH_TOKEN: <token>,   ← inject qua ENV
        ORCA_SOCKET_PATH: ~...
    })
```

**Code thực tế:**
```typescript
// session-manager.ts:87-98 — ORCA_RPC_AUTH_TOKEN không có trong env!
const child = fork(this.config.userProcessEntry, [], {
  env: {
    ...process.env,
    ...credentialEnv,
    ORCA_USER_DATA_PATH: userDataPath,
    ORCA_USER_ID: userId,
    ORCA_SOCKET_PATH: socketPath,
    NODE_OPTIONS: '--max-old-space-size=512'
    // ORCA_RPC_AUTH_TOKEN: NOT HERE
  }
})

// Token chỉ đến qua IPC 'ready' message:
// user-process-entry.ts:62
process.send({ type: 'ready', socketPath: sockPath, rpcAuthToken: rpcAuthToken })
```

**Token injection hack trong WsSessionRouter:**
```typescript
// ws-session-router.ts:110-114
if (parsed.authToken === 'cookie-auth') {
  parsed.authToken = authToken  // ← thay thế token tại proxy layer
  upstream.write(JSON.stringify(parsed) + '\n')
  return
}
```

## Ảnh hưởng

Cơ chế hiện tại (IPC + proxy injection) **hoạt động** nhưng:
1. Không match với thiết kế HLD
2. `'cookie-auth'` là magic string hardcode trong ws-session-router — dễ bị bỏ sót nếu client thay đổi
3. Nếu client không gửi `authToken: 'cookie-auth'`, token sẽ không được inject → request bị reject bởi user process với "Invalid auth token"
4. Không có unit test cho token injection path

## Liên quan đến luồng

- **BL-TM-01**: Bước 2 — RPC Dispatch, parseAndAuth.
