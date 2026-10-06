# AG-CV-TASK-006-02: `registerCodeIntelHandlers` cho `RelayDispatcher` (cây `agent/`)

**From Solution:** [AG-CV-SOL-006-relay-ssh-part-b-handlers](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) mục 2.1
**Priority:** P2
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-relay-handlers.ts`, `codeintel-relay-handlers.test.ts` (mới)
**Depends on:** [001](./AG-CV-TASK-006-01-transport-neutral-core-guard.md); điều kiện: O-5 chấp thuận
**Status:** [ ] TODO

## Context
Mẫu `relay-auth-status-handlers.ts` (`registerAuthStatusHandlers`, `throw Object.assign(new Error, {code})`). Part B: id số, thông báo `dispatcher.notify` tới mọi client, `rpc.cancel` -> `context.signal`, không `capabilities`.

## Việc cần làm
1. `registerCodeIntelHandlers(dispatcher, config, log)`: duyệt `CODEINTEL_METHODS` (nạp động để lỗi nạp không phá khởi động relay, bọc `try/catch`); `setCodeIntelNotifier(dispatcher.notify)`; `signal: context.signal`.
2. `toRelayThrowable(err)` dùng `toErrorPayload`: `Object.assign(new Error(message), {code, data})`; lỗi lạ `-32000` không `data`.
3. Không `console.*`; log qua `log`.

## Kiểm thử
`RelayDispatcher` thật với `write` giả (khuôn `relay-auth-status-handlers.test.ts`): đăng ký đủ; `codeintel.nope` -> `-32601`; lỗi có `data` (sau task 03); `rpc.cancel`; `notify`; `isStale`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-relay-handlers.test.ts`

## Tiêu chí hoàn thành
- [ ] Mọi method trong bảng đăng ký ở Part B; cancel kill CLI con.

## Rủi ro
- `RelayDispatcher.onRequest` trùng chỉ cảnh báo rồi ghi đè (`dispatcher.ts:154-168`): đừng đăng ký hai lần.
