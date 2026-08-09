# Đánh giá `agent/` Code vs Thiết kế — Connection Modes & Wire Protocol

**Ngày:** 2026-08-08
**Phạm vi:** Cách `agent/` (Dev Server Agent) kết nối tới backend/gateway — wire protocol, `relay-websocket`, `direct-websocket`, token lifecycle, reconnect/backoff — đối chiếu với `docs/crs/v2/agent/*`, ADR-004/005/013 (v1) và ADR-017/018/019 (v2), F29, `docs/flows/logic/agent-ws.md`, `docs/flows/code/agent-ws/agent-connection-modes.md`, `docs/logic/agent-ws/BL-AWS-01..03`, `docs/hld/dev-server-architecture.md` §4–§5.
**Phương pháp:** Đọc toàn bộ tài liệu thiết kế liệt kê, đối chiếu trực tiếp với `agent/src/relay/*`, `agent/src/shared/agent-wire-protocol.ts`, `agent/src/main/ssh/relay-protocol.ts`, và (khi thiết kế mô tả phía nhận trên backend) `backend/src/main/dev-server/agent-ws-server.ts`, `backend/src/server/agent-token-routes.ts`.

---

## 1. Tổng kết mức độ khớp

| Mục thiết kế | Trạng thái | Vấn đề chính |
|---|---|---|
| CR-AG-001 Wire Protocol Framing (13-byte header) | ⚠️ Khớp code, nhưng **tài liệu tự mâu thuẫn nhau** | CR-AG-001/F29/HLD §5 đúng với code (`TYPE`: `0x01 Regular`/`0x09 KeepAlive`), nhưng ADR-004 và BL-AWS-01 mô tả 4 giá trị TYPE khác hẳn (`0=DATA,1=ACK,2=KEEPALIVE,3=CLOSE`) — không khớp code và không khớp cả các doc khác trong cùng bộ |
| CR-AG-003 `relay-websocket` mode | ✅ Khớp cơ chế | `agent-connection-relay.ts` implement đúng vai trò WS server + Bearer token qua query hoặc header, nhưng khác vài chi tiết nhỏ (xem 2.2) |
| CR-AG-004 `direct-websocket` mode | ❌ Sai lệch — **port** | Doc/CR-AG-004/F29/BL-AWS-02/HLD đều ghi `ws://orca:6768/agent`; code thật attach `AgentWebSocketServer` vào `httpPort` (mặc định 6769), và bản thân code backend **tự mâu thuẫn** giữa comment (6769) và message lỗi runtime (6768) |
| Keepalive & timeout | ❌ Sai lệch | F29 ghi "KeepAlive mỗi 30s, timeout 90s"; code thật dùng `AGENT_KEEPALIVE_INTERVAL_MS=5000`, `TIMEOUT_MS=20000` (khớp đúng CR-AG-001 §3.4, không khớp F29) |
| Close/error codes khi handshake thất bại | ❌ Sai lệch | F29 và BL-AWS-02 ghi custom close code `4001/4002/4003(/4004)`; code thật dùng WS chuẩn `1008`/`1011` toàn bộ, kèm JSON-RPC error `-33101 AuthFailed` (đúng CR-AG-001 §3.7) |
| Token lifecycle/renewal (BL-AWS-03) | ❌ Sai lệch nghiêm trọng | Doc mô tả admin UI tạo token 64-hex random + lưu DB `orca_agent_tokens` + revoke qua `DELETE /admin/api/agent-tokens/:id`; code thật là `AgentTokenManager` tự gia hạn qua `POST /api/agent-token` (auth bằng `ORCA_AGENT_API_SECRET`), token dạng dự đoán được `agt-<devServerId>-<timestamp>`, không có bảng DB nào, không có endpoint revoke |
| Reconnect/backoff | ⚠️ Một phần | ADR-019 mô tả công thức `1000 * 2^n` cap 30s (1→2→4→8→16→30s); code thật dùng mảng cố định `[1000,2000,5000,15000,30000]` — cùng ý tưởng nhưng số khác |
| ADR-013/017/018/019 "v6.0 Dev Server Agent" (A0–A4, `src/agent/`, HMAC signed context) | ❌ **Hoàn toàn chưa implement** — vision khác code thật | Cả 4 ADR tự ghi "❌ Chưa implement / 🚧 Proposed"; không có `ContextVerifier`, `SignedExecutionContext`, `_ctx`, `ReconnectManager` class, hay layer A0–A4 nào trong `agent/src` — package thật (`agent/src/relay/*`) vẫn là kiến trúc Phase-2 CR-AG-00x (giao thức 13-byte header, không có signed context) |
| `deploy/dev/agent/agent.js` + scripts (agent-connection-modes.md) | ❌ Lạc hậu | File/script tham chiếu (`deploy/dev/agent/agent.js`, `start-agent-direct.sh`, `connect-agent.sh`) **không còn tồn tại** trong repo — đã được thay bằng agent TypeScript `agent/src/relay/agent-entry.ts` (tự ghi trong comment: "Replaces: deploy/dev/agent/agent.js (CommonJS v1.0)") |

