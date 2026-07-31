# TASK-DS-006 — Fix `orcaUrl` Emit Từ Env Var `ORCA_AGENT_WS_URL`

**Solution:** [SOL-DS-003](../solutions/SOL-DS-003-orca-url-config.md)  
**Bug:** [BUG-DS-003](../BUG-DS-003-orca-url-literal.md)  
**Files:** `src/main/dev-server/dev-server-relay-bridge.ts`, `deploy/dev/.env`  
**Phụ thuộc:** Không  
**Estimated:** 15 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thay thế literal `ws://<orca-host>:6768/agent` trong `connectDirectWebSocket()` bằng URL được đọc từ environment variable `ORCA_AGENT_WS_URL`. Khi không set, fallback về `ws://<ORCA_ADVERTISED_HOST>:<ORCA_HTTP_PORT>/agent`.

---

## Context

Đọc trước:
- `src/main/dev-server/dev-server-relay-bridge.ts` dòng 185-192 — emit hiện tại
- `src/shared/agent-wire-protocol.ts` — `AGENT_WS_PATH` constant
- `deploy/dev/.env` — các env vars hiện có

---

## Thay Đổi Cần Thực Hiện

### Thay đổi 1: `src/main/dev-server/dev-server-relay-bridge.ts`

**TÌM** (dòng ~186-192):
```typescript
      // Notify UI so user can configure and start the agent
      this.emit('agentTokenGenerated', {
        devServerId: this.config.id,
        agentToken,
        orcaUrl: `ws://<orca-host>:6768${AGENT_WS_PATH}`,
      })
```

**THAY BẰNG:**
```typescript
      // Resolve Orca WS URL cho agent setup instructions.
      // Priority:
      //   1. ORCA_AGENT_WS_URL — full override (e.g. wss://b15.openledger.vn/agent)
      //   2. ws://{ORCA_ADVERTISED_HOST}:{ORCA_HTTP_PORT}/agent
      //   3. ws://localhost:{ORCA_HTTP_PORT}/agent (dev fallback)
      const orcaWsUrl =
        process.env['ORCA_AGENT_WS_URL'] ??
        (() => {
          const host = process.env['ORCA_ADVERTISED_HOST'] ?? 'localhost'
          const port = process.env['ORCA_HTTP_PORT'] ?? '6769'
          return `ws://${host}:${port}${AGENT_WS_PATH}`
        })()

      // Notify UI so user can configure and start the agent
      this.emit('agentTokenGenerated', {
        devServerId: this.config.id,
        agentToken,
        orcaUrl: orcaWsUrl,
      })
```

### Thay đổi 2: `deploy/dev/.env`

Thêm vào cuối file:

```bash
# URL public của Orca WS server để agent kết nối (direct-websocket mode)
# Dùng khi UI hiển thị setup instructions cho user cài agent thủ công
ORCA_AGENT_WS_URL=wss://b15.openledger.vn/agent
```

---

## Verify

```bash
# Rebuild và restart server:
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npm run build:server  # hoặc lệnh build tương ứng

# Trong Orca UI: Add Dev Server → direct-websocket → Connect
# Quan sát URL hiển thị trong "Setup Instructions":
# Expected: wss://b15.openledger.vn/agent (KHÔNG phải ws://<orca-host>:6768/agent)

# Hoặc check server log:
grep "agentTokenGenerated\|orcaUrl" logs/server.log
```

---

## Definition of Done

- [x] String literal `ws://<orca-host>:6768` không còn trong `dev-server-relay-bridge.ts`
- [x] `orcaUrl` đọc từ `ORCA_AGENT_WS_URL` env var (priority 1)
- [x] Fallback về `ORCA_ADVERTISED_HOST:ORCA_HTTP_PORT` (priority 2)
- [x] Fallback cuối về `localhost:6769` nếu không set gì
- [x] TypeScript compile OK (no errors)
