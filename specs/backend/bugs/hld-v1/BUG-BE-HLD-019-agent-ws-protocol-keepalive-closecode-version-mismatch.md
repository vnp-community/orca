# BUG-BE-HLD-019 — Agent WebSocket Protocol: keepalive timing sai số liệu, close code 4001-4003 không tồn tại, version-mismatch check là hằng số chết

**Mức độ:** 🟡 MEDIUM (Documentation drift + missing feature)
**Status:** 🔴 Open
**Module:** `backend/src/main/ssh/relay-protocol.ts`, `backend/src/shared/agent-wire-protocol.ts`, `backend/src/main/dev-server/ws-handshake.ts`, `agent-ws-server.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.8/F29)

---

## Mô tả

`docs/features/F29-agent-websocket-protocol.md` mô tả 3 chi tiết cụ thể không khớp code:

1. **Keepalive timing sai hoàn toàn:** doc ghi gửi mỗi 30s, ngắt kết nối sau 3 lần miss (90s). Thực tế `KEEPALIVE_SEND_MS = 5_000` (5s), `TIMEOUT_MS = 20_000` (20s) — (`relay-protocol.ts:24-25`, lặp lại ở `agent-wire-protocol.ts:21-22`). Không có logic "3 lần miss" — ngắt ngay khi quá 20s không nhận data (`ssh-channel-multiplexer.ts:525,534`). Có thêm 1 `ws.ping()` interval 30s riêng (`agent-ws-server.ts:124-126`) chỉ để giữ kết nối qua reverse-proxy — dễ nhầm với cơ chế keepalive 0x09 chính, đây có thể là nguồn gốc gây nhầm lẫn khi viết doc.

2. **Close code 4001/4002/4003 không tồn tại:** doc ghi `4001=Unauthorized`, `4002=HandshakeTimeout`, `4003=VersionMismatch`. Code thực tế đóng bằng mã WS chuẩn **1008** (Policy Violation, token sai — `ws-handshake.ts:200`, `agent-ws-server.ts:156`) hoặc mã mặc định **1005** (handshake timeout — `ws-handshake.ts:141,158,166`). Grep `4001`/`4002`/`4003` trong `backend/src/main/dev-server/`: 0 kết quả.

3. **`AGENT_MIN_VERSION` là hằng số chết:** khai báo tại `agent-wire-protocol.ts:31` nhưng **không được tham chiếu ở bất kỳ đâu khác** trong `backend/src` — nghĩa là version-mismatch detection (chính là lý do tồn tại của close code 4003) chưa từng được cài đặt.

## Hậu quả

- Client/tooling implement theo đúng doc (chờ 90s trước khi coi là mất kết nối, parse close code 4001-4003 để phân loại lỗi) sẽ hoạt động sai với server thật.
- Không có bảo vệ nào chống Agent version cũ/không tương thích kết nối vào Backend — có thể gây lỗi runtime khó chẩn đoán nếu giao thức đổi breaking.

## Bằng chứng

- `backend/src/main/ssh/relay-protocol.ts:24-25` — `KEEPALIVE_SEND_MS=5_000`, `TIMEOUT_MS=20_000`.
- `backend/src/shared/agent-wire-protocol.ts:21-22,31` — hằng số trùng lặp + `AGENT_MIN_VERSION` không dùng.
- `backend/src/main/dev-server/ws-handshake.ts:141,158,166,200` — mã đóng thật 1005/1008.
- `backend/src/main/dev-server/agent-ws-server.ts:124-126,156` — `ws.ping()` 30s riêng biệt + close 1008.

## Đề xuất fix

1. **Nếu số liệu 5s/20s là quyết định có chủ đích** (nhiều khả năng, vì đã tune kỹ và dùng nhất quán ở 2 nơi): cập nhật lại `docs/features/F29-agent-websocket-protocol.md` cho khớp, xoá mô tả "3 lần miss" gây hiểu nhầm.
2. Cập nhật doc close code từ 4001-4003 sang mã WS chuẩn thật (1005/1008), hoặc nếu muốn có mã lỗi custom rõ nghĩa hơn, cân nhắc thêm implement thật (đổi từ 1008 sang custom code trong dải 4000-4999 theo RFC 6455).
3. Implement version-mismatch check thật: so sánh version Agent gửi trong handshake với `AGENT_MIN_VERSION`, đóng kết nối với lý do rõ ràng nếu không tương thích — biến hằng số chết thành logic thật.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.8 (F29)
- Doc gốc: `docs/features/F29-agent-websocket-protocol.md`
