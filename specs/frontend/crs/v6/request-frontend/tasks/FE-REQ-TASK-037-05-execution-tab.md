# FE-REQ-TASK-037-05: Tab Execution: tóm tắt, "Needs you", bước và task, thẻ lỗi, Handover

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.4, CR-037 mục 2.6
**Priority:** P1
**Area:** frontend / request / execution
**File:** `frontend/src/renderer/src/components/request/execution/{ExecutionTab,ExecutionSummaryStrip,ExecutionNeedsYou,ExecutionStepList,ExecutionTaskRow,ExecutionFailureCard,HandoverRow}.tsx` và `execution-task-state.ts` (mới), test cùng tên, `components/request/RequestDetailPane.tsx` (sửa nối tab), `i18n/locales/*.json` (khoá `ExecutionTab.*`)
**Depends on:** FE-REQ-TASK-037-02, 037-03; FE-REQ-TASK-021-02 (`usePlanTree`), 036-03 (`ClarificationPanel`), 036-07 (`PlanDriftReviewSheet`); backend BE-REQ-TASK-038-06
**Status:** [~] PARTIAL

---

## Context

- Dữ liệu: `useExecutionSummary` (lần thử, nguyên nhân, `needsYou`, `handover`), `usePlanTree` (cây Phase/task), `TaskAgentActivityPanel` (`components/task/agent-activity/`) cho nhật ký agent, `ExecutionResultPanel` cho kết quả.
- Task không có `failed` trong `TaskStatus`; `Failed` là trạng thái hiển thị suy ra từ `summary.failed[]` (F4). `max_attempts` từ `summary`.
- Chỉ có "Stop run" (huỷ Request hiện có, có xác nhận); không có Pause (F12).
- `hotfix` không có tab Plan: thẻ duyệt `pre_deploy` bắt đầu nằm trong "Needs you" của tab này (F8).
- Engine `openspec`: có Verify và hàng Handover; `native`: không có (CR-037 2.6).

## Việc cần làm

1. `execution-task-state.ts`: `deriveTaskDisplayState(task, summary)` trả `queued|waiting|running|done|failed|blocked` kèm dữ liệu (`attempt`, `maxAttempts`, `cause`, `lastError`, `waitsOn`). Thuần TS, test bảng.
2. `ExecutionTab({request})`: trước `executing` hiện "Not started. Waiting for plan approval" kèm liên kết sang Plan. Khi `executing`/`completed`/`request_backlog`, ghép `ExecutionSummaryStrip`, `ExecutionNeedsYou` (nếu có), `ExecutionStepList`, chân theo engine, `ExecutionFailureCard` khi `request_backlog` do task lỗi.
3. `ExecutionSummaryStrip`: thanh tiến độ (`Progress`), "27/28 tasks · step 7/7", thời gian đã chạy, nhãn trạng thái (Running / Needs you / Stopped / Done) có icon; nút "Stop run" mở hộp xác nhận rồi `actions.cancel`.
4. `ExecutionNeedsYou`: thẻ theo `needsYou[]`: câu hỏi agent thì "Answer" (mở `ClarificationPanel`/cuộn tới); `pre_deploy`/phase thì "Approve/Reject" (`PlanDecisionButtons`, `ApprovalRiskGateSection`); `drift` thì "Review" (`PlanDriftReviewSheet`); task lỗi làm Request về backlog thì "Reopen and rerun" (`actions.reopen`), "View last log", "Cancel request". Có `due_at` thì "N days left".
5. `ExecutionStepList` và `ExecutionTaskRow`: bước đã xong thu gọn một dòng; bước hiện tại mở sẵn; bước sau chỉ đếm; không Phase thì danh sách phẳng. Hàng task theo bảng trạng thái CR-037 2.6. Task `running`: mở `TaskAgentActivityPanel` (thu gọn, ~5 dòng cuối) và "Open full log" (mở `TaskDetail`). Task `failed`: nguyên nhân, dòng lỗi cuối, "Rerun" (`ExecuteTask` hiện có qua `TaskDetail`), "Log", "Attempts". Task OpenSpec `done`: "Verify passed (exit 0)" khi `summary` có dữ liệu.
6. `HandoverRow` (OpenSpec) mô tả archive và bàn giao, hiển thị `handover.state` và liên kết tới worktree task, nhánh proposal, đề xuất tri thức khi `ready`; chân native "Done when the last task is done".
7. `ExecutionFailureCard` (thay `RequestBacklogBanner` dài một câu cho lỗi task): "Stopped: `<task>` failed n/max times", nguyên nhân có cấu trúc, "Everything before it is done and kept", hành động như bước 4.
8. Nối vào `RequestDetailPane`; làm mới theo sự kiện và poll 5 giây khi `executing` và tab mở.

## Kiểm thử

