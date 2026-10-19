# FE-REQ-TASK-037-04: Tab Plan: một khối duyệt, đề cương chỉ đọc, giữ chế độ `step_by_step`

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.4, CR-037 mục 2.5 và 2.9
**Priority:** P1
**Area:** frontend / request / plan
**File:** `frontend/src/renderer/src/components/request/plan/{RequestPlanTab,PlanApprovalBar,PlanOutline,plan-view-state,PlanApprovalCard}.tsx` (sửa/mới), `PlanTree.tsx` (sửa nhỏ), test cùng tên, `i18n/locales/*.json` (khoá `PlanApprovalCard.*`, `PlanOutline.*`)
**Depends on:** FE-REQ-TASK-037-02, 037-03, FE-REQ-TASK-021-03, 021-04, 021-05, 036-06
**Status:** [x] DONE

---

## Context

- `RequestPlanTab` hiện hiển thị `PlanApprovalBar`, `PlanTree` với `PhaseApprovalBar` (qua `renderPhaseActions`), `PlanGateChips`, `PlanDriftSection`; `PlanTaskRow` có badge trạng thái, nút mở `TaskDetail` để chạy.
- Backend: `executionMode=continuous` thì không có Approval `phase`; cổng duyệt duy nhất theo `startGate` (`plan`, `task_list`, `pre_deploy`). Rủi ro mọi Phase thu ở Approval Plan (CR-038 B8). `hotfix` không có tab Plan (thẻ duyệt `pre_deploy` ở tab Execution, task 05).
- `executionMode=step_by_step` (mặc định) giữ nguyên giao diện CR-REQ-021.
- Không thêm phím tắt cho Duyệt; từ chối cần lý do ≥ 10 ký tự (`RejectReasonDialog`).

## Việc cần làm

1. `PlanApprovalCard` (khi có Approval `pending` của `startGate` và `executionMode=continuous`): tiêu đề "Approve once and the agent runs all N steps/tasks", đoạn "It stops and asks you only when:" (cần thông tin; task lỗi hai lần thì Request về backlog; có cổng pre-deploy nếu `executionGates` hoặc `startGate` có), nút chính "Approve and start execution", phụ "Request changes" (từ chối kèm lý do) và "Regenerate". Dùng `usePlanDecision`, `ApprovalRiskGateSection`/`useApprovalRiskGate` hiện có và quy tắc "chưa có Decision thì khoá Duyệt" (036-04).
2. Danh sách rủi ro mọi Phase: liệt kê phát hiện theo Phase (ghi rõ tên Phase) và yêu cầu chấp nhận trước khi nút Approve bật (`acceptedFindingIds` gồm mọi Phase). Nếu backend chưa trả theo Phase, hiện cảnh báo "Risk is shown for the plan as a whole" và vẫn cho duyệt như hiện nay (ghi `unsupported` vào PR).
3. `PlanOutline` (chỉ đọc): nếu `flow.hasPhases` thì danh sách bước (tên, số task, tổng `estimatedHours` nhãn "estimated", mở rộng thấy tên task); nếu không thì danh sách task phẳng theo thứ tự. Không badge trạng thái task, không nút Run, không "Approve phase", "Start phase", `PlanGateChips`.
4. Sau khi duyệt: `PlanOutline` có dấu tích theo `summary.progress`, dải "Plan approved by `<tên>`, `<giờ tương đối>`. Follow in Execution" với nút chuyển tab (state `tab` của `RequestDetailPane`). Giữ `PlanDriftSection`, `ReadinessBadge`.
5. `step_by_step`: render nhánh cũ nguyên vẹn (`PlanTree` + `PhaseApprovalBar` + `PlanGateChips`); chọn nhánh theo `request.executionMode` (rỗng coi như `step_by_step`).
6. `plan-view-state.ts`: thêm trạng thái hiển thị tổng "awaiting_approval | approved_readonly | step_by_step" nếu giúp test; giữ `derivePlanViewState` cho các trạng thái rỗng/tải/lỗi.
7. Chạy `gitnexus_impact` cho `RequestPlanTab`, `PlanApprovalBar` trước khi sửa.

