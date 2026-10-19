# TASK-REQ-038-03: Thực thi liên tục: tự bắt đầu Phase, ghim mode, kiểm quyền người duyệt

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/{start_execution,start_phase,evaluate_execution,approval_artifact_handler}.go` (sửa), `internal/usecase/start_phase_auto.go` (mới, nếu cần tách), `internal/domain/execution_errors.go` (sửa), các fake và `internal/adapter/contracttest/execution_flow_contract.go` (sửa)
**Depends on:** TASK-REQ-038-01, TASK-REQ-038-02
**Status:** [x] DONE

---

## Context

- `StartExecution.Execute` trả `started=false` cho luồng có Phase và chỉ gọi `Evaluate.Run`; Phase đầu chờ `StartPhase` của người.
- `EvaluateExecution.completePhases` khi phase xong mở Approval `phase` cho Phase kế (`e.Open.Execute`). `continuous` phải thay bước này bằng `StartPhase{Internal:true}`.
- `StartPhase` có `Internal` (bỏ kiểm người gọi) nhưng cần `ActorID`; `ensureAccess` và `checkPredecessors` vẫn chạy.
- Handler Approval của Plan (`approval_artifact_handler.go`, `OnApproved` dùng `TriggerPlanApproved`) là nơi chuyển Request sang `executing`.
- Người thực thi = `decided_by` của Approval khởi đầu (B11). Gỡ quyền giữa chừng: task kế tiếp lỗi quyền, Request về `request_backlog` `category=blocked_dependency`.

## Việc cần làm

1. **Ghim mode:** trong luồng chuyển sang `executing` (cùng transaction với `TriggerPlanApproved`), gọi `Requests.PinExecutionMode(req.ID, EffectiveExecutionModeOf(...))`.
2. **Kiểm quyền lúc duyệt:** trong `OnApproved` của handler Plan (và `pre_deploy` làm `StartGate`), khi mode hiệu lực là `continuous` và luồng có Phase, gọi `ExecutionAccess.Ensure(ctx, req, approverID)`; lỗi thì trả `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` và **không** chuyển trạng thái. Thêm lỗi domain tương ứng ở `execution_errors.go`.
3. **`StartExecution.Execute`:** nếu `continuous` và `flow.PhasesFor(req.Size)`: chọn Phase đầu sẵn sàng bằng `tree.NextReadyPhase(started, edges)` (đọc `depends_on` qua `GetSubtree`, như `nextReadyPhase`), gọi `StartPhase.Execute` với `Internal:true`, `ActorID` = `decided_by` của Approval khởi đầu (qua `ExecutionActors`/`ApprovalGateReader`), `Trigger=auto`; trả `started=true`.
4. **`completePhases`:** khi mode đã ghim là `continuous`: sau khi ghi `phase_done`, nếu không Phase nào còn chạy và `nextReadyPhase` có kết quả thì gọi `StartPhase` như trên thay cho `e.Open.Execute(... SubjectPhase ...)`. `step_by_step` giữ nguyên.
5. **`StartPhase`:** thêm `Trigger` vào `StartPhaseInput` (`manual` mặc định, `auto` khi `Internal`); `claim` ghi `phase_starts.trigger`. Không đổi kiểm `ErrPhaseNotApproved`: chỉ áp khi `HasExecutionGate(GatePhase)` theo mode đã ghim (task 02).
6. **Điều kiện dừng:** xác nhận bằng test rằng không Phase mới tự bắt đầu khi Request ở `awaiting_information`, `request_backlog`, `cancelled`, hoặc `drift` đang mở; `ops_request` vẫn mở `pre_deploy`.
7. Chạy `gitnexus_impact` trên `StartExecution.Execute`, `EvaluateExecution.completePhases`, `StartPhase.Execute` và báo blast radius trong PR.

## Kiểm thử

- `TestStartExecution_Continuous_StartsFirstPhaseAuto`, `_StepByStep_WaitsForPerson` (hồi quy).
- `TestCompletePhases_Continuous_StartsNextPhaseOnce` (gọi lặp không bắt đầu hai lần), `_RespectsDependsOn`, `_StopsOnBacklog`, `_StopsOnAwaitingInformation`.
- `TestPlanApproval_Continuous_ActorWithoutAccess_Rejected_NoTransition`.
- `TestPinExecutionMode_ChangingProjectSettingMidRun_NoEffect`.
- `TestOpsRequest_Continuous_StillOpensPreDeployGate`.
- Contract flow: 3 Phase chạy hết sau một Approval Plan; `phase_starts.trigger=auto`.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run 'Continuous|StartPhase|CompletePhases|PlanApproval' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [x] `continuous`: một lần duyệt Plan thì mọi Phase tự chạy tuần tự, không Approval `phase`.
- [x] `step_by_step`: hành vi không đổi, toàn bộ test hiện có xanh.
- [x] Người duyệt thiếu quyền bị từ chối ngay lúc duyệt.
- [x] Gỡ quyền giữa chừng đưa Request về backlog với `category=blocked_dependency`.
- [x] Gọi lặp (event trùng, reconcile) không bắt đầu Phase hai lần.

## Rủi ro và lưu ý

- Blast radius của `StartPhase`/`EvaluateExecution` cao (mọi luồng thực thi). Giữ nhánh `continuous` nhỏ và tách hàm.
- Chưa kiểm chứng lỗi quyền của `task-service` đủ phân biệt với lỗi task thường; nếu không, ghi `cause=env_defect` và đưa vào backlog.
- Không đổi hành vi cổng `pre_deploy` và Clarification (B7).

## Ghi chú triển khai

Hoàn thành 2026-10-16. Đã chạy `gitnexus_impact` theo tài liệu cho `StartExecution.Execute`, `EvaluateExecution.completePhases`, `StartPhase.Execute` trước khi sửa.

- `usecase/execution_mode_pin.go` ghim chế độ khi vào `executing` (`Pinner`, `Modes` trong `StartExecution`). `start_phase.go` tách `claimPhase` dùng chung cho thủ công và tự động (`start_trigger`), cổng duyệt Phase theo mode.
- `continuous_phases.go`: `startNextPhase` tự bắt đầu Phase kế tiếp với tư cách người duyệt Plan (dùng `ExecutionActors`/`ExecutionAccess` có sẵn, không dựng cơ chế mới); dừng khi có drift review chờ duyệt, Request về backlog, `awaiting_information` hoặc task hết số lần thử. `evaluate_execution.go` rẽ nhánh `continuous` ở `completePhases`.
- `approval_execution_precheck.go` + `decide_approval.go`: duyệt Plan bị từ chối ngay với `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` nếu người duyệt không chạy được thực thi (`ExecutionAccess.CheckReady`, chỉ đọc). Gỡ quyền giữa chừng: `returnWhenActorForbidden` đưa Request về backlog `blocked_dependency`.
- Kiểm chứng: `continuous_execution_test.go`, `approval_execution_precheck_test.go` và kịch bản `flowContinuousChangeRequest`, `flowContinuousFailureStops` trong `execution_flow_contract.go` (repository thật + task-service trong bộ nhớ, Postgres). Gọi lặp không bắt đầu Phase hai lần (`claimPhase` idempotent theo `phase_starts`).