---

## 2. Chi tiết theo mục

### 2.1 Wire Protocol Framing (CR-AG-001 / F29 / HLD §5)

- Frame 13-byte `[TYPE u8][SEQ u32BE][ACK u32BE][LEN u32BE][PAYLOAD]` — ✅ đúng, thấy tại `agent/src/main/ssh/relay-protocol.ts:14,18-21` (`MessageType.Regular=1`, `MessageType.KeepAlive=9`) và `agent/src/relay/agent-wire.ts:41-65` (`encodeDataFrame`/`encodeKeepaliveFrame`). Khớp `CR-AG-001-wire-protocol-spec.md:41-59` và `HLD §5` (dòng 155-166).
- **Doc tự mâu thuẫn**: `ADR-004-relay-binary-ssh-remote-execution.md:59` ghi `TYPE: 0=DATA, 1=ACK, 2=KEEPALIVE, 3=CLOSE`; `BL-AWS-01-relay-websocket.md:75-80` ghi `0x00=DATA, 0x01=ACK, 0x02=KEEPALIVE(30s), 0x03=CLOSE`. Cả hai đều **sai** so với code thật và sai so với chính CR-AG-001/F29/HLD trong cùng bộ tài liệu — đây là mâu thuẫn nội bộ giữa các tài liệu, không chỉ giữa doc và code.
- Max frame payload 16MB — ✅ đúng, `MAX_MESSAGE_SIZE = 16 * 1024 * 1024` tại `agent/src/main/ssh/relay-protocol.ts:15`, khớp `F29-agent-websocket-protocol.md:159`.
- **Keepalive/timeout sai lệch với F29**: F29 (`docs/features/F29-agent-websocket-protocol.md:122,157`) ghi "KeepAlive gửi mỗi 30s, timeout 90s". Code thật dùng `AGENT_KEEPALIVE_INTERVAL_MS = 5_000`, `AGENT_TIMEOUT_MS = 20_000` (`agent/src/shared/agent-wire-protocol.ts:21-22`), và `KEEPALIVE_SEND_MS=5_000`/`TIMEOUT_MS=20_000` (`agent/src/main/ssh/relay-protocol.ts:24-25`, dùng thực tế tại `ssh-channel-multiplexer.ts:499,525,534,540`). Giá trị này khớp đúng `CR-AG-001-wire-protocol-spec.md:113-114` (5000ms/20000ms) — chỉ F29 sai.
- `AGENT_MIN_VERSION = '1.0.0'` được định nghĩa (`agent/src/shared/agent-wire-protocol.ts:31`) nhưng **không hề được đọc/so sánh** ở bất kỳ đâu trong `agent/src` hay `backend/src` (grep xác nhận chỉ xuất hiện trong khai báo và trong test kiểm tra giá trị literal của chính nó) — cơ chế "version mismatch → -33100 HandshakeFailed" mà CR-AG-001 §3.5 mô tả (`CR-AG-001-wire-protocol-spec.md:150-160`) là **hằng số chết**, chưa có logic thực thi.

### 2.2 `relay-websocket` mode (CR-AG-003, agent là WS server)