## Kiểm thử

- `continuous`: thẻ duyệt hiện đúng theo `startGate` (`plan`, `task_list`, `pre_deploy` cho `security`); không có nút phase; từ chối cần lý do ≥ 10 ký tự.
- Rủi ro theo Phase khoá Approve tới khi chấp nhận hết; gửi `acceptedFindingIds` đầy đủ.
- Sau duyệt: chỉ đọc, có liên kết sang Execution.
- `step_by_step`: toàn bộ test CR-021 hiện có vẫn xanh.
- Cỡ M không Phase: danh sách phẳng; cỡ L có Phase: theo bước.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/request/plan` (chưa chạy).

## Tiêu chí hoàn thành

- [x] `continuous`: một lần duyệt, không có "Approve phase"/"Start phase"/chip cổng ở tab Plan.
- [x] Đề cương theo bước hoặc phẳng đúng theo `hasPhases`.
- [x] Sau duyệt Plan chỉ đọc kèm liên kết Execution.
- [x] `step_by_step` không đổi hành vi.
- [x] i18n năm locale.

## Rủi ro và lưu ý

- Người duyệt phải thấy rủi ro mọi Phase trước khi duyệt một lần; nếu backend chưa trả theo Phase thì đây là điểm mất kiểm soát, cần cảnh báo rõ.
- `estimatedHours` có thể thiếu hoặc do AI ước; luôn nhãn "estimated".
- Plan dài (hàng chục task): đề cương mặc định thu gọn trừ bước đầu.

## Ghi chú triển khai

Hoàn thành 2026-10-16; nghiệm thu bằng e2e thật qua giao diện (xem 037-09).

- `plan/PlanApprovalCard.tsx`: khi `request.executionMode === 'continuous'` và có Approval `plan`/`task_list` đang chờ, thẻ nói "Approve once and the agent runs all N steps/tasks", liệt kê khi nào nó dừng và hỏi (cần thông tin, task lỗi thì về backlog, drift, `pre_deploy` nếu luồng có), nút "Approve and start execution" / "Request changes" (từ chối cần lý do ≥ 10 ký tự qua `RejectReasonDialog`), cùng "Regenerate". Dùng lại `PlanApprovalBar` (thêm hai prop nhãn) nên giữ nguyên `ApprovalRiskGateSection`, `useApprovalRiskGate` và quy tắc khoá Duyệt khi chưa có Decision.
- `plan/PlanOutline.tsx`: đề cương chỉ đọc; theo bước (tên, số task, tổng `estimatedHours` ghi "estimated", bước đầu mở sẵn) khi `flow.hasPhases`, danh sách phẳng khi không; không badge trạng thái, không nút Run, không "Approve phase"/"Start phase"/`PlanGateChips`. Dấu tích theo task đã xong.
- `RequestPlanTab`: sau khi duyệt hiện dải "Plan approved … The agent runs it on its own" với nút "Follow in Execution"; `step_by_step` (và `executionMode` rỗng) vẽ nhánh cũ nguyên vẹn (`PlanTree` + `PhaseApprovalBar` + `PlanGateChips`), toàn bộ test CR-021 vẫn xanh. Nhánh theo `flow` của backend khi có.
- **Điểm bổ sung phía backend (task 038-04):** chế độ chỉ được ghim lúc duyệt Plan, nên trước đó `request.executionMode` rỗng và thẻ không biết mình thuộc chế độ nào; `engine`/`executionMode` giờ được điền theo cài đặt dự án cho Request chưa ghim.
- Lỗi mới `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` có câu giải thích riêng ở `plan-decision-error-message.ts`.
- Rủi ro theo từng Phase: backend chưa trả theo Phase, nên thẻ hiện cảnh báo "Risk is assessed for the plan as a whole, including every step" (đúng như spec cho trường hợp này) và vẫn cho duyệt. Ghi nhận là `unsupported` phía backend.
- Test: `PlanApprovalCard.test.tsx` (4), `PlanOutline.test.tsx` (2), `plan-decision-error-message.test.ts`, toàn bộ `plan/` 57 test xanh.
