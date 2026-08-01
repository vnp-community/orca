# BUG-DS-003 — `orcaUrl` Emit Với Literal `<orca-host>`

**ID:** BUG-DS-003  
**Mức độ:** 🟠 High  
**Module:** `dev-server-relay-bridge.ts` — `connectDirectWebSocket()`  
**Phát hiện:** 2026-07-26  
**Status:** ✅ FIXED — 2026-08-01 (Tasks: TASK-DS-001~011)

---

## Mô Tả

Khi user connect một dev server theo luồng thủ công (không dùng daemon script), Orca emit sự kiện `agentTokenGenerated` chứa URL mà user cần dùng để cấu hình agent. URL này hardcode literal `<orca-host>` thay vì hostname thực tế.

---

## Root Cause

**`dev-server-relay-bridge.ts` L186-192**:

```typescript
this.emit('agentTokenGenerated', {
  devServerId: this.config.id,
  agentToken,
  orcaUrl: `ws://<orca-host>:6768${AGENT_WS_PATH}`,  // ← LITERAL STRING!
})
```

Không có logic resolve hostname của Orca server. String `<orca-host>` là placeholder chưa được thay thế bằng giá trị thực.

---

## Tái Hiện

1. Trong Orca UI: Settings → Dev Servers → Add Dev Server
2. Connection Type: **direct-websocket**
3. Click "Connect" (không dùng daemon script)
4. UI hiển thị hướng dẫn setup agent

**Kết quả**: User nhận URL `ws://<orca-host>:6768/agent` thay vì URL thực.

---

## Hậu Quả

- User setup agent thủ công sẽ fail vì URL sai
- Agent cố connect đến `ws://<orca-host>:6768/agent` → DNS lookup fail
- Đặc biệt ảnh hưởng khi user setup agent lần đầu không dùng scripts

---

## Fix

```typescript
// dev-server-relay-bridge.ts
import { getPlatform } from '../../platform/context'

// Trong connectDirectWebSocket():
const orcaHost = process.env.ORCA_ADVERTISED_HOST
  ?? process.env.SERVER_HOST
  ?? 'localhost'
const orcaPort = process.env.ORCA_HTTP_PORT ?? '6769'

this.emit('agentTokenGenerated', {
  devServerId: this.config.id,
  agentToken,
  orcaUrl: `ws://${orcaHost}:${orcaPort}${AGENT_WS_PATH}`,
})
```

Hoặc thêm `orcaAdvertisedUrl` vào server config và đọc từ đó.

---

## Files Liên Quan

| File | Dòng | Vai trò |
|------|------|---------|
| `src/main/dev-server/dev-server-relay-bridge.ts` | L190 | Bug location |
| `src/shared/agent-wire-protocol.ts` | AGENT_WS_PATH | Path constant |