- ✅ Khớp cơ chế: `agent/src/relay/agent-connection-relay.ts:44-45` mở `WebSocketServer` tại path `/orca-relay`, port `config.agentPort` (mặc định `6799`, `agent-config.ts:97`) — khớp `CR-AG-003-relay-websocket-mode.md:26,166` và `F29:46`.
- Token bắt buộc, không fallback: `agent-connection-relay.ts:32-42` — nếu `ORCA_AGENT_TOKEN` rỗng thì `process.exit(1)`; đây là siết chặt hơn pseudocode CR-AG-003 (`agent.yaml` default `secret_token`) — cải thiện bảo mật so với thiết kế gốc.
- Auth chấp nhận cả `?token=` query lẫn `Authorization: Bearer` (`agent-connection-relay.ts:96-129`) — khớp `CR-AG-003:158-162` và `agent-connection-modes.md:365-368` (mô tả `agent.js listenRelay()`), dù bản `agent.js` gốc không còn tồn tại trong repo (xem 2.6).

### 2.3 `direct-websocket` mode (CR-AG-004) — port mismatch

- **Doc ghi port 6768**: `CR-AG-004-direct-websocket-mode.md:24,40,254-261,293`; `F29-agent-websocket-protocol.md:56,119`; `BL-AWS-02-direct-websocket.md:19`; `docs/hld/dev-server-architecture.md:136`; `docs/logic/agent-ws.md:16`.
- **Code thật (backend)**: `backend/src/server/index.ts:46-47` — `rpcPort` mặc định `6768`, `httpPort = rpcPort + 1` mặc định `6769`; `AgentWebSocketServer.attach(httpServer)` được gọi tại dòng `106` gắn vào **httpServer trên `httpPort`**, log dòng `108`: `` `[Orca Server] Agent WS: ws://0.0.0.0:${httpPort}/agent` `` — tức **6769**, không phải 6768.
- **Code tự mâu thuẫn nội bộ**: `backend/src/main/dev-server/agent-ws-server.ts:5-6` ghi đúng trong comment (`Browser → :6768/`, `Agent → :6769/agent`), nhưng thông báo lỗi runtime khi slot hết hạn tại chính file này lại ghi sai: `agent-ws-server.ts:103` — `` `ORCA_URL=ws://<orca-host>:6768${AGENT_WS_PATH}` `` — hướng dẫn người dùng cấu hình agent trỏ tới cổng **sai** (6768 thay vì 6769).
- Config mặc định phía agent (`agent/src/relay/agent-config.ts:86`): `ORCA_URL` mặc định `'wss://b15.openledger.vn/agent'` — hardcode domain production thật, không có port tường minh (dựa vào TLS 443 + reverse-proxy Nginx, đúng như `docs/flows/code/agent-ws/agent-connection-modes.md:39` mô tả hạ tầng thật `Nginx → :6769/agent`). Đây là bằng chứng gián tiếp xác nhận **6769 mới là port thật**, không phải 6768 như đa số doc ghi.

### 2.4 Close/error codes khi handshake thất bại

- F29 (`F29:95`) và BL-AWS-02 (`BL-AWS-02-direct-websocket.md:83-88`) ghi custom close code `4001=Unauthorized/Invalid token`, `4002=HandshakeTimeout`, `4003=VersionMismatch`, `4004=Server at capacity`.
- Code thật **không dùng các mã này ở đâu cả** trong luồng WS thật của agent — grep toàn repo `agent/src` + `backend/src/main/dev-server` chỉ tìm thấy `ws.close(1008, ...)` (`agent/src/relay/agent-connection-relay.ts:123`; `agent/src/relay/agent-session.ts:309`; `backend/src/main/dev-server/agent-ws-server.ts:156`; `backend/src/main/dev-server/ws-handshake.ts:200`) và `ws.close(1011, ...)` (`agent-session.ts:246`) — toàn bộ là mã WS chuẩn (Policy Violation / Server Error), không phải mã ứng dụng 4xxx. Mã `4001` chỉ xuất hiện trong một test không liên quan (`remote-runtime-shared-control-connection.test.ts:634`, protocol E2EE khác).
- Điểm đúng: lỗi auth **có** kèm JSON-RPC error frame `code: AgentErrorCode.AuthFailed` (`-33101`) trước khi đóng WS — `backend/src/main/dev-server/ws-handshake.ts:180-203` — khớp đúng `CR-AG-001-wire-protocol-spec.md` bảng error code §3.7. Vấn đề chỉ nằm ở **WS close code**, không phải JSON-RPC error code.

