# TASK-BE-013.4: Test suite cho Agent WebSocket tracing (handshake, token-verify, token-routes)

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-013](../solutions/SOL-BE-TRACE-013-agent-ws.md) §3
**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-013.2 + TASK-BE-013.3
**Status:** ✅ Done (2026-08-04) — created all 3 new Vitest files with real (non-stub) implementations: `dev-server-relay-bridge-tracing.test.ts` (6 tests, incl. a self-contained `vi.mock('ws')` fake since none existed in the codebase before), `agent-ws-server-tracing.test.ts` (5 tests, incl. the bugfix assertion comparing `span.id` between `start()` and `fail()`), `agent-token-routes-tracing.test.ts` (2 tests, using a real `http.createServer` harness matching `push-api-routes.test.ts` convention). All 13 pass; `pnpm run typecheck:node` clean for all 3 files. Noted drift: `src/main/dev-server/__tests__/agent-ws-server.test.ts` (pre-existing, NOT one of the 3 files this task creates) has 2 failing tests caused by its `mockWs = { close: vi.fn() }` lacking `.once()` — confirmed via `git show HEAD:...` that the `ws.once('close', ...)` call in `agent-ws-server.ts` predates this session entirely, so this is a pre-existing baseline gap, not a regression from TASK-BE-013.3's edit; left unmodified as out of this task's scope.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết test file mới — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các symbol đã instrument ở TASK-BE-013.2/013.3 trước khi viết test:

```bash
codegraph explore "DevServerRelayBridge.connectRelayWebSocket"
codegraph explore "AgentWsServer.handleConnection"
```

## Mô tả

Tạo 3 file test Vitest mới xác nhận hành vi tracing của `connectRelayWebSocket()` (TASK-BE-013.2), `handleConnection()` + nhánh GET token routes (TASK-BE-013.3) — bao gồm cả assertion trực tiếp xác nhận bugfix span mồ côi (span dùng lại `id` giống nhau giữa `start()` và `fail()` trong nhánh handshake-reject).

## File: `src/main/dev-server/__tests__/dev-server-relay-bridge-tracing.test.ts` [NEW]

Test cases cần cover:

- `connectRelayWebSocket() success`: mock WebSocket `open` → handshake resolve → assert `agentWsHandshakeFlow` nhận 1 `start()`, 1 `step('tcpConnected')`, 1 `ok()` với `platform`/`nodeVersion`
- `connectRelayWebSocket() — TCP timeout`: không bao giờ fire `open` trong 10s (dùng fake timers) → assert `span.fail()` với `phase:'tcpConnect'`
- `connectRelayWebSocket() — WS error trước open`: fire `ws.on('error')` → assert `fail()` với `phase:'tcpConnect'`
- `connectRelayWebSocket() — handshake reject`: `open` fires nhưng `runOrcaInitiatorHandshake` reject → assert `fail()` với `phase:'handshake'`
- `connectRelayWebSocket() — reconnect tạo span mới`: simulate `close` event sau khi đã connect thành công 1 lần → assert `attempt()` gọi lại tạo **span thứ 2** với `id` khác span đầu (không tái dùng `id` cũ)

Target: **≥ 5 test cases**.

## File: `src/main/dev-server/__tests__/agent-ws-server-tracing.test.ts` [NEW]

Test cases cần cover:

- `handleConnection() — span bắt đầu trước khi biết kết quả handshake`: assert `agentWsTokenVerifyFlow.start()` được gọi ngay khi `handleConnection()` invoke, TRƯỚC khi `runOrcaReceiverHandshake` resolve/reject
- `handleConnection() — token hợp lệ, slot tồn tại`: assert `step('tokenLookup', {tokenPrefix})` rồi `ok({devServerId, sessionId})`; verify `tokenPrefix` KHÔNG chứa token đầy đủ (length assertion: `tokenPrefix.length <= 15` — 12 ký tự + `...`)
- `handleConnection() — slot expired (race)`: mock handshake resolve nhưng slot đã bị xoá → assert `fail('slot-expired', {devServerId})`
- `handleConnection() — invalid token (xác nhận bugfix span mồ côi)`: mock handshake reject → assert `fail(err, {reason:'invalid-token'})` dùng ĐÚNG span đã mở từ đầu — so sánh `span.id` giữa lần `start()` và lần `fail()`, phải là cùng 1 span, KHÔNG phải span ngẫu nhiên mới (đây là assertion trực tiếp cho bugfix mô tả ở TASK-BE-013.3)
- `handleConnection() — agentWsFlow (lifecycle) không bị ảnh hưởng`: assert `Tracers.agentWsFlow.start()` vẫn được gọi đúng như hành vi cũ (connected → disconnect), độc lập `id` với `agentWsTokenVerifyFlow`

Target: **≥ 5 test cases**.

## File: `src/server/__tests__/agent-token-routes-tracing.test.ts` [NEW]

Test cases cần cover:

- `GET /api/agent-token — có ít nhất 1 trace event`: mock `pendingMeta` có 2 entries → assert `tokenTracer.start({op:'list'}).ok({count:2})` được gọi
- `GET /api/agent-token — không tạo tracer mới`: assert chỉ `agentToken:register` xuất hiện trong danh sách flow name của mọi event phát sinh từ route này (không có flow name mới nào)

Target: **≥ 2 test cases**.

## Verification

```bash
pnpm run typecheck:node
pnpm test --run src/main/dev-server/__tests__/dev-server-relay-bridge-tracing.test.ts
pnpm test --run src/main/dev-server/__tests__/agent-ws-server-tracing.test.ts
pnpm test --run src/server/__tests__/agent-token-routes-tracing.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `dev-server-relay-bridge-tracing.test.ts` có ≥ 5 test case, cover toàn bộ danh sách ở trên
- [ ] `agent-ws-server-tracing.test.ts` có ≥ 5 test case, cover toàn bộ danh sách ở trên, đặc biệt test bugfix span mồ côi (`span.id` giống nhau giữa `start()` và `fail()` trong nhánh invalid token)
- [ ] `agent-token-routes-tracing.test.ts` có ≥ 2 test case, cover toàn bộ danh sách ở trên
- [ ] Không có assertion nào chứa giá trị token đầy đủ — mọi test liên quan tới `tokenPrefix` chỉ so sánh độ dài/prefix, không so sánh token gốc thật
- [ ] Tất cả test pass với `pnpm test --run`
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
