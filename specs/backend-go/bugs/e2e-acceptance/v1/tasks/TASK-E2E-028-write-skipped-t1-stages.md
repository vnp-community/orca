# TASK-E2E-028: Viết các stage e2e Go đang skip (Solution, Plan, CommitPlan, ReportTaskOutcome)

**From Solution:** SOL-E2E-006 · **Resolves:** OBS-D
**Priority:** P2 · **Service/Area:** request-service
**Files:**
- backend-go/services/request-service/e2e/scenarios_flows_test.go
- backend-go/services/request-service/e2e/scenarios_cross_cutting_test.go
- backend-go/services/request-service/e2e/stubs
**Depends on:** TASK-E2E-026
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

49 stage tự skip với nhãn "implemented now … extend this stage" (TASK-REQ-025-04).

## Việc cần làm

- [x] Mỗi stage gọi RPC thật với tham số đúng (`GenerateSolution`, `ChooseSolutionOption`, `Approve`, `GeneratePlan`/`GetPlanProposal`/`CommitPlan`, `StartPhase`, `RecordRequestCheck`, `ReportTaskOutcome`, `Reject`).
- [x] Stub AI xác định ở tầng test (`flow_ai_answers_test.go`): trả Solution/Chẩn đoán/Findings/Answer/Plan hợp lệ theo `type`, `size` và danh sách AC đọc từ prompt.
- [x] Kiểm `WantStatus` (và `WantEvents`, `WantApprovals`) sau từng stage; đường E01-E12 đi trọn tới `completed`.

## Tiêu chí hoàn thành

- [x] Số stage skip theo nhãn "implemented now … extend this stage" = 0 (đã xoá `probeRPC`, `Stage.Needs` và cơ chế `blocked`; `grep` không còn).
- [x] 11 loại có kịch bản đi trọn tới `completed` (E01-E12), thêm E13 (Plan bị từ chối về backlog), E15 (task lỗi hai lần về backlog), E16 (sự kiện lặp/đảo thứ tự).

## Kiểm thử

Chạy gói e2e hai dialect.

## Kết quả thực thi

Xong. Chạy `go test -tags e2e ./e2e -count=1` (Colima: `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock`, `TESTCONTAINERS_RYUK_DISABLED=true`): **xanh trên Postgres (33 giây) và MySQL 8 (37 giây), 0 skip**.

**File đổi** (đều trong `backend-go/services/request-service`):
- `e2e/harness_test.go`: dựng task-service giả (TCP) và truyền `TASK_SERVICE_ADDR`; cài stub AI.
- `e2e/task_events_test.go` (mới): `taskStub` bọc `contracttest.FakeTaskService`; sự kiện trạng thái task đi vào binary thật qua RPC `ReportTaskOutcome` (cùng use case với consumer NATS), có hàng đợi tuần tự để không nghẽn khoá Request.
- `e2e/flow_ai_answers_test.go` (mới), `e2e/flow_driver_test.go` (mới): tài xế từng trạng thái; mọi phê duyệt do admin của tenant (không phải người báo); cổng `pre_deploy` từng task do admin thứ hai; bộ hạn mức ghi/AI vẫn bật, driver thử lại khi `ResourceExhausted` và không bắn `StartPhase` dồn dập (mỗi lần gọi tốn một token).
- `e2e/scenarios_flows_test.go`, `scenarios_cross_cutting_test.go`, `scenario_table_test.go`, `kit_test.go`: stage thật thay stage thăm dò; thêm `admin2`, `waitStatusWithin`.
- `internal/adapter/contracttest/execution_fake_task_service.go`, `execution_fake_plan_tree.go` (mới, kèm test): fake có thêm `CreateTask`, `CreatePlanTree` (idempotent theo request, thay Plan, chặn khi có task đang chạy), `NewFakeTaskService`, `Register`, `TasksOfRequest`, `TenantID` trong `StatusChange`.

**Lỗi sản phẩm tìm thấy nhờ chạy thật và đã sửa**: với loại có cổng Phase (`change_request`), Approval của **Phase đầu** không bao giờ được mở (chỉ Phase kế tiếp được mở khi Phase trước xong, và `StartPhase` đòi Approval `approved`), nên Request kẹt ở `executing`. Sửa: `StartExecution` gọi `EvaluateExecution.Run` cho Request có Phase, và `completePhases` mở Approval của Phase kế tiếp kể cả khi chưa Phase nào xong (idempotent, loại không có cổng Phase không mở). Test: `TestStatusConsumer_GatedFlowOpensTheFirstPhaseApprovalOnce`, `TestStatusConsumer_FlowsWithoutPhaseGateOpenNoPhaseApproval`; E01 trên cả hai dialect là bằng chứng đầu cuối. Đây cũng là một phần của mục "StartPhase với Plan có Phase" ở handoff mục 6.

**Chưa kiểm**: chạy qua NATS thật cho sự kiện task (e2e dùng RPC cùng use case); agent AI thật (stub xác định); `ORCA_E2E_EXECUTE_AGENT`.