### 2.5 Token lifecycle/renewal — sai lệch nghiêm trọng nhất về mô hình

- **Doc (`BL-AWS-03-token-management.md`)** mô tả: Admin bấm "Generate Token" trên UI → `crypto.randomBytes(32).toString('hex')` (64 hex) → hash SHA-256 lưu trong **DevServer config**, hoặc theo `docs/flows/logic/agent-ws.md:104-140` là admin API `POST /admin/api/agent-tokens` lưu bảng **`orca_agent_tokens`** trong DB, revoke qua `DELETE /admin/api/agent-tokens/:id`.
- **Code thật hoàn toàn khác mô hình**: token được **agent tự yêu cầu** qua `POST /api/agent-token` (`backend/src/server/agent-token-routes.ts`), auth bằng `Authorization: Bearer <ORCA_AGENT_API_SECRET>` (không phải admin session — `agent-token-routes.ts:38-51`); token có định dạng **đoán được** `agt-<devServerId>-<timestamp>` sinh bởi `generateAgentToken()` (`agent/src/shared/agent-wire-protocol.ts:89-91`), không phải `crypto.randomBytes(32)`; **không có bảng DB nào** — chỉ có `Map` in-memory `pendingMeta`/`pendingSlots` (`agent-token-routes.ts:33`, `backend/src/main/dev-server/agent-ws-server.ts:46`); **không có endpoint revoke** trong file này.
- Điểm khớp: token có hash SHA-256 trước khi dùng làm key tra cứu (`agent-ws-server.ts:93,214-216`, comment "FIX TASK-AWS-002") — đúng nguyên tắc "không lưu plaintext" mà F29/BL-AWS-03 đề cập, dù cơ chế tổng thể (nguồn phát token, nơi lưu, cách revoke) khác hẳn.
- **Agent-side renewal hoàn toàn không có trong bất kỳ doc nào đã đọc**: `agent/src/relay/agent-token-manager.ts` implement `AgentTokenManager` với renewal chủ động ở 80% TTL (`TOKEN_RENEW_RATIO = 0.8`, dòng 29), TTL mặc định 24h (`AGENT_TOKEN_DEFAULT_TTL_SEC = 86_400`, dòng 32), pre-fetch token kế tiếp giữ trong `.next` để dùng ngay khi reconnect (dòng 95-106) — logic này phức tạp và tinh vi hơn nhiều so với những gì `BL-AWS-03`/`CR-AG-004` mô tả (token "one-time TTL 600s" theo `agent-connection-modes.md:15`), và không doc nào trong phạm vi audit này ghi lại cơ chế renew chủ động này.

### 2.6 Reconnect/backoff

- `ADR-019-agent-autonomous-operation-reconnect.md:65-68` định nghĩa công thức `Math.min(1000 * Math.pow(2, attempts), 30_000)` → dãy 1s, 2s, 4s, 8s, 16s, 30s.
- Code thật (`agent/src/relay/agent-connection-direct.ts:27`): `RECONNECT_DELAYS_MS = [1_000, 2_000, 5_000, 15_000, 30_000]` — cùng ý tưởng (exponential-ish, cap 30s) nhưng **giá trị cụ thể khác** (5s và 15s thay vì 4s/8s/16s). ADR-019 tự ghi trạng thái "❌ Chưa implement" (dòng 213) dù thực tế logic reconnect **đã có** trong `agent-connection-direct.ts` — chỉ là công thức không khớp 100% với ADR.
- Logic force-renew token khi reconnect sau khi đã handshake thành công (`agent-connection-direct.ts:99-126`, comment "FIX BUG-DS-AWS") là chi tiết vận hành thực tế không được ADR-019 hay bất kỳ doc nào mô tả trước.

### 2.7 ADR-013/017/018/019 (v6.0 "Dev Server Agent") — vision khác hẳn code thật

