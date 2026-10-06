# FE-REQ-TASK-022-03: `useRequestSummaries`, `useMinuteClock` và định dạng thời gian tương đối

**From Solution:** [FE-REQ-SOL-022](../solutions/FE-REQ-SOL-022-approval-inbox.md) mục 2.3, 2.5 (D3)
**Priority:** P1
**Area:** frontend (hooks, lib)
**File:** `frontend/src/renderer/src/hooks/useRequestSummaries.ts` (mới), `hooks/useMinuteClock.ts` (mới), `lib/request-relative-time.ts` (mới), tests `.test.ts(x)` cùng tên (mới)
**Depends on:** FE-REQ-SOL-018 (`callRequestRpc`, `parseRequest`, slice `requestsById`, `upsertRequests`)
**Status:** [ ] TODO

## Context

- `lib/mcp-relative-time.ts` (`formatMcpRelativeTime(iso, now)`) dùng `Intl.RelativeTimeFormat(undefined, ...)` nên theo locale trình duyệt, không theo `i18n.language` của app; không có nhánh "quá hạn" riêng. Không dùng lại, không sửa (thuộc MCP).
- `hooks/useMcpApprovalDeadline.ts` tick 1 Hz cho một deadline; hộp duyệt có tới hàng chục hàng nên cần một timer chung 60 s.
- `i18n/i18n.ts` xuất `i18n` (`i18n.language`) và `translate(key, fallback, options)`.
- `request.list` (CR-016) không có bộ lọc theo danh sách id, nên tóm tắt phải lấy bằng `request.get {id}`. Mẫu giới hạn đồng thời: `FETCH_CONCURRENCY = 4` trong `useTaskDependencyEdges.ts`.

## Việc cần làm

1. `lib/request-relative-time.ts`:
   - `formatRelativeTime(iso: string | undefined, now: number, locale: string): string` dùng `Intl.RelativeTimeFormat(locale, {numeric:'auto'})`, đơn vị year/month/day/hour/minute (không giây: tick 1 phút), trả `'-'` khi `iso` không hợp lệ. Dùng `Math.trunc` giống mẫu để "còn 59 phút" không làm tròn lên.
   - `formatDueState(dueAt, now, locale): {kind:'none'|'upcoming'|'overdue'; text: string}`; `overdue` khi `dueAt < now`.
   - `formatAbsoluteTime(iso, locale)` cho tooltip (`Intl.DateTimeFormat(locale, {dateStyle:'medium', timeStyle:'short'})`).
2. `hooks/useMinuteClock.ts`: `useMinuteClock(enabled = true): number` trả `Date.now()` cập nhật mỗi 60 s qua một `setInterval` cho mỗi component gắn hook (chỉ gắn ở `ApprovalInboxTab` và truyền xuống bằng props, không ở từng hàng); căn lại ngay khi `visibilitychange` về `visible`; dọn timer khi unmount.
3. `hooks/useRequestSummaries.ts`: `useRequestSummaries(requestIds: string[]): {byId: Record<string, OrcaRequest>; isLoading: boolean; failedIds: Set<string>}`:
   - đọc `requestsById` từ store; chỉ gọi `request.get {id}` cho id chưa có (hoặc đã quá 5 phút, `fetchedAt` giữ trong ref cục bộ);
   - tối đa 4 lời gọi đồng thời, hàng đợi theo thứ tự xuất hiện; lỗi một Request thêm vào `failedIds` (không thử lại liên tục: thử lại tối đa một lần ở lần `requestIds` đổi tiếp theo);
   - kết quả đi qua `parseRequest` rồi `upsertRequests`; mã `REQUEST_NOT_FOUND` đưa id vào `failedIds` kèm cờ `notFound` để UI bỏ hàng (task 05, 06);
   - khử trùng id đầu vào; không tạo effect mới khi mảng id đổi tham chiếu nhưng cùng nội dung (khoá bằng `ids.join(',')`).

## Kiểm thử

- `request-relative-time.test.ts`: `now` cố định; "còn 3 giờ", "quá hạn 2 ngày" qua `Intl` (so khớp bằng regex theo locale `en`, và một locale khác như `ja` chỉ kiểm không ném lỗi); `iso` rác → `'-'`; ranh giới đúng 0 phút.
- `useMinuteClock.test.tsx`: fake timers, tăng 60 s cập nhật một lần; unmount dọn interval (`vi.getTimerCount()` bằng 0).
- `useRequestSummaries.test.tsx`: mock `callRequestRpc`; 10 id gọi tối đa 4 đồng thời (đếm đỉnh); id đã có trong store không gọi; một lỗi không làm hỏng các id khác; `REQUEST_NOT_FOUND` đánh dấu `notFound`.

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/hooks/useRequestSummaries.test.tsx frontend/src/renderer/src/hooks/useMinuteClock.test.tsx frontend/src/renderer/src/lib/request-relative-time.test.ts`.

## Tiêu chí hoàn thành

- [ ] Thời gian tương đối theo `i18n.language`, không theo locale trình duyệt.
- [ ] Số lời gọi `request.get` đồng thời không vượt 4; id đã cache không gọi lại trong 5 phút.
- [ ] Một timer duy nhất cho cả hộp duyệt; dọn khi unmount.
- [ ] Hook dùng lại được ở CR-REQ-023 (không tham chiếu thành phần hộp duyệt).

## Rủi ro và lưu ý

- N+1 là hệ quả của việc backend chưa nhúng tóm tắt (CR-022 Q1). Nếu backend nhúng sau, `useRequestSummaries` vẫn hữu ích cho CR-REQ-023 (nhóm Task/Execute chỉ có `requestId`).
- Hạn tính theo giờ máy chủ, nhưng so với đồng hồ máy khách (`Date.now()`); lệch đồng hồ làm sai "quá hạn" vài phút. Chỉ hiển thị; backend vẫn là nguồn thật khi duyệt (`REQUEST_APPROVAL_EXPIRED`).
- Cache `requestsById` dùng chung với `RequestsTab` (CR-REQ-019); không xoá phần tử ở đây.
