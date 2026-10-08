# FE-REQ-TASK-019-03: `RequestDetailPane`, tiêu đề, dòng thời gian, hành động hủy, trả về backlog, mở lại

**From Solution:** [FE-REQ-SOL-019](../solutions/FE-REQ-SOL-019-request-list-detail-classification-ui.md) mục 2.3
**Priority:** P0
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/RequestDetailPane.tsx`, `RequestDetailHeader.tsx`, `RequestStageTimeline.tsx`, `RequestOverviewTab.tsx`, `RequestBacklogBanner.tsx`, `ReturnToBacklogDialog.tsx`, `CancelRequestDialog.tsx` (đều mới); test cùng tên
**Depends on:** FE-REQ-TASK-018-03, 018-05, 019-01
**Status:** [x] DONE (verified 2026-10-07: RequestDetailPane.test.tsx 11 tests + request-action-rules.test.ts pass)

## Context

- Kênh: `request.get {id}`; `request.cancel {id,reason}`; `request.returnToBacklog {id,stage,reason}` (`stage ∈ classification|analysis|plan|phase|task`); `request.reopen {id}`. Mã lỗi: `REQUEST_REASON_REQUIRED`, `REQUEST_TRANSITION_NOT_ALLOWED`, `REQUEST_STATE_STALE`.
- CR-016 không có `viewerCan`: nút hiện đầy đủ, `forbidden` xử lý sau.
- `RequestAnalysisTab`, `RequestPlanTab` do SOL-020/021 cắm; ở task này là chỗ trống có `data-testid`.
- Dùng `ui/tabs.tsx`, `ui/dialog.tsx`, `ui/textarea.tsx`.

## Việc cần làm

1. `RequestDetailPane({requestId})`: `useRequest(id)`; tải (Skeleton), `not_found` ("Request không còn tồn tại" + nút Về danh sách), lỗi `network` (Thử lại); Tabs: Tổng quan, Phân tích (ẩn nếu registry `analysisKind===null`), Plan (ẩn nếu `plan==='none'`), Lịch sử, Liên quan.
2. `RequestStageTimeline`: vẽ `buildStageTimeline` (icon lucide + chữ, không chỉ màu), ghi chú hotfix.
3. `RequestDetailHeader`: `#number`, tiêu đề, `RequestStatusBadge`; nút theo bảng: Hủy (chưa `completed`/`cancelled`), Mở lại (`request_backlog`), Trả về backlog (trạng thái đang xử lý: `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`), Sửa loại (019-04), Tạo Request con (019-05).
4. `ReturnToBacklogDialog`: chọn `stage` (mặc định theo status hiện tại qua model), `reason` bắt buộc (cắt khoảng trắng, không rỗng), nút khoá khi rỗng; `isScreenSubmitShortcut` gửi.
5. `CancelRequestDialog`: lý do tuỳ chọn; xác nhận phá hủy (`variant="destructive"`).
6. `RequestBacklogBanner`: khi `request_backlog` hiện `returnedFromStage`, `returnReason`, người trả, thời điểm, nút "Mở lại".
7. Sau mỗi hành động thành công `refetch()` và `upsertRequests`; lỗi `conflict`/`invalid_state` tải lại và toast ngắn.
8. `RequestOverviewTab`: nội dung (`body`) hiển thị văn bản thuần, nguồn (`RequestSourceBadge`), người báo cáo, size, urgency. Không `dangerouslySetInnerHTML`.

## Kiểm thử

- Component: skeleton, `not_found`, `network`; nút đúng theo từng trạng thái (bảng test); trả về backlog không gửi khi lý do rỗng; hủy; mở lại; banner backlog; tab Phân tích ẩn với `task`/`docs`/`ops_request`.
- Hook mock `useRequestActions`: gọi đúng kênh và tham số (assert `request.returnToBacklog` nhận `{id,stage,reason}`).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/RequestDetail`.

## Tiêu chí hoàn thành

- [ ] Mọi trạng thái rỗng/tải/lỗi có UI; không lỗi đỏ cho `unsupported`.
- [ ] Phím `Mod+Enter` (`metaKey` Mac, `ctrlKey` nơi khác; nhãn qua `ShortcutKeyCombo`) trong hộp lý do.
- [ ] Khoá i18n `RequestDetailHeader.*`, `ReturnToBacklogDialog.*`, `CancelRequestDialog.*`, `RequestBacklogBanner.*` đủ 5 locale.
- [ ] Không bấm đôi gửi hai lần (nút khoá khi đang gửi).

## Rủi ro và lưu ý

- Có thể `request.get` thiếu trường `returnedFromStage`/`returnReason`: banner chịu thiếu.
- Chưa rõ `executing` có cho trả về backlog hay không (CR-REQ-006): để backend từ chối bằng `REQUEST_TRANSITION_NOT_ALLOWED` và hiển thị.
