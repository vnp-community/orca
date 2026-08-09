# TASK-HLD-031: Sửa `docs/features/F29-agent-websocket-protocol.md` — keepalive timing + close code sai lệch với code thật

**Priority:** 🟢 LOW — chỉ sửa tài liệu, không sửa code/runtime behavior
**Effort:** ~20 phút
**Status:** ✅ DONE — 2026-08-09 (xác nhận giá trị thật trước khi sửa doc: `KEEPALIVE_SEND_MS=5_000`/`TIMEOUT_MS=20_000` (relay-protocol.ts) và `AGENT_KEEPALIVE_INTERVAL_MS=5_000`/`AGENT_TIMEOUT_MS=20_000` (agent-wire-protocol.ts) khớp task; `ws.close(1008, ...)` xác nhận tại `ws-handshake.ts:200` (auth fail) và `agent-ws-server.ts:156` (slot expired); `ws.ping()` mỗi 30s xác nhận tại `agent-ws-server.ts:124-126`. Áp đủ 4 thay đổi: sửa close-code sai (dòng ~91, ~95), sửa keepalive timing sai (dòng ~122, ~156-157), thêm section "Version Compatibility Check", thêm section "Ghi chú: hai cơ chế keepalive độc lập" cuối file. Verification bash của task (0 kết quả cho `4001|4002|4003` và `90s (3 missed)`) đều pass. Lưu ý đã ghi trong section mới (đúng như task cảnh báo): phần "Version Compatibility Check" mô tả state SAU KHI TASK-HLD-032 hoàn tất — trong phiên làm việc này TASK-HLD-032 được thực hiện ngay sau, nên doc và code sẽ nhất quán khi cả 2 task kết thúc.)
**Bug refs:** BUG-BE-HLD-019
**Solution ref:** [SOLUTION-agent-ws-protocol-exact.md](../solutions/SOLUTION-agent-ws-protocol-exact.md) — Phần A (A.1, A.2, A.3)
**Depends on:** None

---

## Mục tiêu

`docs/features/F29-agent-websocket-protocol.md` mô tả sai 2 điểm so với code thật đang chạy ổn định:

1. **Close code:** doc ghi mã tùy biến `4001`/`4002`/`4003` (Unauthorized/HandshakeTimeout/VersionMismatch) — code thật dùng mã WS chuẩn `1008` (Policy Violation, cho auth fail và slot expired) và `1005` (No Status Received, mặc định của `ws.close()` không tham số, cho timeout).
2. **Keepalive timing:** doc ghi "gửi mỗi 30s, timeout 90s (3 missed)" — code thật là `KEEPALIVE_SEND_MS = 5_000` / `TIMEOUT_MS = 20_000` (nhất quán ở cả `relay-protocol.ts` và `agent-wire-protocol.ts`), không có logic "3 lần miss" — timeout kích hoạt ngay khi quá 20s không nhận bất kỳ frame nào (Regular hoặc KeepAlive).

Đây là bug tài liệu thuần túy — **không sửa số liệu/logic code**, chỉ sửa doc cho khớp thực tế. Phần version-mismatch check (code thật, không phải doc) thuộc TASK-HLD-032 — độc lập với task này.

## File cần sửa/tạo

```
docs/features/F29-agent-websocket-protocol.md   (sửa dòng 91, 95, 122, 156-157; thêm 2 section mới)
```

## Thay đổi cụ thể

### 1. Close code sai (dòng 91, 95)

Text sai hiện tại:
```markdown
// Validate khi kết nối
SHA-256(received) === storedHash ? OK : close(4001)
```
```markdown
- UI: `AgentTokenPanel` — hiển thị token 1 lần, copy-to-clipboard, regenerate
- Close codes: 4001=Unauthorized, 4002=HandshakeTimeout, 4003=VersionMismatch
```

Fix — thay bằng mã đóng WS chuẩn thật đang dùng (`ws-handshake.ts:200`: `1008` cho auth fail; `agent-ws-server.ts:156`: `1008` cho slot expired; timeout dùng mã mặc định `1005` vì `ws.close()` không truyền code — `ws-handshake.ts:141,158,166`):

```markdown
// Validate khi kết nối
SHA-256(received) === storedHash ? OK : close(1008, 'Authentication failed...')
```
```markdown
- UI: `AgentTokenPanel` — hiển thị token 1 lần, copy-to-clipboard, regenerate
- Close codes: `1008` (Policy Violation) = token sai/slot hết hạn, `1005` (No Status
  Received, mặc định của `ws.close()` không tham số) = handshake timeout. Không có
  mã tùy biến 4000-4999 — xem mục "Version Compatibility Check" nếu cần version-mismatch
  dùng mã riêng.
```

### 2. Keepalive timing sai (dòng 122, 156-157)

Text sai hiện tại:
```markdown
- [x] KeepAlive 0x09 gửi mỗi 30s, timeout 90s
```
```markdown
| KeepAlive interval | 30s |
| KeepAlive timeout | 90s (3 missed) |
```

Fix — khớp `KEEPALIVE_SEND_MS = 5_000` / `TIMEOUT_MS = 20_000` (`relay-protocol.ts:24-25`, lặp lại y hệt ở `AGENT_KEEPALIVE_INTERVAL_MS` / `AGENT_TIMEOUT_MS` trong `agent-wire-protocol.ts:21-22`) — không có logic "3 lần miss", timeout kích hoạt ngay khi quá 20s không nhận **bất kỳ** frame nào (Regular hoặc KeepAlive), không riêng KeepAlive:

