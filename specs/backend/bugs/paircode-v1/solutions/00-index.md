# Solutions Index — PairCode v1 Bugs

**Bug set:** [paircode-v1](../00-index.md)  
**TDD refs:** TDD-11 §Addendum v4.0 (Multi-User WS Routing), TDD-04 §3 (Auth)  
**Phiên bản:** 1.4.138  
**Ngày:** 2026-07-27  
**Trạng thái:** 🔴 Open — chưa implement

---

## Tổng Quan Giải Pháp

3 bugs trong `paircode-v1` có chung nguyên nhân gốc: **WsSessionRouter chưa được wire** và **browser RPC channel chỉ hỗ trợ Pair Code**. Giải pháp được thiết kế theo 2 solution files:

| Solution | Fixes | Scope | Effort |
|----------|-------|-------|--------|
| [SOL-PC-001](./SOL-PC-001-ws-session-router-wiring.md) | BUG-PC-001, BUG-PC-002 | Server-side: `server/index.ts` + `OrcaRuntimeRpcServer` | ~2h | ✅ DONE |
| [SOL-PC-002](./SOL-PC-002-browser-session-rpc.md) | BUG-PC-001, BUG-PC-003 | Client-side: `web-preload-api.ts` + `useDevServersSync.ts` | ~2h | ✅ DONE |

**Thứ tự thực hiện:** SOL-PC-001 trước (server side), sau đó SOL-PC-002 (client side).

---

## Kiến Trúc Sau Fix

```
                     ORCA SERVER (172.20.2.39)
                     ┌──────────────────────────────────────────┐
                     │                                          │
Agent (172.20.2.31)  │  Port 6769/agent  ✅ Giữ nguyên          │
 node agent.js ─────►│  AgentWebSocketServer                    │
                     │  DevServerManager                        │
                     │                                          │
Browser (login)      │  Port 6768 — WsSessionRouter ✅ NEW       │
 https://b15...  ───►│  ├── cookie valid → user process (Unix)  │
     cookie ─────────►│  ├── no cookie → 4401 auth required     │
                     │  └── pair code → legacy E2EE path (kept) │
                     │                                          │
                     │  Port 6769/auth  ✅ Giữ nguyên           │
                     │  POST /auth/local → Set-Cookie           │
                     └──────────────────────────────────────────┘
```

---

## Phương Pháp Tiếp Cận

Theo TDD-11 §Addendum v4.0:
> `WebSocket request → WsSessionRouter`
> `├── ORCA_MULTI_USER=0 → delegate to rpcServer (backward compat)`
> `└── ORCA_MULTI_USER=1 → SessionManager.getOrSpawn(userId)`

Spec đã định nghĩa rõ ràng flow này. Việc cần làm là **implement đúng theo spec** — attach `WsSessionRouter` vào HTTP server upgrade event, và cập nhật browser-side để dùng session cookie thay vì pair code khi đang ở web mode.

---

## Files Sẽ Bị Sửa

| File | Solution | Thay đổi |
|------|----------|---------|
| `src/server/index.ts` | SOL-PC-001 | Wire `WsSessionRouter` vào HTTP server upgrade event |
| `src/main/runtime/runtime-rpc.ts` | SOL-PC-001 | Expose WS server upgrade handler để inject WsSessionRouter |
| `src/renderer/src/web/web-preload-api.ts` | SOL-PC-002 | Fallback RPC path dùng session cookie khi không có pair code |
| `src/renderer/src/hooks/useDevServersSync.ts` | SOL-PC-002 | Error handling cho `devServer.list()` |

---

## Compatibility Notes

- **Pair Code path giữ nguyên**: User dùng Pair Code vẫn hoạt động như cũ (backward compat)
- **ORCA_MULTI_USER=0**: Không ảnh hưởng — WsSessionRouter chỉ active khi `ORCA_MULTI_USER=1`
- **Agent connection**: Không thay đổi — agent vẫn connect qua `/agent` path riêng
