# backend-go Solutions — MCP Protocol Server (v5)

**CRs:** [docs/crs/v5/mcp-protocol-server](../../../../../../docs/crs/v5/mcp-protocol-server/README.md)
**Hợp đồng FE:** [CONTRACT-mcp-ui-api.md](../../CONTRACT-mcp-ui-api.md) · **Quy ước chung + T1..T8:** [crs/v5/README.md](../../README.md)
**TDD tham chiếu:** [`api-gateway.md`](../../../../tdd/services/api-gateway.md) §5 (stateless, fan-out), §6, §9 · [`notification-service.md`](../../../../tdd/services/notification-service.md) §3 · [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) · [`arch/05`](../../../../tdd/architecture/05-data-architecture.md)

## Re-verify trước khi thiết kế (CR vs mã thật, 2026-10-01)

| Khẳng định của CR | Kết quả khi đọc mã | Lệch? |
|---|---|---|
| CR-003: `agent-rpc-dispatch-misc.ts:29-68` có `tools/list`/`tools/call`, không có `initialize` | Đúng: `agent/src/relay/agent-rpc-dispatch-misc.ts` có `case 'tools/list'` (dòng 30) và `case 'tools/call'` (dòng 44), không có `initialize`. Ở `backend-go` không có gì về MCP | Không |
| CR-003: "dùng SDK Go chính thức `modelcontextprotocol/go-sdk`" | **Không có** trong bất kỳ `go.mod`; chưa thể kiểm API Streamable HTTP/EventStore của SDK trong môi trường này ⇒ **(chưa xác minh)**. Thiết kế đặt *seam* để SDK hoặc cài đặt tự viết đều cắm được; có bước spike bắt buộc (BE-003 §B) | Chưa xác minh |
| CR-004: "dùng lại cơ chế `SubscribeEphemeral` của notification-service" | **Lệch quan trọng.** `common/eventbus.Consumer.SubscribeEphemeral` là **consumer JetStream không tên trên một stream** (`stream.CreateOrUpdateConsumer`, `InactiveThreshold` 5 phút), không phải core-NATS pub/sub. Dùng nó cho `mcp.session.<id>` nghĩa là mỗi session/replica tạo một JetStream consumer + cần stream bao subject đó (ghi đĩa, trùng với stream `MCP`). Chữ trong T5 ("core-NATS ephemeral … dựa trên SubscribeEphemeral") mâu thuẫn nội bộ | **Lệch** ⇒ BE-004 §B quyết định lại; yêu cầu sửa văn bản T5 |
| CR-004: `wscompat` có `StreamChannelHandler` (`registry.go:78`) | Đúng; nhưng kênh push UI (`mcp.events.subscribe`) là `RegisterStream` (ack `nil`, sự kiện đẩy dạng `Streaming:true` với `Result = ev.Args[0]` — `handler.go:348`, `push_bridge.go:pushEventResult`) | Không (xác nhận hình dạng cho FE) |
| CR-004: nhiều replica api-gateway, SSE gắn một TCP | Đúng về nguyên tắc; limiter hiện **per-replica in-memory** (`usecase/rate_limit.go`), không Redis; `go.mod` không có Redis | Xác nhận: không có Redis để dựa vào |
| CR-004: `dispatchRPCTimeout = 60s` "lưới an toàn" | Đúng (`registry.go`), **nhưng** nó áp cho `Dispatch`, không áp cho `DispatchStreamChannel` (cố ý không timeout). Tool dài hạn (BE-009) phải có deadline riêng | Xác nhận, ghi cho BE-009 |
| CR-003: phiên bản spec tối thiểu `2025-06-18` (D5) | Không có file khoá phiên bản nào hiện hữu ⇒ tạo mới (BE-003 §C) | Không |

## Solutions

| Solution | CR | Service / Area | Effort | Status |
|---|---|---|---|---|
| [BE-MCP-SOL-003](./BE-MCP-SOL-003-streamable-http-and-lifecycle.md) | CR-MCP-003 (+ kênh `mcp.server.info`) | `api-gateway` (`adapter/mcpserver`, `wscompat/channels_mcp_server_info.go`) | Medium | ✅ Implemented (unit/integration tests) — see service README for gaps |
| [BE-MCP-SOL-004](./BE-MCP-SOL-004-sessions-sse-resumability.md) | CR-MCP-004 (+ `mcp.session.list/close`, `mcp.admin.session.list`) | `mcp-service` (migration `0002_sessions`, usecase session), `api-gateway`, `common/eventbus` | Large | ✅ Implemented (unit/integration tests) — see service README for gaps |

## Thứ tự thực thi & phụ thuộc

```
BE-MCP-SOL-002 ──▶ BE-MCP-SOL-003 (transport + lifecycle, SessionStore in-memory, POST JSON, GET/DELETE = 405)
                        └─▶ BE-MCP-SOL-004 (SessionStore thật qua mcp-service, SSE, resume, cross-replica, DELETE)
BE-MCP-SOL-001 ──▶ BE-MCP-SOL-004 (bảng sessions, RPC Session)
```

- BE-003 hoàn tất độc lập: `mcp.server.info` + `initialize` + `ping` + `tools/list` (rỗng/đúng catalog khi BE-007 xong) đủ cho client tham chiếu (Python SDK, MCP Inspector) ở **một replica**.
- BE-004 là điều kiện cho tool dài hạn (BE-009) và production nhiều replica; **không** chặn tool đồng bộ.
- Phụ thuộc cắm: xác thực thật cần BE-005/006 (`TokenVerifier`); đến lúc đó dùng token dev (`denyAllVerifier` thay bằng verifier giả trong test).

## Quyết định đã chốt & điều còn mở

1. **Đã chốt (D4, 2026-10-01):** Go SDK chính thức cho server, Python SDK chính thức cho client kiểm thử (BE-015). Spike 0.5 ngày (BE-003 §B) chỉ xác minh hook Streamable HTTP/session-store và ghi khoảng trống vào ADR.
2. Sửa T5 thành: *core-NATS ephemeral cho tín hiệu điều khiển + stream JetStream giới hạn cho vòng đệm resume* (BE-004 §B).
3. Chủ sở hữu `common/eventbus` duyệt API mới (core-NATS publisher/subscriber, replay theo sequence) — tác động nhiều service (BE-004 §E).
