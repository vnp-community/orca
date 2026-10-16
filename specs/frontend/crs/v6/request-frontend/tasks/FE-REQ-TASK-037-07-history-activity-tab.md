# FE-REQ-TASK-037-07: Tab History thành nhật ký hoạt động

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.4, CR-037 mục 2.8
**Priority:** P1
**Area:** frontend / request / activity
**File:** `frontend/src/renderer/src/components/request/activity/{RequestActivityTab,ActivityList,ActivityRow,ActivityFilterBar,activity-grouping}.ts(x)` (mới), `RequestHistoryTab.tsx` (sửa, giữ làm dự phòng), `RequestDetailPane.tsx` (sửa), test cùng tên, `i18n/locales/*.json` (khoá `ActivityTab.*`)
**Depends on:** FE-REQ-TASK-037-02; backend BE-REQ-TASK-038-05, 07
**Status:** [x] DONE

---

## Context

- `RequestHistoryTab` chỉ hiển thị `request.typeHistory` (một dòng trên #1242). Dữ liệu đầy đủ do `request.activity` (BE-SOL-038) trả: `kind`, `severity`, `actorKind`, `occurredAt`, `summary`, `subjectType/Id`, `attempt`, `details`.
- Chỉ sự kiện cấp Request và task; log agent mở riêng (F11).
- Không hiển thị `body` của Request hay log thô trong dòng; chi tiết mở bằng `Sheet` có kiểm quyền.
- Thời gian tương đối dùng `formatRequestRelativeTime` hiện có; hiển thị người làm bằng icon (AI `Bot`, người `User`, hệ thống `Cog`/`Server`, agent `Cpu`) kèm chữ.

## Việc cần làm

1. `RequestActivityTab({request})`: dùng `useRequestActivity`; hiện `ActivityFilterBar` (All, Needs attention, Approvals, Execution, Changes, By me) ánh xạ sang `kinds`/`severity`/`actorKind` theo CR-038 bảng 2.2; danh sách nhóm theo ngày (`activity-grouping.ts`: "Today", "Yesterday", ngày), mới nhất trước, nút "Load more".
2. `ActivityRow`: icon theo `kind`, tiêu đề ngắn (từ `kind` + `summary`), thời điểm tương đối (tooltip thời điểm tuyệt đối), người làm, một dòng chi tiết, liên kết tới đối tượng (task mở `TaskDetail`; approval mở tab liên quan; log mở `TaskDetail` tab Agent activity; phân tích/plan/execution chuyển tab). `severity=error` nổi bật bằng icon và chữ (không chỉ màu); `attention` có icon riêng.
3. Trạng thái: rỗng "No activity yet", tải (skeleton), lỗi (Retry), `unsupported` thì dùng `RequestHistoryTab` cũ (đổi loại) không lỗi đỏ.
4. Làm mới theo `request.status_changed` và sự kiện thực thi khi tab mở; không poll riêng; đổi bộ lọc xoá danh sách và tải lại.
5. `RequestDetailPane`: tab "History" render `RequestActivityTab` khi hỗ trợ, ngược lại `RequestHistoryTab`. Tên tab giữ "History".
6. Truy cập: danh sách `<ol>` có nhãn, mỗi dòng có thể điều hướng bằng bàn phím (liên kết thật), `aria-live` không dùng (không cướp tiêu điểm).

## Kiểm thử

- `activity-grouping` (ranh giới ngày, múi giờ người dùng).
- Bộ lọc ánh xạ đúng tham số; "By me" lọc theo người dùng hiện tại.
- `ActivityRow` các `kind` chính (`task.failed`, `approval.decided`, `request.returned`, `request.type_changed`); không hiển thị `body`.
- `unsupported` rơi về danh sách đổi loại.
- #1242: có dòng AI phân loại, về backlog, task lỗi.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/request/activity src/renderer/src/components/request/RequestHistoryTab` (chưa chạy).

## Tiêu chí hoàn thành

- [x] History hiển thị các `kind` của CR-038, nhóm theo ngày, có bộ lọc và "Load more".
- [x] Dòng lỗi và cần chú ý nổi bật bằng icon và chữ.
- [x] Liên kết tới đúng đối tượng/tab (nút "Open <tab>" theo `activityTargetTab`, 037-09).
- [x] Runtime cũ vẫn thấy danh sách đổi loại.
- [x] i18n năm locale (do 037-09 thêm khoá vào cả năm locale).

## Rủi ro và lưu ý

- `kind` lạ từ backend mới hơn phải hiển thị chung (không crash).
- Request dài có nhiều dòng: phân trang bắt buộc, không tải hết.
- Tên tab giữ "History" (quyết định thiết kế); nếu sau đổi sang "Activity" cần cập nhật khoá i18n và test.

## Ghi chú triển khai

- Thư mục mới `components/request/activity/` (tên file cụ thể thay cho `ActivityList`/`activity-grouping` trong mục File ở trên): `RequestActivityTab.tsx` (props `requestId, active, typeHistory, typeHistoryLoading, typeHistoryError, onRetryTypeHistory`), `ActivityFilterBar.tsx`, `ActivityEntryRow.tsx`, và phần logic thuần `activity-entry-presentation.ts` (kind -> icon, nhóm, tone, tên task, lần chạy), `activity-entry-text.ts` (chữ của từng dòng từ `kind` + `details`; `kind` lạ dùng `summary`), `activity-day-grouping.ts` (nhóm theo ngày theo múi giờ người xem, Today/Yesterday), `activity-filter-params.ts` (trạng thái bộ lọc -> `RequestActivityFilter`).
- Bộ lọc: chip nhóm All / Approvals / Tasks / Clarifications / Type & plan changes (`kinds`), nút bật "Needs attention only" (`minSeverity: 'attention'`), chip người làm Anyone / User / AI / Agent / System (`actorKind`). Nhóm "Type & plan changes" gồm `request.classified`, `request.type_changed`, `analysis.generated`, `request.returned`, `request.reopened`, `request.cancelled`; `request.created` chỉ hiện ở All. Hook tự xoá danh sách và tải lại khi bộ lọc đổi.
- Dòng: icon theo `kind` (đổi hình khi duyệt bị từ chối), tiêu đề, chip mức độ có icon và chữ ("Needs attention" / "Stopped") cho `attention`/`error`, "Attempt N of M" từ `details.maxAttempts`, một dòng chi tiết (nguyên nhân + lỗi, lý do, bình luận; `line-clamp-2`, chỉ văn bản), người làm bằng icon + chữ, `<time>` có tooltip thời điểm tuyệt đối. Màu chỉ dùng token `text-status-success`, `text-destructive`, `text-primary`, `text-muted-foreground`.
- `unsupported` hiển thị `RequestHistoryTab` (không đổi) với dữ liệu đổi loại, không có lỗi đỏ và không có thanh lọc. Khi `active=false` tab không render và hook không gọi.
- Khoá i18n: tiền tố `auto.components.request.activity.`, viết thẳng trong các lời gọi `translate()`; nhãn loại Request tái dùng khoá `RequestStatus`/type sẵn có qua `getRequestTypePresentation`.
- Kiểm thử: `activity-entry-presentation`, `activity-entry-text`, `activity-day-grouping`, `activity-filter-params` (theo bảng) và `RequestActivityTab.test.tsx` (tải, rỗng, dòng theo ngày, `kind` lạ, bộ lọc gọi hook với tham số đúng, Load more, lỗi + Retry, `unsupported`). Đã chạy `npx vitest run --config config/vitest.config.ts src/renderer/src/components/request/activity`: 5 file, 92 test xanh. `tsc` không có lỗi ở file mới.
- Chưa làm (ngoài phạm vi lượt này): nối `RequestActivityTab` vào `RequestDetailPane` (tab History), khoá ở năm locale, liên kết từ dòng tới task/tab (component chưa nhận callback điều hướng) và "By me" theo người dùng hiện tại (bộ lọc người làm hiện theo loại `user`, không theo id).
