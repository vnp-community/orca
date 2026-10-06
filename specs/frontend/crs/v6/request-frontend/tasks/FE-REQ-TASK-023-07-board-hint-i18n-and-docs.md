# FE-REQ-TASK-023-07: Gợi ý trên Board, i18n năm locale, test phủ khoá và tài liệu trang Backlog

**From Solution:** [FE-REQ-SOL-023](../solutions/FE-REQ-SOL-023-backlog-screens.md) mục 2.8, 2.9
**Priority:** P1
**Area:** frontend (components/task, i18n, docs)
**File:** `frontend/src/renderer/src/components/task/TaskBoardView.tsx` (sửa), `components/task/__tests__/TaskBoardView.test.tsx` (sửa), `i18n/locales/{en,es,ja,ko,zh}.json` (sửa), `i18n/request-backlog-locale-coverage.test.ts` (mới), `docs/ui/pages/requests.md` (thêm mục Backlog)
**Depends on:** FE-REQ-TASK-023-04, 023-05, 023-06 (danh sách khoá thật); FE-REQ-SOL-018 (đã gỡ `backlog` khỏi `STATUS_ORDER`, có `requestFlowSupport`)
**Status:** [ ] TODO

## Context

- `TaskBoardView.tsx` có `STATUS_ORDER: TaskStatus[]` (dòng 9) và render cột bằng `STATUS_ORDER.map` (dòng 74); SOL-018 gỡ `'backlog'`. Test hiện có render `<TaskBoardView tasks={tasks} onSelect={onSelect} />` (`__tests__/TaskBoardView.test.tsx`).
- Hiển thị gợi ý khi `useAppStore((s) => s.requestFlowSupport) === 'supported'` (slice SOL-018). Không tạo status mới, không thêm cột.
- Khoá đọc theo tên không được công cụ sinh vào catalog: bắt buộc có test phủ khoá (mẫu `i18n/task-jira-link-locale-coverage.test.ts`).
- `openRequestPage({section:'backlog'})` thuộc slice SOL-018.

## Việc cần làm

1. `TaskBoardView.tsx`: thêm dòng gợi ý nhỏ phía trên các cột (`text-muted-foreground`, `text-xs`) với `translate('auto.components.task.TaskBoardView.backlogMoved', 'Tasks not started yet live under Requests > Backlog')` và nút liên kết (`variant="link"`) gọi `openRequestPage({section:'backlog'})`. Chỉ hiện khi `requestFlowSupport==='supported'`. Không đụng `STATUS_ORDER` (SOL-018 đã làm).
2. Cập nhật `TaskBoardView.test.tsx`: không còn cột `backlog`; gợi ý hiện khi `supported`, ẩn khi `unsupported` hoặc `unknown`; bấm nút gọi `openRequestPage` với `{section:'backlog'}` (mock store).
3. Chốt danh sách khoá từ code task 03 đến 06 (tiền tố `auto.components.request.backlog.`): `BacklogSegmentControl.{request,task,execute}`, `BacklogToolbar.{search,type,category,refresh}`, `RequestBacklogTable.col.*` (9 cột) và `RequestBacklogTable.actorSystem`, `TaskBacklogTable.col.*`, `ExecuteBacklogTable.col.*` và `ExecuteBacklogTable.noErrorDetail`, `ReturnedFromStage.{classification,analysis,plan,phase,task}`, `ReturnedCategory.{missing_info,infeasible,blocked_dependency,rejected,other}`, `GateStatus.{approved,pending,rejected,none}`, `ReopenRequestDialog.{title,body,confirm,viewRequest}`, `CancelRequestDialog.{title,body,reason,confirm}`, `BacklogEmptyState.{request,task,execute,filtered}`, `BacklogErrorState.{network,forbidden,retry}`, `TaskBacklogRow.planNotSplit`, `BacklogRow.{reopened,alreadyHandled}`; cộng `auto.components.task.TaskBoardView.backlogMoved`.
4. Thêm mọi khoá vào đủ 5 locale (JSON lồng nhau). Chuỗi gốc tiếng Anh theo CR-023 mục 2.5 ("No Request has been returned", "Every task already has an approved Plan", "No task is waiting to run or failed"...). Tham số nội suy kiểu `{{number}}`, `{{stage}}`.
5. `request-backlog-locale-coverage.test.ts`: mảng `KEYS`, với mỗi locale `en, es, ja, ko, zh` khẳng định chuỗi không rỗng; chuỗi của locale khác `en` không được trùng bản `en` (trừ ngoại lệ ghi rõ).
6. `docs/ui/pages/requests.md`: thêm mục "Backlog": ba phân đoạn, kênh `backlog.requests|tasks|execute`, cột, hành động ghi chỉ ở Request backlog, phím `1/2/3`, `j/k/Enter`, trạng thái, liên hệ cột Board đã gỡ. Nếu SOL-018 chưa tạo file, báo người điều phối thay vì tạo trùng. Rà `docs/ui/pages/tasks.md` và `docs/ui/page-tree.md` xem còn nhắc cột `backlog` không.

## Kiểm thử

- Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/task/__tests__/TaskBoardView.test.tsx frontend/src/renderer/src/i18n/request-backlog-locale-coverage.test.ts`.
- Thử `pnpm verify:localization-catalog` và `pnpm verify:localization-coverage` ở gốc: các script này trỏ `config/scripts/...` không thấy trong repo hiện tại, ghi nhận kết quả thật.
- Thủ công: đổi ngôn ngữ UI sang `ja`, `zh`, kiểm các tiêu đề cột và hộp thoại.

## Tiêu chí hoàn thành

- [ ] Board có dòng gợi ý khi có Request flow, không có khi không hỗ trợ; không còn cột `backlog`.
- [ ] Mọi chuỗi mới của màn Backlog có đủ 5 locale; test phủ khoá xanh; không khoá mồ côi.
- [ ] `docs/ui/pages/requests.md` có mục Backlog.
- [ ] `TaskBoardView.test.tsx` xanh với dòng gợi ý.

## Rủi ro và lưu ý

- Dịch máy cần người bản ngữ rà (ghi rõ trong PR).
- Khoá `TaskBoardView.backlogMoved` thêm vào component cũ; chạy `rg "'backlog'" frontend/src/renderer/src/components/task` để chắc chắn không còn tham chiếu trạng thái `backlog` (ngoài test chuẩn hoá của SOL-018).
- Người dùng quen kéo task vào cột `backlog` sẽ bối rối; dòng gợi ý chỉ giảm nhẹ (CR-023 mục 6).
