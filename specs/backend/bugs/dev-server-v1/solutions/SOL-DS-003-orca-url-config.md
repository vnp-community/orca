# SOL-DS-003 — Fix `orcaUrl` Từ Environment Config

**Fixes:** [BUG-DS-003](../BUG-DS-003-orca-url-literal.md)  
**TDD Ref:** TDD-08 §2 (DevServer add flow), TDD-13 §3 (DevServerRelay.connect)  
**Files:** `src/main/dev-server/dev-server-relay-bridge.ts`, `deploy/dev/.env`  
**Effort:** ~15 phút  
**Status:** ✅ DONE — 2026-07-27 (TASK-DS-006)  
**Implemented in:** `src/main/dev-server/dev-server-relay-bridge.ts` dng 186-203 (`connectDirectWebSocket()`)

---

## Phân Tích

Theo TDD-11 §2 (web-server-mode.md), server đọc cấu hình từ environment variables. `ORCA_HTTP_PORT` đã được sử dụng trong `src/server/index.ts`. Cần thêm `ORCA_ADVERTISED_HOST` để resolve public hostname/IP của Orca server cho agent setup instructions.

Thực tế: Orca server chạy tại `172.20.2.39:6769` (nội bộ) hoặc qua Nginx `wss://b15.openledger.vn/agent` (public). Agent dùng public URL. URL này đã có trong `.env` (`AGENT_ORCA_URL`).

---

## Thay Đổi Cần Thực Hiện

### Option A — Đọc từ env var (Minimal change — khuyến nghị)

**File: `src/main/dev-server/dev-server-relay-bridge.ts`**

Tìm (L186-192):
```typescript
this.emit('agentTokenGenerated', {
  devServerId: this.config.id,
  agentToken,
  orcaUrl: `ws://<orca-host>:6768${AGENT_WS_PATH}`,
})
```

Thay bằng:
```typescript
// Resolve Orca WS URL cho agent setup instructions.
// Priority: ORCA_AGENT_WS_URL (full override) → ORCA_ADVERTISED_HOST:ORCA_HTTP_PORT → localhost
const orcaWsUrl = process.env['ORCA_AGENT_WS_URL']
  ?? (() => {
    const host = process.env['ORCA_ADVERTISED_HOST'] ?? 'localhost'
    const port = process.env['ORCA_HTTP_PORT'] ?? '6769'
    return `ws://${host}:${port}${AGENT_WS_PATH}`
  })()

this.emit('agentTokenGenerated', {
  devServerId: this.config.id,
  agentToken,
  orcaUrl: orcaWsUrl,
})
```

### Deploy config — `deploy/dev/.env`

Thêm vào `.env`:
```bash
# URL public của Orca server (agent dùng để kết nối)
# direct-websocket mode: agent connect đến URL này
ORCA_AGENT_WS_URL=wss://b15.openledger.vn/agent
```

---

## Không Cần Thay Đổi

- `src/server/index.ts` — đã xử lý `ORCA_HTTP_PORT`
- Agent scripts — đã dùng `AGENT_ORCA_URL` từ `.env`
- `connect-agent.sh` — đã có `AGENT_ORCA_URL` variable

---

## Verification

```bash
# Set env và restart server:
ORCA_AGENT_WS_URL=wss://b15.openledger.vn/agent

# Trong Orca UI: Add Dev Server → direct-websocket → Connect
# UI phải hiển thị: "Run on dev server:"
#   ORCA_URL=wss://b15.openledger.vn/agent \
#   AGENT_TOKEN=agt-dev-local-xxx \
#   node agent.js
# KHÔNG phải: ORCA_URL=ws://<orca-host>:6768/agent
```

---

## Files Liên Quan

| File | Thay đổi |
|------|---------|
| `src/main/dev-server/dev-server-relay-bridge.ts` | L190 — resolve orcaUrl từ env |
| `deploy/dev/.env` | Thêm `ORCA_AGENT_WS_URL` |
| `deploy/dev/.env.example` | Document new var |