Đây là phát hiện quan trọng nhất, tương tự phát hiện "port 6768/6769" của audit backend:

- **ADR-013** (`docs/adrs/v1/ADR-013-dev-server-agent-replaces-relay.md:174`) tự ghi trạng thái `❌ Chưa implement (v6.0 proposed)`, dự kiến `src/agent/` package mới, `AgentConnectionManager`, `SignedContextIssuer`, `AgentDispatcher`.
- **ADR-017** (layer model A0–A4: `rpc/agent-rpc-server.ts`, `rpc/context-verifier.ts`, `pty/pty-manager.ts`, `storage/ai-credential-store.ts`...) tự ghi `❌ src/agent/ package chưa tồn tại` (dòng 221).
- **ADR-018** (Control Plane/Data Plane, HMAC-SHA256 signed `RpcExecutionContext` 30s TTL) tự ghi `🚧 Pattern định nghĩa; cần enforce qua code review` (dòng 175) — chưa có ESLint rule, chưa có `context-verifier.ts`.
- **ADR-019** (`ReconnectManager` class riêng, SQLite local state `agent_pty_sessions`/`agent_worktrees`/`agent_task_runs`, `EventEmitter` với buffer 1000 events + replay) tự ghi `❌ Chưa implement` (dòng 213).
- **Xác nhận bằng grep trên code thật**: không tìm thấy `ContextVerifier`, `SignedExecutionContext`, hay field `_ctx` ở bất kỳ đâu trong `agent/src` (kết quả rỗng). Cấu trúc thư mục `agent/src/relay/*` (flat, không phân lớp A0–A4) khác hoàn toàn so với cấu trúc `src/agent/{rpc,pty,worktree,git,fs,execution,storage,reporting}/` mà ADR-017 mô tả.
- **Kết luận**: package `agent/` thật trong repo hiện tại triển khai đúng kiến trúc **CR-AG-001→004 (Phase 2, 13-byte header, không có signed context)** — không phải kiến trúc "Dev Server Agent v6.0" mà ADR-013/017/018/019 mô tả. `docs/flows/code/agent-ws/agent-connection-modes.md` mục 7-9 (HMAC signed context, `pty-session-store.ts`, `profile-aware-agent-spawner.ts` tại `src/agent/...`) trích dẫn các file thuộc vision v6.0 **chưa hề tồn tại** trong `agent/src` thật — toàn bộ các đường link "Files Liên Quan" trỏ tới `src/agent/rpc/context-verifier.ts`, `src/agent/pty/pty-session-store.ts` ở cuối tài liệu này (dòng 771-773) là đường dẫn không có thật trong code hiện tại.

### 2.8 Tài liệu triển khai lạc hậu (`agent-connection-modes.md`)

- Tài liệu mô tả chi tiết `deploy/dev/agent/agent.js` (CommonJS), script `start-agent-direct.sh`, `connect-agent.sh` — cả thư mục `deploy/dev/agent/` **không còn tồn tại**; `deploy/dev/scripts/` hiện chỉ còn `build-local.sh`, `gen-certs.sh`, `get-pairing-url.sh`, `setup-ssh-keys.sh`, `sync-to-server.sh` — không có `start-agent-direct.sh` hay `connect-agent.sh`.
- Agent thật là gói TypeScript build sẵn: `agent/src/relay/agent-entry.ts:2-6` tự ghi chú "Orca Dev Agent v2.1 ... Replaces: deploy/dev/agent/agent.js (CommonJS v1.0) ... Built by: pnpm run build:agent → out/relay/agent.js". Log khởi động thực tế in `"Orca Dev Agent v2.1.0"` (`agent-entry.ts:35`), nhưng payload `agent.handshake` lại gửi `agentVersion: '5.0.0'` hardcode (`agent/src/relay/agent-session.ts:201`) — hai con số version khác nhau ngay trong cùng file/luồng, không doc nào phản ánh version thật của agent.

---

## 3. Nhận định tổng quan

