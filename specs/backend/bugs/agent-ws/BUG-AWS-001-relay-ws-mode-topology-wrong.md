# BUG-AWS-001: BL-AWS-01 relay-websocket Mode — Orca chủ động kết nối đến Dev Server KHÔNG đúng topology

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AWS-001,002,003  
**Implementation:** WsSessionRouter rewrite, SHA-256 token hash, permanent token  

## Mức độ: 🔴 CRITICAL

## Tóm tắt

HLD BL-AWS-01 mô tả:
```
[AgentConnectionManager.connect(agentEndpoint)]
    agentEndpoint: ws://dev-server-01:6799/orca-relay
    Orca → chủ động kết nối đến Dev Server
```

Thực tế từ `src/main/dev-server/agent-ws-server.ts`, `dev-server-relay-bridge.ts`:
```typescript
// Agent WebSocket Server: lắng nghe kết nối TỪ Dev Server
// Dev Server là client, chủ động kết nối vào ws://orca:6768/agent
```

**BL-AWS-01 (relay-websocket mode = Orca connect đến Dev Server) KHÔNG ĐƯỢC IMPLEMENT.**

Topology thực tế trong code (và đúng per component-mapping.md) là:
- Dev Server = WS **client** → chủ động kết nối vào `ws://orca:6768/agent`
- Orca = WS **server** → lắng nghe, nhận connection từ Dev Server

Không có code nào tại Orca Server chủ động `connect()` ra `ws://dev-server:6799`.

## Ảnh hưởng

1. BL-AWS-01 trong logic doc mô tả ngược chiều kết nối → sai
2. Developer đọc doc có thể hiểu nhầm và implement sai
3. Không có "relay-websocket mode" theo nghĩa HLD — chỉ có "direct-websocket mode" (Dev Server kết nối vào)

## Root Cause

HLD viết 2 mode:
- `relay-websocket`: Orca → Dev Server (HLD C3.8)
- `direct-websocket`: Dev Server → Orca (HLD C4.5)

Nhưng code chỉ implement 1 mode: **Dev Server → Orca**.

SSH relay mode thực chất là: SSH tunnel → nhưng chiều WS vẫn là Dev Server kết nối vào Orca (qua tunnel local port).

## Fix đề xuất

Cập nhật HLD/docs để phản ánh đúng: chỉ có 1 topology — Dev Server luôn là WS client.  
Rename BL-AWS-01 thành "SSH Tunnel mode" (Dev Server kết nối vào Orca qua SSH tunnel, không phải ngược lại).

## Files liên quan

- `src/main/dev-server/agent-ws-server.ts`: AgentWebSocketServer (listen mode)
- `src/main/dev-server/dev-server-relay-bridge.ts`: DevServerRelayBridge (handle incoming)
- `docs/flows/logic/agent-ws.md`: BL-AWS-01 mô tả sai chiều
