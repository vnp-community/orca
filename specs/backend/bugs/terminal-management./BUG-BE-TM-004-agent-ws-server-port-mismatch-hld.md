# BUG-BE-TM-004: AgentWebSocketServer lắng nghe cổng 6769 nhưng HLD quy định 6768/agent

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-003  
**Note:** dev-server-relay-bridge.ts: port 6769 → 6768  

## Mức độ: HIGH

## Tóm tắt

Theo HLD (terminal-create-flow.md), Agent kết nối vào Backend qua `wss://backend:6768/agent`. Nhưng `agent-ws-server.ts` có comment chỉ rõ Agent connect vào cổng **6769**:

```
// Browser  → ws://:6768/        (existing OrcaRuntimeRpcServer — unchanged)
// Agent    → ws://:6769/agent   (NEW — this file handles /agent path on HTTP server)
```

Còn trong `dev-server-relay-bridge.ts` khi tính toán URL fallback:

```typescript
const port = process.env['ORCA_HTTP_PORT'] ?? '6769'
return `ws://${host}:${port}${AGENT_WS_PATH}`
```

Default port là 6769, không phải 6768.

## File liên quan

- [`src/main/dev-server/agent-ws-server.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/agent-ws-server.ts) — Lines 5-8
- [`src/main/dev-server/dev-server-relay-bridge.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/dev-server-relay-bridge.ts) — Lines 250-256

## Chi tiết sai khác

| Tài liệu HLD | Code thực tế |
|---|---|
| Agent kết nối `ws://backend:6768/agent` | Agent kết nối `ws://backend:6769/agent` (default) |
| Browser kết nối `wss://backend:6768/` | Browser kết nối `ws://backend:6768/` (OK) |
| Cùng một HTTP server, path khác nhau | Hai port khác nhau |

## Ảnh hưởng

1. **Nếu triển khai theo HLD** (6768 cho tất cả): Agent sẽ không connect được vì dùng 6769 mặc định → `terminal.create` fail với "Not connected".
2. **Nếu triển khai theo code** (6769 cho Agent): Cần mở thêm firewall port 6769, tài liệu sai → confusion cho ops team.
3. **ORCA_HTTP_PORT** env var phải được set đúng trong production deployment.

## Cách fix đề xuất

Quyết định canonical: chọn một trong hai:
- **Option A (theo HLD)**: Dùng cùng HTTP server trên port 6768, path `/agent` — sửa default trong `dev-server-relay-bridge.ts` từ `'6769'` thành `'6768'`.
- **Option B (giữ code)**: Cập nhật HLD để phản ánh 6769 là port riêng cho Agent.

## Liên quan đến luồng

- **Pre-condition**: Dev Server Agent connect inbound vào `wss://backend/agent`.
- **BL-TM-01**: Relay routing sẽ fail nếu Agent không connect được.
