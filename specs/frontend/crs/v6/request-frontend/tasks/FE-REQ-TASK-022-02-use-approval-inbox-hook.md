# FE-REQ-TASK-022-02: Hook `useApprovalInbox` và phân loại kết cục lỗi quyết định

**From Solution:** [FE-REQ-SOL-022](../solutions/FE-REQ-SOL-022-approval-inbox.md) mục 2.3, 2.4
**Priority:** P1
**Area:** frontend (hooks, lib)
**File:** `frontend/src/renderer/src/hooks/useApprovalInbox.ts` (mới), `frontend/src/renderer/src/lib/approval-decision-outcome.ts` (mới), tests cùng tên `.test.ts(x)` (mới)
**Depends on:** FE-REQ-TASK-022-01; FE-REQ-SOL-018 (`callRequestRpc`, `RequestRpcError`, `subscribeRequestEvents`, slice `setPendingApprovalCount`, `setRequestFlowSupport`)
**Status:** [ ] TODO

## Context

- Mẫu hook: `hooks/useTaskSource.ts` (cờ `cancelled` khi unmount, nuốt lỗi khi runtime không có kênh) và `useTaskDependencyEdges.ts` (đồng thời có giới hạn, `refetchKey`).
- Kênh theo CR-REQ-016 mục 2.4: `approval.listPending {subjectType?, pageSize, pageToken}`; `approval.approve {id, expectedVersion, expectedDigest, comment?}`; `approval.reject {id, expectedVersion, expectedDigest, comment}`. Không có `total`. Không có luồng sự kiện riêng cho approval: chỉ `request.event` (mục 2.5 CR-016).
- Mã lỗi backend tiền tố `REQUEST_APPROVAL_*` (CR-009/010), bảng CR-016 ghi `APPROVAL_*`: khớp theo hậu tố.
- `useApprovals` của SOL-018 (`{approvals, pendingCount, approve, reject, refetch}`) không phân trang, không polling. Hook này là lớp riêng cho hộp duyệt, gọi cùng `callRequestRpc`; không sửa `useApprovals`.

## Việc cần làm

1. `approval-decision-outcome.ts`: `type ApprovalDecisionOutcome = 'ok'|'closed'|'changed'|'forbidden'|'validation'|'unsupported'|'network'|'unknown'` và `classifyApprovalDecisionError(err: unknown): {outcome; code?: string}`. Chuẩn hoá mã: bỏ tiền tố `REQUEST_`, so hậu tố theo bảng SOL-022 mục 2.4 (`ALREADY_DECIDED|EXPIRED|NOT_FOUND→closed`; `VERSION_CONFLICT|DIGEST_MISMATCH|STAGE_MISMATCH→changed`; `NOT_APPROVER|FORBIDDEN|SELF_APPROVAL_FORBIDDEN|AGENT_FORBIDDEN→forbidden`; `COMMENT_REQUIRED→validation`). `RequestRpcError.kind==='unsupported'→unsupported`, `network→network`.
2. `useApprovalInbox(options: {subjectGroup; projectId?: string; overdueOnly: boolean; active: boolean})` trả `{rows, isLoading, isLoadingMore, error, hasMore, loadMore, refetch, approve, reject, supported}`.
3. Tải trang đầu khi `active` và khi đổi `serverSubjectTypeFor(subjectGroup)`; `pageSize: 50`; ghép trang bằng `nextPageToken`, khử trùng theo `id`.
4. Lọc client: `matchesGroup`, `overdueOnly` (`isOverdue` với `now` từ `Date.now()` mỗi lần render dựa `useMinuteClock` ở task 03; ở task này nhận `now` qua tham số tuỳ chọn để test), `projectId` lọc qua `requestsById[row.requestId]?.projectId` (hàng chưa có tóm tắt thì tạm giữ).
5. Polling 30 s khi `active && document.visibilityState === 'visible'` (`setInterval` + `visibilitychange`); `subscribeRequestEvents` (SOL-018): sự kiện có `eventType` chứa `approval.` thì `refetch()` debounce 500 ms.
6. Sau mỗi lần tải trang đầu xong gọi `setPendingApprovalCount(n)` (n = số hàng trang đầu, tối đa 50; số chính xác do lời gọi đếm của SOL-018, xem task 06).
7. `approve(row)` / `reject(row, comment)`: gửi `expectedVersion: row.version`, `expectedDigest: row.subjectDigest`; trả `{outcome, code}`. Khi `closed` bỏ hàng khỏi state cục bộ; khi `changed` gọi `refetch()`; khi `unsupported` gọi `setRequestFlowSupport('unsupported')`; `reject` kiểm `comment.trim().length >= 10` trước khi gọi, không gửi nếu ngắn hơn (`validation`).
8. Gộp lỗi truyền tải vào `error` nhưng giữ `rows` cũ (để UI làm mờ).

## Kiểm thử

- `approval-decision-outcome.test.ts`: bảng mã cho cả hai tiền tố, mã lạ → `unknown`, `Error` thường → `unknown`, lỗi truyền tải → `network`.
- `useApprovalInbox.test.tsx` (`@vitest-environment happy-dom`, `renderHook`, mock `callRequestRpc`): trang đầu rồi `loadMore` (không trùng id); polling dùng fake timers; sự kiện `approval.decided` gây `refetch`; unmount không setState (cờ huỷ); `approve` gửi đúng `expectedVersion/expectedDigest`; `reject` comment 9 ký tự không gọi RPC; `closed` bỏ hàng; `changed` refetch; `unsupported` không toast và đặt flag.

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/hooks/useApprovalInbox.test.tsx frontend/src/renderer/src/lib/approval-decision-outcome.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mọi lệnh ghi gửi `expectedVersion` và `expectedDigest`; không bao giờ gọi `approval.approve` khi thiếu digest.
- [ ] Không có `tenantId`, `userId` trong tham số RPC.
- [ ] Polling dừng khi tab ẩn hoặc `active=false`; timer được dọn khi unmount.
- [ ] `unsupported` không phát toast, không log lỗi.
- [ ] Test xanh, không `any`.

## Rủi ro và lưu ý

- Chạy được trên runtime environment/SSH vì chỉ đi qua `callRequestRpc`; không dùng `window.api` riêng cục bộ.
- Hai nguồn đếm (`setPendingApprovalCount` ở đây và lời gọi nền của SOL-018) có thể chênh nhau tạm thời; task 06 chốt nguồn chính.
- `projectId` của hàng chỉ có sau khi `useRequestSummaries` nạp xong; lọc dự án có thể "nhấp nháy" khi dữ liệu về.
