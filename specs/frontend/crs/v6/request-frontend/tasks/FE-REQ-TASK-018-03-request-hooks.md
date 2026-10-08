# FE-REQ-TASK-018-03: Bộ hook Request, Solution, Approval, Backlog, sự kiện

**From Solution:** [FE-REQ-SOL-018](../solutions/FE-REQ-SOL-018-request-frontend-foundation.md) mục 2.5
**Priority:** P0
**Area:** frontend / renderer hooks
**File:** `frontend/src/renderer/src/hooks/useRequestFlowSupport.ts`, `useRequests.ts`, `useRequest.ts`, `useRequestActions.ts`, `useSolutions.ts`, `useApprovals.ts`, `useBacklog.ts`, `useRequestEvents.ts` (đều mới) và `*.test.ts(x)`; `frontend/src/renderer/src/App.tsx` (sửa, gắn `useRequestEvents`)
**Depends on:** FE-REQ-TASK-018-01, 018-02; store slice (018-04) cho `setRequestFlowSupport`, `setPendingApprovalCount`, `upsertRequests`
**Status:** [~] PARTIAL — hooks + tests for useRequests/useRequest*/useRequestActions/useRequestFlowSupport/useSolutions/useApprovals/useRequestEvents pass and useRequestEvents is mounted in App.tsx; no test for useBacklog, and its nextPage() does not send pageToken (belongs to 023-02)

## Context

- Hook mẫu: `useTaskSource.ts` (cờ `cancelled`), `useTasks.ts` (`refetchTrigger`), `useTaskActivity.ts` (polling 4 giây, `useReducer`).
- Kênh và tham số: SOL-018 mục 2.2. Phân trang theo `pageSize`/`pageToken` → `nextPageToken`.
- Không có `viewerCan` trong CR-016: hook không trả quyền, UI dựa lỗi `forbidden`.

## Việc cần làm

1. `useRequestFlowSupport()`: gọi `request.flowStatus` một lần mỗi target (cache theo `JSON.stringify(target)`), `enabled=true` thành `'supported'`; `unsupported`/`enabled=false` thành `'unsupported'`; lỗi mạng giữ `'unknown'` và thử lại khi focus cửa sổ. Ghi vào store qua `setRequestFlowSupport`.
2. `useRequests(filters)`: `request.list`; trả `{requests,isLoading,error,nextPage,refetch,supported}`; đổi `filters` thì huỷ yêu cầu cũ; `nextPage()` nối trang, khử trùng theo `id`; `upsertRequests` vào store.
3. `useRequest(id)`: gọi song song `request.get` và `request.typeHistory`; trả `{request,history,links,isLoading,error,refetch}`. `links` đọc từ `request.links` của `request.get` nếu có, nếu không `[]` và `linksSupported=false` (tạm, CR-016 câu hỏi mở 1).
4. `useRequestActions()`: `classify, confirmType, changeType, returnToBacklog, reopen, cancel, spawnChild, generatePlan, startPhase`, mỗi hàm `Result<T,RequestRpcError>`; chống bấm đôi bằng map `inFlight` theo `(action,id)`; `returnToBacklog`, `changeType`, `cancel` chặn `reason` rỗng ở client (`REQUEST_REASON_REQUIRED` giả lập `validation`).
5. `useSolutions(requestId)`: `solution.list`, `generate({feedback?, idempotencyKey})` (sinh `idempotencyKey` bằng `crypto.randomUUID()`), `choose({solutionId,optionId,comment?})`.
6. `useApprovals(filters)`: `approval.list` (khi có `requestId`) hoặc `approval.listPending`; `approve({approval,comment?})`, `reject({approval,comment})` gắn `expectedVersion`, `expectedDigest`; `reject` ném `validation` nếu `comment.trim().length < 10`; `pendingCount`.
7. `useBacklog(view, filters)`: ánh xạ `view` → `backlog.requests|tasks|execute`; parse bằng `parseBacklogItem`.
8. `useRequestEvents()`: gọi `subscribeRequestEvents`; mỗi event `emitRequestEvent` + `upsertRequests` khi đủ trường; fallback polling 15 giây (`RequestPage` mở) và `approval.listPending {pageSize:1}` 60 giây cập nhật `pendingApprovalCount`; chỉ khi `document.visibilityState==='visible'`. Gắn vào `App.tsx` một lần, chỉ chạy khi `requestFlowSupport==='supported'`.
9. Mọi hook huỷ khi unmount; không dùng `any`.

## Kiểm thử

- Mỗi hook một file test (`renderHook`, mock `callRequestRpc`): phân trang và khử trùng, huỷ khi unmount, `supported=false` khi `unsupported`, `reject` rỗng không gọi RPC, `approve` gửi đúng `expectedVersion`/`expectedDigest`, cache `useRequestFlowSupport` reset khi đổi target, `useRequestEvents` rơi về polling với target `local`, không polling khi tab ẩn.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/hooks/useRequest src/renderer/src/hooks/useSolutions src/renderer/src/hooks/useApprovals src/renderer/src/hooks/useBacklog`.

## Tiêu chí hoàn thành

- [ ] 8 hook + test; `App.tsx` chỉ thêm một dòng gọi hook (không thêm logic).
- [ ] Không toast trong hook; hiển thị lỗi là việc của component.
- [ ] Runtime không hỗ trợ: không lỗi, `supported=false`.
- [ ] `tsc` sạch lỗi mới.

## Rủi ro và lưu ý

- `solution.choose` nhận `optionId` (CR-016), README mục 8 số 5 nói `chosen_option` là chỉ số nguyên: `optionId` gửi đúng `SolutionOption.id` đã parse (có thể là `String(index)`); xác nhận khi có CONTRACT.
- Polling nhiều cửa sổ gây tải: giữ khoảng 15/60 giây, không rút ngắn.
