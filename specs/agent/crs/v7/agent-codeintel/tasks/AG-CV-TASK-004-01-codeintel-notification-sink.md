# AG-CV-TASK-004-01: Sink thông báo agent -> backend

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.5
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-notification-sink.ts`, `codeintel-notification-sink.test.ts` (mới); sửa nhỏ `agent-rpc-dispatch-codeintel.ts`
**Depends on:** [AG-CV-TASK-001-09](./AG-CV-TASK-001-09-codeintel-status-method-table-and-dispatcher.md)
**Status:** [x] DONE

## Context
`makeNotifier(ws,state)` (`agent-rpc-dispatch.ts:280`) gắn với `WireState` từng kết nối, bỏ khi ws không mở. Contract §6: notifier hiện hành = của lần gọi `codeintel.*`/`quality.*` gần nhất; mọi thông báo mang `workspaceRoot` (PQ-17); ws chưa mở -> bỏ.

## Việc cần làm
1. `type CodeIntelNotificationSink = (method, params) => void`; `setCodeIntelNotifier(sink)` và `emitCodeIntelNotification(method, params)` (bỏ nếu chưa đặt hoặc thiếu `workspaceRoot` kiểu string).
2. Dispatcher gọi `setCodeIntelNotifier(makeNotifier(ws,state))` mỗi lần xử lý method nhóm này (lõi không import `makeNotifier`).
3. Hàm `clearNotifierIfWs(ws)` khi phiên dừng (tuỳ chọn).

## Kiểm thử
Notifier đổi theo lần gọi; ws đóng -> không ném; thiếu `workspaceRoot` -> bị loại + log.
Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-notification-sink.test.ts`

## Tiêu chí hoàn thành
- [ ] Lõi sink không import transport.

## Rủi ro
- Nhiều phiên đồng thời: thông báo chỉ tới phiên gần nhất (chấp nhận MVP).
