# FE-REQ-TASK-037-03: Thang tiến độ theo luồng và tab hiển thị theo luồng

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.3, CR-037 mục 2.3 và 2.4
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/{request-stage-timeline-model,RequestStageTimeline,request-action-rules,RequestDetailPane}.ts(x)` (sửa), `request-stage-timeline-model.test.ts`, `request-action-rules.test.ts`, `RequestDetailPane.test.tsx` (sửa), `i18n/locales/*.json` (khoá `StageTimeline.step.handover`, `.taskList`, `.state.waiting`, `.state.running`)
**Depends on:** FE-REQ-TASK-037-02
**Status:** [x] DONE

---

## Context

- `buildStageTimeline` hiện luôn thêm bước `execution`, `STATUS_TO_CURRENT_STEP` không có trạng thái nào trỏ tới `phase`, `request_backlog` và `cancelled` hiện bước `current`/`skipped`. `RequestStageTimeline` có bốn trạng thái `done|current|pending|skipped`.
- Luồng từ `RequestFlowView` (task 02) và `engine` từ `OrcaRequest`.
- Ánh xạ chặng và trạng thái: CR-037 mục 2.3.
- `getVisibleDetailTabs(type)` đọc bảng tĩnh và trả `{analysis, plan}`.

## Việc cần làm

1. Viết lại `buildStageTimeline({flow, engine, status, returnedFromStage, resumeStatus, summary})` trả `{steps: Step[], current}` với `Step = {id: 'classification'|'analysis'|'plan'|'execution'|'handover', labelKey, state: 'done'|'running'|'waiting'|'stopped'|'pending', sub?}`.
   - Chặng: Classification; Analysis nếu `analysisKind != none`; Plan (nhãn `plan` hoặc `taskList`) nếu `planKind` là `plan`/`task_list`; Execution nếu `planKind != none`; Handover nếu `engine=openspec`.
   - Ánh xạ: `classifying` chạy; `awaiting_type_confirmation` chờ người; `analyzing` chạy; `awaiting_analysis_approval` chờ người; `planning` chạy; `awaiting_plan_approval` chờ người ở Plan (hoặc Execution khi `single_task`); `executing` chạy, chờ người nếu `summary.needsYou` không rỗng; `completed` mọi chặng xong (Handover "đang xử lý" khi `summary.handover.state=archiving`); `awaiting_information` theo `resumeStatus`; `request_backlog`/`cancelled` đặt chặng của `returnedFromStage` (`phase` và `task` gộp vào Execution) ở `stopped`, các chặng trước `done`, sau `pending`.
   - `sub`: "step 3/7 · 9/28 tasks" (từ `summary.progress`), "waiting for approval", "stopped".
2. `RequestStageTimeline`: icon + chữ cho từng trạng thái (`Check`, `Loader2` quay có `aria-hidden`, `Hand`/`CircleHelp`, `MinusCircle`, `Circle`), tông `primary` cho chạy và chờ, `status-success` cho xong, `destructive` cho `stopped` do lỗi, `muted-foreground` cho chờ chưa tới; `sr-only` cho trạng thái; `aria-current="step"` ở chặng hiện tại.
3. `getVisibleDetailTabs(flow)` trả `{analysis, plan, execution}`: `analysis` khi `analysisKind != none`; `plan` khi `planKind` là `plan`/`task_list`; `execution` khi `planKind != none` (kể cả `single_task`). Truyền `flow` từ `RequestDetailPane` qua `useRequestFlow`.
4. `RequestDetailPane`: thêm tab "Execution" (nội dung do task 05), giữ thứ tự Overview, Analysis, Plan hoặc Task list, Execution, History, Related; nhãn tab Plan là "Task list" khi `planKind=task_list`.
5. Không rẽ nhánh theo `type`; `question`/`spike` kết thúc ở Analysis chỉ nhờ `planKind=none`.

## Kiểm thử

- Bảng 11 loại × trạng thái chính (`classifying`, `analyzing`, `awaiting_analysis_approval`, `planning`, `awaiting_plan_approval`, `executing`, `completed`, `request_backlog` với từng `returnedFromStage`, `cancelled`, `awaiting_information` với `resumeStatus`) × `engine` (`native`, `openspec`).
- `question`/`spike`: không có Execution; `hotfix`: không có Plan, Execution chờ khi `awaiting_plan_approval`; chỉ OpenSpec có Handover.
- #1242 (`change_request`, `request_backlog`, `returnedFromStage=task`): Execution `stopped`, Plan `done`, không có "Phases".
- `getVisibleDetailTabs` cho 11 loại.
- `RequestDetailPane` với runtime cũ (không có `request.flow`) dùng bảng dự phòng, không lỗi.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/request/request-stage-timeline src/renderer/src/components/request/request-action-rules src/renderer/src/components/request/RequestDetailPane` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Không còn chặng "Phases" và không còn Execution cho `question`/`spike`.
- [x] Request về backlog hiển thị chặng bị dừng, không hiện "current".
- [x] Tab hiển thị đúng theo luồng cho 11 loại.
- [x] Mỗi trạng thái chặng có icon và chữ, có văn bản `sr-only`.
- [x] i18n năm locale cho khoá mới.

## Rủi ro và lưu ý

- "Chạy" và "chờ người" cùng tông `primary`; phân biệt chỉ bằng icon và chữ (F9), cần kiểm trực quan chế độ tối.
- Chặng Handover "đang xử lý" phụ thuộc `handover.state` (câu hỏi mở 1); không có dữ liệu thì chỉ hiện `pending`/`done`.
- Các test cũ của `buildStageTimeline` sẽ đổi (tên bước, `skipped`); cập nhật, không xoá độ phủ.

## Ghi chú triển khai

Hoàn thành 2026-10-16; đã xem trên giao diện web thật (localhost:28080) với dữ liệu thật.

- `request-stage-timeline-model.ts` viết lại: chặng theo luồng (`classification`, `analysis`, `plan`, `execution`, `handover`); không còn chặng "Phases"; `question`/`spike` kết thúc ở Analysis; `hotfix` chờ `pre_deploy` giữ chặng Execution; Handover chỉ khi engine `openspec`. Trạng thái `done|running|waiting|stopped|pending`, mỗi trạng thái có icon và chữ (`sr-only`), "đang chạy" và "chờ bạn" cùng tông primary (không thêm token màu) nhưng khác icon và chữ. Chú thích dưới chặng ("step 3/7 · 9/28 tasks", "waiting for approval", "27/28 tasks · stopped").
- Request về backlog hiển thị chặng bị dừng bằng `returnedFromStage` (#1242: Execution `stopped`, "27/28 tasks · stopped"). Request đã huỷ mà không có chặng ghi lại thì không đánh dấu chặng nào là dừng (đánh dấu sẽ là đoán; phát hiện khi xem #1229, #1206 trên giao diện thật).
- `getVisibleDetailTabs(flow)` trả `{analysis, plan, execution}`; kiểm trên dữ liệu thật: change_request đủ bốn tab, task `Plan`+`Execution`, hotfix `Analysis`+`Execution` (không có Plan), question chỉ `Analysis`.
- `RequestDetailPane` nối `useRequestFlow` và `useExecutionSummary`; tab Execution ẩn khi runtime không có kênh. Test: `request-stage-timeline-model.test.ts` (11 loại × trạng thái × engine), `request-action-rules.test.ts`.
- 191 khoá i18n mới trong năm locale (es/ja/ko/zh do tôi dịch, cần người bản ngữ rà).
