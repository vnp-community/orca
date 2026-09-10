# TASK-EMU-006: Nối `emulator-entry.ts` với transport thật

**Solution:** [SOL-EMU-001](../solutions/SOL-EMU-001-shared-transport-extraction.md)
**Priority:** P1
**Depends on:** TASK-EMU-001
**Status:** `[x]` DONE — direct-websocket mode thật đã nối, verify build +
test xanh trong pass này. Chưa verify với `infra-fleet-service` thật chạy
local (xem mục "Chưa verify" bên dưới).

## Việc đã làm

1. `emulator/src/relay/emulator-session.ts` (mới) — session/handshake nhỏ
   riêng cho Mobile Emulator Agent, **không** copy/import
   `agent/src/relay/agent-session.ts` (đúng ranh giới bảo mật CR-DS-009: agent
   này không có dispatcher cho `git.*`/`pty.*`, chỉ có
   `EmulatorRpcDispatcher` — `device.*`). Gửi handshake
   `{ agentVersion, platform, arch, capabilities: ['device'], agentToken }`
   (mirror shape mô tả trong `specs/agent/tdd/v5/04-handshake-session.md`,
   nhưng theo đúng shape response thật `{ result: { ok, orcaVersion,
   sessionId } }` mà `backend/src/main/dev-server/ws-handshake.ts` thực sự
   gửi — không theo mô tả `status`/`code=1008`-làm-RPC-error đã lỗi thời của
   spec doc). Dùng `createWireState`/`encodeDataFrame`/`decodeFrame`/
   `parseJsonPayload`/`encodeKeepaliveFrame`/`MessageType` từ
   `orca-dev-agent-transport`. Không có capability-detection git/pty, không
   PTY/fs-watch cleanup trong `stop()` (agent này không tạo PTY/watcher).
2. `emulator/src/relay/emulator-connection-direct.ts` (mới) — dial WS outbound
   tới `ORCA_BACKEND_URL` bằng package `ws`, reconnect-with-backoff đơn giản
   (1s→2s→5s→15s→30s, mirror `agent-connection-direct.ts` nhưng bỏ
   `AgentTokenManager`/token renewal và instrumentation debug — không cần
   cho pass này).
3. `emulator/src/relay/emulator-entry.ts`: nếu `config.backendUrl`
   (`ORCA_BACKEND_URL`) có set → `connectDirect(config, log, dispatcher)`
   (chế độ thật); nếu không → giữ nguyên chế độ stdio debug hiện có (đã có từ
   TASK-EMU-005), không đổi hành vi stdio mode.
4. `emulator/package.json` thêm `"orca-dev-agent-transport": "workspace:*"`.
5. `emulator/src/relay/emulator-session.test.ts` (mới, 15 test) — mock
   `WebSocket` bằng `EventEmitter` (cùng style
   `agent/src/relay/__tests__/agent-session.test.ts`): xác nhận handshake
   gửi đúng method/params (`capabilities: ['device']`, không có `tools`),
   xử lý đúng response ok (`onHandshakeOk` fire) và lỗi (đóng ws code 1008),
   dispatch đúng `device.*` sau handshake và gửi response, không dispatch
   trước khi handshake xong, không crash khi dispatcher throw, keepalive gửi
   mỗi 5000ms + trả lời keepalive đến, và một test round-trip qua
   `decodeFrame` thật của `orca-dev-agent-transport` để xác nhận frame gửi
   đi giải mã lại đúng.

## Verify (chạy thật trong pass này, có bằng chứng)

**tsc --noEmit cho `emulator/`:**
```
$ node agent/node_modules/typescript/bin/tsc --noEmit -p emulator/tsconfig.json
(không có output — sạch)
```

**vitest run cho toàn bộ `emulator/`** (bao gồm cả test cũ TASK-EMU-002..005):
```
Test Files  5 passed (5)
     Tests  28 passed (28)
```
(Trước khi thêm `emulator-session.test.ts`: 4 file / 13 test — toàn bộ test
cũ vẫn xanh, không sửa nội dung nào.)

**Build:**
```
$ node build.mjs
out/emulator.js  143.4kb
Build complete in 23ms
```

**Smoke test thủ công cả 2 chế độ** (không cần backend thật):
- Stdio debug mode (không set `ORCA_BACKEND_URL`) — không đổi hành vi so với
  TASK-EMU-005:
  ```
  $ echo '{"jsonrpc":"2.0","id":1,"method":"device.capabilities","params":{}}' | node out/emulator.js
  {"jsonrpc":"2.0","id":1,"result":{"platform":"linux","android":{...},"ios":{...}}}
  ```
- Direct-websocket mode (`ORCA_BACKEND_URL` set, backend không tồn tại) —
  xác nhận nhánh mới chạy đúng, dial thật, xử lý lỗi kết nối và
  reconnect-with-backoff đúng, không crash:
  ```
  $ ORCA_BACKEND_URL=ws://127.0.0.1:1/agent node out/emulator.js
  [info] orca-emulator-agent starting in direct-websocket mode (ws://127.0.0.1:1/agent)
  [info] Connecting to ws://127.0.0.1:1/agent ...
  [error] WebSocket error: connect ECONNREFUSED 127.0.0.1:1
  [warn] Connection dropped (code=1006). Reconnecting...
  [info] Reconnect in 1s (attempt 1)...
  ```

## Chưa verify (ngoài phạm vi pass này)

Mục "Verify" gốc của task này ("Kết nối thử tới một `infra-fleet-service`
chạy local … xác nhận `RegisterDevServer` nhận được kết nối, `device.capabilities`
relay qua `Relay`/`RelayByDevServer` trả kết quả đúng") **chưa chạy** — cần
`docker-compose` stack thật (`deploy/dev/docker-compose.yml`) và các thay đổi
`AgentKind`/`kind` ở `backend-go` (TASK-EMU-007, chưa làm) để
`RegisterDevServer` phân biệt được Mobile Emulator Agent với Dev Server
Agent. `emulator-session.ts`'s handshake hiện **không gửi `devServerId`**
(khác `agent-session.ts`) — cần xác nhận với backend-go thật xem
`AgentWebSocketServer`/`DevServerRelayBridge` phía Mobile Emulator Agent có
cần một field định danh tương đương hay không trước khi coi
end-to-end-với-backend-thật là xong.
