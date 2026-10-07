# FE-CV-TASK-050-13: `useCodeIntelQuery` và `useCodeIntelPagedQuery`

**From Solution:** [FE-CV-SOL-050-store-and-query-hooks](../solutions/FE-CV-SOL-050-store-and-query-hooks.md) mục 4.5
**Priority:** P0
**Area:** frontend / hooks
**File:** `hooks/useCodeIntelQuery.ts`, `hooks/useCodeIntelPagedQuery.ts` (mới), tests `.test.tsx`
**Depends on:** FE-CV-TASK-050-07, FE-CV-TASK-050-10, FE-CV-TASK-050-12
**Status:** [x] DONE

## Context

- UI-API §2.4: `CODEINTEL_TIMEOUT | {"retryAfterMs":3000,"inProgress":true}` ⇒ thử lại tự động tối đa 90 s; phân trang `limit` + `pageToken` opaque, `nextPageToken`.
- Mẫu huỷ khi gỡ: `hooks/useTaskSource.ts`.

## Việc cần làm

1. `useCodeIntelQuery(worktreeId, view, params, {enabled, scopeKey})`: `queryKey = view|scopeKey|hash(params)`; đọc cache trước; không gọi khi `enabled=false` hoặc support ≠ `enabled` hoặc selector `unsupported`; trả `{data, meta, status, error, stale, truncated, staleSignal, refetch, applyNow}`.
2. Thử lại `timeout+inProgress` (chờ `retryAfterMs ?? 3000`, ≤ 90 s tổng), huỷ khi gỡ, bỏ kết quả cũ khi tham số đổi.
3. Tải lại khi `codeIntelResyncCounter` đổi; `changed` thường chỉ đặt `staleSignal` (`applyNow` mới tải).
4. `useCodeIntelPagedQuery(…, {select})`: `items`, `fetchNextPage`, `hasNextPage`; loại trùng trang đang bay; đổi tham số reset.
5. Kết quả sai hình dạng ⇒ `error.kind='tool-failed'`.

## Kiểm thử

- Cache hit; retry 90 s (đồng hồ giả); huỷ; resync; staleSignal; phân trang (2 trang, trang trùng bay); disabled không gọi.

## Tiêu chí hoàn thành

- [ ] Không `any`; mọi lens dùng hook này, không tự gọi RPC.

## Rủi ro

- Khoá cache chứa `scopeKey` do caller: nếu caller quên, cache có thể trộn giữa phạm vi (test bắt buộc ở SOL-051).
