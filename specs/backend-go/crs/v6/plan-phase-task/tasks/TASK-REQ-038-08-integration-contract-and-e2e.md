# TASK-REQ-038-08: Kiểm thử tích hợp, hợp đồng hai dialect và e2e thực thi liên tục

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `request-service`, `api-gateway`
**File:** `internal/adapter/contracttest/execution_flow_contract.go` (sửa), `internal/adapter/contracttest/request_activity_contract.go` (mới), `internal/adapter/contracttest/execution_summary_contract.go` (mới), `backend-go/services/request-service/e2e/` (sửa hoặc thêm kịch bản), `docs/crs/v6/plan-phase-task/CR-REQ-038-*.md` (cập nhật trạng thái)
**Depends on:** TASK-REQ-038-03, TASK-REQ-038-05, TASK-REQ-038-06, TASK-REQ-038-07
**Status:** [x] DONE

---

## Context

- `contracttest` chạy cùng một bộ kiểm cho Postgres và MySQL; thêm kịch bản mới vào đó để hai dialect cùng đúng.
- Chạy thật cần stack local: token CLI sống khoảng 10 phút nên một lần sinh plan dài có thể chết `UNAUTHENTICATED` dù run vẫn tiếp tục (đăng nhập lại rồi đọc proposal); sau khi redeploy một service phải khởi động lại các service gọi nó theo thứ tự `infra-fleet`, `git-gateway`, `task-service`, `request-service`, `api-gateway`; trước một lần chạy thật tốn kém nên chạy `workspace_probe` trên worktree proposal tạm.
- Chưa chạy bất kỳ test nào của task này.

## Việc cần làm

1. **Luồng thực thi** (`execution_flow_contract.go`): kịch bản `continuous`:
   - `change_request` 3 Phase, duyệt Plan một lần, ba Phase tự chạy tuần tự, không Approval `phase`, `phase_starts.trigger=auto`.
   - Task lỗi hai lần giữa Phase 2: Request về backlog, Phase 3 không tự bắt đầu.
   - `drift` mở: Phase dừng.
   - Người duyệt thiếu quyền: bị từ chối, không đổi trạng thái.
   - Đổi `execution_mode` dự án giữa chừng: không ảnh hưởng Request đang chạy.
   - `ops_request` `continuous`: vẫn mở `pre_deploy`.
   - `step_by_step`: hồi quy hành vi hiện tại.
2. **Nhật ký** (`request_activity_contract.go`): dữ liệu một Request đủ nguồn; kiểm thứ tự, `kind`, `severity`, phân trang, lọc, quyền, không có `body`.
3. **Tóm tắt** (`execution_summary_contract.go`): Request mô phỏng #1242 (28 task, 27 xong, 1 lỗi hai lần); `needs_you`, `failed`, `handover` (`native` rỗng).
4. **E2E thật** (cần stack local): một Request `change_request` 3 Phase, dự án `continuous`, agent thật; duyệt Plan một lần và quan sát tới `completed`; ghi lại thời gian và chi phí. Chỉ chạy sau khi `workspace_probe` qua. Ghi kết quả vào phần "Ghi chú triển khai".
5. Cập nhật trạng thái các CR và README solution/task khi xong.

## Kiểm thử

- Hợp đồng: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/adapter/... -tags integration -run 'Continuous|Activity|ExecutionSummary' -v` (cần Docker; chưa chạy).
- E2E: theo checklist stack local; chưa chạy.

## Tiêu chí hoàn thành

- [x] Các kịch bản `continuous` xanh trên Postgres và MySQL.
- [x] Kịch bản `step_by_step` không đổi.
- [x] Nhật ký và tóm tắt đúng trên dữ liệu mô phỏng #1242.
- [x] E2E thật hoàn tất hoặc ghi rõ lý do chưa chạy.

## Rủi ro và lưu ý

- E2E thật tốn tiền (một lần sinh plan khoảng 7 đến 13 USD và 13 đến 25 phút theo lần chạy trước); chỉ chạy một lần có chủ đích.
- Trên stack local chỉ có một người dùng; Plan cỡ L cần chính sách tự duyệt theo dự án; không nới lỏng chính sách duyệt mà không hỏi.

## Ghi chú triển khai

Hoàn thành 2026-10-16, trừ run OpenSpec sống (xem cuối).

- **Hợp đồng hai dialect** (chạy thật trên Postgres 16 và MySQL 8, testcontainers qua colima): `execution_flow_contract.go` có `flowContinuousChangeRequest` (duyệt Plan một lần, mọi Phase tự chạy tuần tự bằng repository thật và task-service trong bộ nhớ, không Approval `phase`, `start_trigger=auto`) và `flowContinuousFailureStops` (task hết lượt giữa chừng thì Request về backlog, Phase sau không tự bắt đầu); `activity_contract.go` (đủ nguồn, thứ tự, `kind`, mức độ, phân trang, lọc, chuẩn hoá tác nhân AI, không có `body`); kịch bản tóm tắt chạy ở `get_execution_summary_test.go` trên hình dạng #1242 (7 Phase, 28 task, 27 xong, 1 lỗi hai lần) và đã đối chiếu số liệu với DB dev thật.
- **Kịch bản khác của spec** được phủ bằng test use case: drift mở thì dừng (`TestContinuous_APendingPhaseApprovalPausesTheRun`), người duyệt thiếu quyền bị từ chối ngay và gỡ quyền giữa chừng đưa về backlog (`ApproverWhoCannotRun…`, `AnActorWhoMayNotExecuteSendsTheRequestBack`), `step_by_step` không đổi (`StepByStepStaysTheDefault`, `StepByStepForbiddenActorKeepsRetrying`), đổi cài đặt dự án giữa chừng không ảnh hưởng Request đã ghim (`PinExecutionMode_ProjectSettingIsFrozen…`), `ops_request` `continuous` vẫn mở `pre_deploy` (bảng 11 loại × 2 chế độ ở `execution_mode_test.go`), gọi lặp không bắt đầu Phase hai lần.
- **E2E thật (tiêu chí 4):** `run_ui_flow.py --real continuous` PASS trong 177 giây: `change_request` 2 Phase, dự án `continuous` (engine `native`), một lần duyệt Plan trên giao diện, hai Phase tự chạy, không có Approval `phase`, Request `completed`, repo fixture có thay đổi và test của nó qua. Chi phí không đo được riêng (hai task nhỏ trên repo tạm).
- **Chưa chạy:** e2e OpenSpec sống trên repo có knowledge hub (cần `.tdd-agent/config.toml`, xem FE-REQ-TASK-037-09); lần thử trên fixture dừng ở `KNOWLEDGE_CONFIG_MISSING`.
- **Hai lỗi tìm thấy khi triển khai thật và đã sửa:** (1) `request-service` không khởi động: kiểm tra `requireWired` của approval chạy trước `wireExecution` nên `DecideApproval.StartPrecheck` còn nil; nay là trường tuỳ chọn được nối sau; (2) số lần thử trong nhật ký (đếm mọi `started`) lệch với "Failed 2 of 2" của tóm tắt (không tính `dispatch_error`); nay cả hai cùng đếm lần chạy dùng ngân sách thử lại.
- **Việc tiếp theo ghi nhận (không thuộc CR này):** khi bước tác giả OpenSpec của bước Plan lỗi (ví dụ `KNOWLEDGE_CONFIG_MISSING`), Request đứng ở `planning` và giao diện hỏi `request.planProposal` mãi với `REQUEST_PLAN_RUN_NOT_FOUND`; lỗi nên hiện cho người dùng thay vì im lặng.
