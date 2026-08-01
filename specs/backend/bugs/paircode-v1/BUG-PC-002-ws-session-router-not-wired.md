# BUG-PC-002 — `WsSessionRouter` Được Tạo Nhưng Không Được Wired

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-PC-001~005  
**Note:** paircode-v1 domain fixed 2026-07-27  

**ID:** BUG-PC-002  
**Mức độ:** 🔴 Critical  
**Module:** `src/server/index.ts`  
**Phát hiện:** 2026-07-27  
**Status:** 🔴 Open

---

## Mô Tả

Trong server mode với `ORCA_MULTI_USER=1`, `WsSessionRouter` được khởi tạo đúng cách nhưng bị discard ngay lập tức bằng `void wsRouter`. Kết quả: WebSocket connections từ browser không bao giờ được route đến per-user process dựa trên session cookie. Multi-user isolation hoàn toàn không hoạt động cho WS channel.

---

## Root Cause

### Code location — `src/server/index.ts` L122-129

```typescript
const sessionManager = new SessionManager({ baseDataPath, userProcessEntry })
const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
sessionManagerShutdown = () => sessionManager.shutdown()

console.log('[Orca Server] ✅ Multi-user mode: SessionManager initialized')
console.log(`[Orca Server]    User process entry: ${userProcessEntry}`)
// WsSessionRouter is available for WebSocket server integration (TASK-021)
void wsRouter  // ← BUG: suppress unused-var warning — CHƯA ATTACH VÀO WS SERVER!
```

`wsRouter` được tạo ra nhưng **không được attach** vào `OrcaRuntimeRpcServer`. Không có code nào gọi `wsRouter.handleConnection(ws, req)` khi có WS connection đến.

### Điều Này Nghĩa Là Gì

**Trạng thái hiện tại (BROKEN):**
```
Browser login → cookie session → WS connect port 6768
  → OrcaRuntimeRpcServer xử lý trực tiếp (root process)
  → ctx.devServerManager = devServerManager ✅ (đúng)
  → NHƯNG: E2EE handshake required (4001 Invalid e2ee_hello ❌)
```

**Trạng thái mong muốn (NOT YET IMPLEMENTED):**
```
Browser login → cookie session → WS connect port 6768
  → WsSessionRouter.handleConnection(ws, req)
  → resolveUserFromRequest(req) → userId từ session cookie
  → getOrCreateUserSocket(userId) → proxy đến user-process
  → User process xử lý RPC (isolated per user)
```

### Verify bằng Test Thực Tế

```bash
# Test kết nối WS tới port 6768 (bên trong container)
docker exec orca-server node /tmp/test-rpc.js
# → CLOSED: 4001 Invalid e2ee_hello
# → Server đóng kết nối ngay vì yêu cầu E2EE pair code handshake
# → WsSessionRouter không intercept gì cả
```

---

## Tái Hiện

1. Deploy Orca server với `ORCA_MULTI_USER=1`
2. Login: `POST /auth/local { email, password }` → `Set-Cookie: session=...`
3. Kết nối WS tới `ws://172.20.2.39:6768` với cookie trong header
4. Gửi bất kỳ RPC request nào

**Kết quả**: Connection bị close với code `4001 Invalid e2ee_hello`.  
**Mong đợi**: WsSessionRouter validate cookie → proxy đến user process → RPC response.

---

## Hậu Quả

| Tính năng | Bị ảnh hưởng |
|-----------|-------------|
| Multi-user isolation | ❌ Không hoạt động — mọi user dùng chung root process |
| Login-based WS session | ❌ Không hoạt động — luôn yêu cầu Pair Code |
| Per-user data isolation | ❌ Không có — tất cả data chung |
| Session security | ❌ Session cookie không được validate cho WS |

---

## Fix Đề Xuất

### Bước 1 — Attach WsSessionRouter vào WebSocket server

Trong `src/server/index.ts`, sau khi tạo `wsRouter`, attach nó vào HTTP server upgrade event:

```typescript
// server/index.ts — Phase 2 multi-user wiring
const sessionManager = new SessionManager({ baseDataPath, userProcessEntry })
const wsRouter       = new WsSessionRouter({ sessionManager, authManager })
sessionManagerShutdown = () => sessionManager.shutdown()

// ── THÊM: Wire WsSessionRouter vào HTTP server ──────────────────────────────
// Intercept WS upgrades. Connections with valid session cookie → user process.
// Connections without session → 4401 (auth required).
httpServer.on('upgrade', async (req, socket, head) => {
  const url = new URL(req.url ?? '/', `http://${req.headers.host ?? 'localhost'}`)
  
  // AgentWebSocketServer đã handle /agent path — chỉ route các path còn lại
  if (url.pathname === AGENT_WS_PATH) return
  
  // Upgrade socket thành WS và route qua session router
  const ws = await upgradeToWebSocket(req, socket, head)
  await wsRouter.handleConnection(ws, req)
})
```

### Bước 2 — Tích hợp với RpcServer (optional)

Nếu muốn giữ backward compat với Pair Code path, `OrcaRuntimeRpcServer` cần nhận callback để phân biệt:
- Connection có valid session cookie → route qua `WsSessionRouter`
- Connection không có session → dùng legacy E2EE pair code path

---

## Files Liên Quan

| File | Dòng | Vai trò |
|------|------|---------|
| `src/server/index.ts` | L114-129 | Bug location — `void wsRouter` |
| `src/main/session/ws-session-router.ts` | All | Router implementation (đúng, chưa dùng) |
| `src/main/session/session-manager.ts` | All | Per-user process manager |
| `src/main/auth/auth-manager.ts` | All | Session cookie validation |

---

## Quan Hệ với Bugs Khác

- **Prerequisite cho BUG-PC-001 fix**: Phải fix PC-002 trước thì Phương án A của PC-001 mới hoạt động
- **Độc lập với BUG-PC-003**: PC-003 có thể fix độc lập
