# FE-REQ-TASK-018-04: Store slice `request`, view cấp cao `requests` và điều hướng

**From Solution:** [FE-REQ-SOL-018](../solutions/FE-REQ-SOL-018-request-frontend-foundation.md) mục 2.5
**Priority:** P0
**Area:** frontend / store
**File:** `frontend/src/renderer/src/store/slices/request.ts` (mới), `store/index.ts` (sửa, gần dòng 48 và 117), `store/types.ts` (sửa), `store/slices/ui.ts` (sửa: dòng 507, 632, 1336, 1348, 1540), `frontend/src/shared/types.ts` (sửa dòng 3360); test `store/slices/request.test.ts`, mở rộng test `ui` hiện có
**Depends on:** FE-REQ-TASK-018-01
**Status:** [x] DONE

## Context

- `TopLevelView` (`shared/types.ts:3360`) là union; `TOP_LEVEL_VIEW_LOOKUP: Record<TopLevelView,true>` (`ui.ts:507`) bắt buộc đủ khoá. `sanitizeHydratedActiveView` (`ui.ts:526`) rơi về `terminal` với giá trị lạ.
- Mẫu overlay: `previousViewBeforeTasks` (`ui.ts:632,1336`), `openTaskPage` (`ui.ts:1348`) và đóng ở `ui.ts:1540`.
- `slices/task.ts` ghi chú: action trả object một phần, không mutate (tránh thay toàn state).
- Chạy chạm `ui.ts` (rất lớn, thuộc baseline max-lines): chỉ thêm ít dòng, không thêm `max-lines` disable.

## Việc cần làm

1. `shared/types.ts`: thêm `| 'requests'` vào `TopLevelView`.
2. `ui.ts`: thêm `requests: true` vào `TOP_LEVEL_VIEW_LOOKUP`; thêm `previousViewBeforeRequests: TopLevelView` (mặc định `'terminal'`); thêm `openRequestPage(data?: Partial<RequestPageData>)` (đặt `activeView='requests'`, giữ `previousViewBeforeRequests` như `openTaskPage` chống ghi đè khi đã ở `requests`, gọi `setRequestPageData`) và `closeRequestPage()` (về `previousViewBeforeRequests`).
3. `sanitizeHydratedActiveView`: nhận thêm cờ hỗ trợ; khi `'requests'` mà `requestFlowSupport !== 'supported'` thì `'terminal'` (đọc store tại thời điểm hydrate: tới lúc đó cờ là `'unknown'` nên luôn `'terminal'`; chấp nhận, ghi comment "why").
4. `slices/request.ts`: state `requestFlowSupport`, `requestsById`, `pendingApprovalCount`, `requestPage {section:'requests'|'approvals'|'backlog', requestId:string|null, backlogView:BacklogView, listFilters}`; action `upsertRequests`, `removeRequest`, `setPendingApprovalCount`, `setRequestFlowSupport`, `setRequestPageData`, `setRequestPageSection`, `setRequestPageRequest`. Đều trả object một phần.
5. Đăng ký `createRequestSlice` ở `store/index.ts` và kiểu ở `store/types.ts` như `createTaskSlice`.
6. Không lưu danh sách Solution/Approval ở store.

## Kiểm thử

- `request.test.ts`: `upsertRequests` hợp nhất theo `id` không mất khoá khác; `removeRequest`; `setRequestPageSection` không reset `requestId`; `setPendingApprovalCount` chặn số âm.
- `ui` test: `openRequestPage()` đặt `activeView='requests'`, `closeRequestPage()` về view trước; mở hai lần liên tiếp không ghi đè `previousViewBeforeRequests` bằng `'requests'`; hydrate `'requests'` thì `'terminal'` khi `unknown`.
- Bảo đảm biên dịch: không có chỗ nào `switch (activeView)` thiếu nhánh (`tsc`; `lint:switch-exhaustiveness` ở `package.json` gốc, chưa kiểm chứng).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/store/slices/request src/renderer/src/store/slices/ui`.

## Tiêu chí hoàn thành

- [ ] `activeView='requests'` được đặt/đóng đúng; persist khôi phục an toàn.
- [ ] Không còn lỗi biên dịch về `TOP_LEVEL_VIEW_LOOKUP` hoặc `switch` thiếu nhánh.
- [ ] `ui.ts` chỉ thêm dòng cần thiết; không có `max-lines` disable mới.
- [ ] Test xanh.

## Rủi ro và lưu ý

- Chạy `impact` (GitNexus) cho `openTaskPage`, `sanitizeHydratedActiveView` trước khi sửa theo CLAUDE.md; ghi kết quả vào PR (chưa chạy).
- Rà `App.tsx` và các `switch` theo `activeView` (ví dụ ẩn sidebar theo view) để view mới không vô tình ẩn/hiện sai.