- `deriveTaskDisplayState` (6 trạng thái, task lỗi 1/2 và 2/2, `waiting` theo phụ thuộc).
- Component: mỗi thẻ `Needs you`; `ExecutionTaskRow` 6 trạng thái; `ExecutionFailureCard`; `HandoverRow` OpenSpec và native; "Stop run" có xác nhận.
- #1242: `failed[0]` hiển thị "Failed 2 of 2", không còn "Open".
- Runtime cũ (không có kênh): tab ẩn, không lỗi đỏ.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/request/execution` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Tóm tắt, Needs you, bước/task, thẻ lỗi, Handover hiển thị đúng theo bảng.
- [x] Task lỗi có số lần thử, nguyên nhân và hành động.
- [x] Task đang chạy hiện nhật ký agent trực tiếp.
- [~] OpenSpec có Verify/Handover, native không có (native đã xem trên e2e thật; OpenSpec Handover đã xem trên dữ liệu thật và test, chưa chạy một run OpenSpec sống).
- [x] Không có nút Pause; "Stop run" có xác nhận.
- [x] i18n năm locale; không màu hex.

## Rủi ro và lưu ý

- `TaskAgentActivityPanel` có thể nặng khi nhiều task chạy song song (`REQUEST_MAX_PARALLEL_TASKS`); mở sẵn tối đa 2 task.
- `ExecuteTask` từ hàng task đi qua `TaskDetail`; xác nhận `TaskDetail` mở được trong `Sheet` ngoài `TaskPage` (README frontend mục 3, điểm 14).
- Dòng lỗi cuối của agent có thể chứa dữ liệu nhạy cảm; hiển thị dạng văn bản, cắt độ dài, dùng cùng cơ chế che bí mật của `ExecutionResultPanel` (`redacted`).

## Ghi chú triển khai

Hoàn thành 2026-10-16; đã xem và thao tác trên giao diện web thật với dữ liệu thật (#1242) và qua e2e thật (#1252).

- `execution/`: `ExecutionTab`, `ExecutionSummaryStrip` (thanh tiến độ, "27/28 tasks · step 7/7", thời gian chạy, nhãn Running / Needs you / Stopped / Done có icon; chỉ có "Stop run", mở xác nhận huỷ của pane, không có Pause), `ExecutionNeedsYou` (câu hỏi của agent → "Answer"; Approval `pre_deploy`/step → Approve/Reject có `ApprovalRiskGateSection`; drift → banner `PlanDriftSection`; task hết lượt → "View last log"), `ExecutionStepList` (bước xong thu một dòng, bước hiện tại mở, bước sau chỉ đếm, không Phase thì danh sách phẳng; tối đa 2 nhật ký agent chạy trực tiếp), `ExecutionTaskRow` (6 trạng thái queued/waiting/running/done/failed/blocked, "Failed 2 of 2", nguyên nhân, lời khuyên theo nguyên nhân), `ExecutionFailureCard` (thay banner dài: "Stopped: … failed 2/2 times", "Everything before it is done and kept", "Reopen and rerun" / "View last log" / "Cancel request"), `HandoverRow` (OpenSpec: archive/bàn giao; native: "Done when the last task is done"), `ExecutionTaskSheet` (mở `TaskDetail` trong Sheet; đã xác nhận chạy được ngoài `TaskPage`).
- `execution-task-state.ts` (`deriveTaskDisplayState`, test bảng), `execution-step-model.ts`, `execution-failure-cause.ts`. `Failed` suy ra từ `summary.failed[]`, không thêm vào `TaskStatus` (F4). Dòng lỗi cuối là văn bản thuần, cắt 240 ký tự.
- Trên #1242 thật: bước 1–6 thu gọn "n/n tasks done", bước 7 mở với #TG-1037 "Failed 2 of 2", "Timed out", "execution failed: retryable TIMED_OUT", "The agent ran out of time. Rerunning usually helps; if it repeats, split the task." và hàng "Details/Log" cho các task xong; thẻ lỗi có ba nút. Mở "View last log" hiện `TaskDetail` với nhật ký agent đã kết thúc. Trên #1252 (native, e2e thật): "Done 2/2 tasks · step 2/2", hai bước thu gọn, chân "Done when the last task is done." (không có Handover).
- Chưa kiểm trên giao diện: nhấp "Reopen and rerun" trên #1242 (sẽ chạy lại agent thật trên repo pilot, tốn tiền) và hàng OpenSpec Verify/Handover đang chạy (xem 037-09); phần này chỉ có test đơn vị (`execution-run-components.test.tsx`, 16 test cho strip, handover, thẻ lỗi, step list).
- **Chưa làm:** dòng "Verify passed (exit 0)" của task OpenSpec đã xong (bước 5 của spec). `ExecutionSummary` hiện không mang kết quả Verify của từng task (chỉ `checks_run` nằm trong kết quả thực thi), nên cần thêm trường ở backend hoặc đọc `execution.get` theo từng task; ghi lại như một việc tiếp theo.
- Runtime không có kênh `request.executionSummary`: `RequestDetailPane` ẩn tab, không lỗi đỏ.