1. **Bộ tài liệu CR-AG-001→004 (Phase 2) mô tả khá sát với `agent/src/relay/*` thật** — khung frame 13-byte, 2 mode kết nối, cấu trúc handshake đều đúng ý tưởng. Vấn đề chủ yếu là **sai số cụ thể** (port 6768/6769, keepalive 5s/30s, close code 1008/4001) và tài liệu **mâu thuẫn lẫn nhau** trong cùng bộ (ADR-004, BL-AWS-01 dùng bảng TYPE khác hẳn CR-AG-001).
2. **Khoảng trống thiết kế thật sự lớn nhất**: ADR-013/017/018/019 (v2, "Dev Server Agent v6.0") mô tả một kiến trúc hoàn toàn khác — `src/agent/` phân lớp A0–A4, HMAC-signed `RpcExecutionContext`, `ContextVerifier`, local SQLite state, `ReconnectManager` riêng — nhưng cả 4 ADR này tự đánh dấu "chưa implement", và code thật xác nhận đúng như vậy (không có `ContextVerifier`/`_ctx`/signed context ở đâu cả). `docs/flows/code/agent-ws/agent-connection-modes.md` lại trộn lẫn cả 2 thế hệ thiết kế (Phase 2 code thật + v6.0 vision) trong cùng một file, khiến người đọc dễ tưởng các file `src/agent/rpc/context-verifier.ts` v.v. đã tồn tại.
3. **Token lifecycle là điểm sai lệch mô hình nghiêm trọng nhất** (không chỉ sai tên): tài liệu mô tả một hệ thống quản lý token qua admin UI + DB, còn code thật là cơ chế self-service dựa trên shared secret (`ORCA_AGENT_API_SECRET`) với renewal chủ động phức tạp (80% TTL, pre-fetch) — hai mô hình bảo mật khác nhau về bản chất, không đơn thuần là đổi tên hàm.
4. **Một số cải thiện bảo mật trong code không được doc ghi nhận**: bắt buộc token trong `relay-websocket` (không fallback insecure default), hash SHA-256 token trước khi lưu vào `pendingSlots` map (chống rò rỉ qua heap dump), force-renew token ngay khi reconnect sau handshake thành công.

## 4. Khuyến nghị

- **Hợp nhất bảng TYPE giữa các tài liệu**: xóa/sửa `ADR-004:59` và `BL-AWS-01:75-80` cho khớp với `CR-AG-001`/code thật (`0x01 Regular`/`0x09 KeepAlive`).
- **Sửa port 6768→6769** ở mọi nơi ghi cổng `direct-websocket` (CR-AG-004, F29, BL-AWS-02, HLD §4) — đồng thời sửa **luôn message lỗi trong code** `agent-ws-server.ts:103` (hiện đang tự mâu thuẫn với comment dòng 5-6 của chính file).
- **Sửa keepalive/timeout trong F29** từ "30s/90s" thành "5s/20s" cho khớp CR-AG-001 và code.
- **Sửa close code trong F29/BL-AWS-02** từ `4001-4004` thành mã WS chuẩn thật (`1008`, `1011`) + ghi rõ JSON-RPC error code `-33101 AuthFailed` đi kèm.
- **Viết lại BL-AWS-03** theo đúng mô hình thật (self-service API + `ORCA_AGENT_API_SECRET`, không phải admin UI + DB), và bổ sung tài liệu cho `AgentTokenManager` (renewal 80% TTL) — hiện chưa có doc nào mô tả cơ chế này.
- **Làm rõ trạng thái ADR-013/017/018/019**: hoặc gắn nhãn rõ ràng "tầm nhìn tương lai, chưa triển khai" ngay đầu `agent-connection-modes.md` mục 7-9 để tránh nhầm với code thật, hoặc bỏ các đường link tới file `src/agent/...` không tồn tại.
- **Dọn tài liệu deploy cũ**: cập nhật hoặc xóa phần script `deploy/dev/agent/agent.js`, `start-agent-direct.sh`, `connect-agent.sh` trong `agent-connection-modes.md` vì các file này không còn trong repo.

---

*Phạm vi: Connection Modes & Wire Protocol của `agent/` — một trong 5 mảng của audit tổng `agent/`, xem chỉ mục tại `audit/agent/agent-vs-design-review.md`.*
