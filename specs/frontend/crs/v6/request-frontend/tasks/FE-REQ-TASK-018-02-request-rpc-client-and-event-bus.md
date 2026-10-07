# FE-REQ-TASK-018-02: Lớp gọi RPC `callRequestRpc`, luồng sự kiện và event bus

**From Solution:** [FE-REQ-SOL-018](../solutions/FE-REQ-SOL-018-request-frontend-foundation.md) mục 2.4, 2.5
**Priority:** P0
**Area:** frontend / renderer runtime
**File:** `frontend/src/renderer/src/runtime/request-rpc-client.ts` (mới), `frontend/src/renderer/src/lib/request-event-bus.ts` (mới), test `request-rpc-client.test.ts`, `request-event-bus.test.ts`
**Depends on:** FE-REQ-TASK-018-01
**Status:** [x] DONE

## Context

- `callRuntimeRpc(target, method, params)` và `getActiveRuntimeTarget(settings)` ở `runtime/runtime-rpc-client.ts:52,31`; lỗi là `RuntimeRpcCallError` (`runtime-rpc-result.ts`) có `code` và `message`.
- `subscribeRuntimeStreamChannel(target, method, params, onEvent)` (`runtime-rpc-client.ts:105`) trả `{ack, unsubscribe}` và reject với target `local`.
- Mẫu bus: `lib/mcp-event-bus.ts` (Set listener, bắt lỗi từng listener). Mẫu hook nuốt lỗi: `useTaskSource.ts`.
- Kênh: `request.subscribe {id?}` đẩy `request.event {requestId,eventType,status,type,occurredAt}` (CR-REQ-016 2.5), không kèm `body`.

## Việc cần làm

1. `callRequestRpc<T>(method: RequestRpcMethod, params?: object): Promise<Result<T, RequestRpcError>>`: lấy target từ `useAppStore.getState().settings`; bắt lỗi, trả `{ok:false,error: classifyRequestRpcError(e)}`; không bao giờ ném. Không log `params` (có `body` nội bộ).
2. `callRequestRpcOrThrow<T>` cho hook cần ném (nội bộ), dùng chung phần lõi.
3. `subscribeRequestEvents({requestId?, onEvent, onFallback}): () => void`: nếu `target.kind === 'environment'` gọi `subscribeRuntimeStreamChannel(target,'request.subscribe',{id: requestId},evt => onEvent(parseRequestEvent(evt)))`; nếu target `local`, hoặc reject, hoặc `unsupported` thì gọi `onFallback()` một lần để hook bật polling. Trả hàm huỷ; huỷ trước khi ack về vẫn phải gọi `unsubscribe` sau khi ack.
4. `lib/request-event-bus.ts`: `subscribeRequestBus(listener)`, `emitRequestEvent(event)`; lọc `try/catch` từng listener như `mcp-event-bus.ts`.
5. Không gắn gì vào `App.tsx` ở task này (task 018-03 làm `useRequestEvents`).

## Kiểm thử

- `request-rpc-client.test.ts`: mock `callRuntimeRpc` ném `RuntimeRpcCallError` với `message='REQUEST_VERSION_CONFLICT: x'` thành `kind:'conflict'`; `method_not_found` thành `unsupported`; lỗi mạng thành `network`; thành công trả `ok:true`; kiểm params không bị log (spy `console`).
- Sự kiện: target `local` gọi `onFallback`; target `environment` chuyển tiếp event đã parse; huỷ sớm gọi `unsubscribe`.
- Bus: một listener ném lỗi không chặn listener khác.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/runtime/request-rpc-client src/renderer/src/lib/request-event-bus`.

## Tiêu chí hoàn thành

- [ ] `callRequestRpc` không bao giờ ném; kiểu lỗi đúng bảng.
- [ ] Hoạt động cho cả target `local` và `environment` (SSH/remote đi qua environment).
- [ ] Target `local` rơi về polling mà không báo lỗi.
- [ ] Test xanh, `tsc` không lỗi mới.

## Rủi ro và lưu ý

- `eventType` thật chưa rõ: parser giữ nguyên chuỗi, so khớp ở hook bằng tập chấp nhận cả `request.status_changed` và `orca.request.request.status_changed`.
- Mỗi socket thêm một kết nối NATS ephemeral phía gateway (CR-016 mục 6): chỉ mở một stream toàn cục ở `App.tsx`, không mở mỗi màn.