```markdown
- [x] KeepAlive 0x09 gửi mỗi 5s nếu không có frame Regular nào được gửi; ngắt kết
  nối nếu không nhận được frame nào (Regular hoặc KeepAlive) trong 20s liên tục
```
```markdown
| KeepAlive interval | 5s |
| KeepAlive timeout | 20s (không nhận bất kỳ frame nào — không phải "3 lần miss") |
```

### 3. Thêm ghi chú về 2 cơ chế keepalive độc lập (chèn cuối file, sau bảng Metrics dòng 159)

```markdown
---

## Ghi chú: hai cơ chế keepalive độc lập

Có **hai** lớp keepalive riêng biệt, không nên gộp chung:

1. **Application-level KeepAlive (0x09)** — khung 13-byte header với `TYPE=0x09`,
   gửi mỗi 5s nếu không có frame Regular nào vừa gửi; bên nhận coi mất kết nối nếu
   không nhận **bất kỳ** frame nào (Regular hoặc KeepAlive) trong 20s. Định nghĩa ở
   `src/main/ssh/relay-protocol.ts` (`KEEPALIVE_SEND_MS`, `TIMEOUT_MS`) và lặp lại
   giá trị ở `src/shared/agent-wire-protocol.ts` (`AGENT_KEEPALIVE_INTERVAL_MS`,
   `AGENT_TIMEOUT_MS`). Đây là cơ chế chính bảo vệ giao thức JSON-RPC framed.
2. **Transport-level WS ping** — `ws.ping()` gọi mỗi 30s trong
   `src/main/dev-server/agent-ws-server.ts:124-126`, chỉ nhằm giữ kết nối sống qua
   reverse proxy/load balancer (ALB, Cloudflare) hay đóng idle socket sau một
   khoảng lặng nhất định — không phải cơ chế phát hiện mất kết nối ở tầng ứng
   dụng và không có logic timeout tương ứng ở phía Orca.
```

### 4. Thêm section "Version Compatibility Check" (chèn sau dòng 96, trước "### Language-agnostic SDK examples")

```markdown
---

### Version Compatibility Check

`AGENT_MIN_VERSION` (`src/shared/agent-wire-protocol.ts`) là version Agent tối
thiểu Orca chấp nhận. Được kiểm tra trong `runOrcaReceiverHandshake()`
(`src/main/dev-server/ws-handshake.ts`) ngay sau khi validate token, trước khi
trả `handshake-ok`:

```typescript
if (semverLt(agentVersion, AGENT_MIN_VERSION)) {
  ws.close(1008, `Agent version ${agentVersion} is below minimum ${AGENT_MIN_VERSION}`)
}
```

- Áp dụng cho cả `direct-websocket` (Agent gửi `agentVersion` trong
  `agent.handshake` params) và `relay-websocket` (Orca đọc `agentVersion` từ
  handshake result của Agent).
- Không dùng close code tùy biến (4000-4999) cho version mismatch — dùng `1008`
  (Policy Violation), cùng mã đã dùng cho auth failure, kèm message phân biệt rõ
  lý do trong `reason` string (client phân loại theo message, không theo code).
```

> Lưu ý: implementation thật của check này (`isAgentVersionBelowMinimum()` + wiring trong
> `ws-handshake.ts`) là code, thuộc TASK-HLD-032 — section doc này mô tả hành vi *sau khi*
> TASK-HLD-032 hoàn tất. Nếu TASK-HLD-031 được merge trước TASK-HLD-032, section này mô tả
> hành vi dự kiến/target, không phải hành vi hiện tại — cân nhắc gộp 2 PR hoặc merge theo
> đúng thứ tự (032 trước hoặc cùng lúc 031) để doc không nói trước code.

## Verification

```bash
# 1. Không còn text 4001/4002/4003 hoặc "30s"/"90s" liên quan keepalive trong doc
grep -n "4001\|4002\|4003" docs/features/F29-agent-websocket-protocol.md
# Expected: 0 kết quả

grep -n "30s\|90s (3 missed)" docs/features/F29-agent-websocket-protocol.md
# Expected: 0 kết quả liên quan keepalive (dòng ws.ping() 30s ở section "Ghi chú" là
# nội dung MỚI mô tả đúng transport-level ping, không phải lỗi)

# 2. Giá trị đúng đã xuất hiện
grep -n "5s\|20s\|1008\|1005" docs/features/F29-agent-websocket-protocol.md
# Expected: có mặt ở các vị trí đã sửa

# 3. Đối chiếu lại với code thật để chắc chắn doc không lệch thêm lần nữa
grep -n "KEEPALIVE_SEND_MS\|TIMEOUT_MS" backend/src/main/ssh/relay-protocol.ts
grep -n "AGENT_KEEPALIVE_INTERVAL_MS\|AGENT_TIMEOUT_MS\|AGENT_MIN_VERSION" backend/src/shared/agent-wire-protocol.ts
# Expected: giá trị khớp với những gì vừa ghi vào doc (5_000 / 20_000)

# 4. Review thủ công: đọc lại toàn bộ file đã sửa để đảm bảo markdown không bị lỗi cú pháp
# (heading level, code fence đóng đúng) sau khi chèn 2 section mới.
```
