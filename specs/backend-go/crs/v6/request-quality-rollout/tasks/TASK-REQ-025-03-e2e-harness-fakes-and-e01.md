# TASK-REQ-025-03: Khung e2e T1 (testcontainers, fake cổng AI, `task-service` giả) và kịch bản E01

**From Solution:** BE-REQ-SOL-025
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/services/request-service/e2e/harness_test.go` (mới), `.../e2e/fakes/` (mới: `classifier.go`, `solution_generator.go`, `plan_generator.go`, `agent_readonly.go`, `task_service.go`), `.../e2e/scenario_table_test.go` (mới), `.../e2e/request_flow_test.go` (mới, E01)
**Depends on:** CR-REQ-003 đến 013 ở mức chạy được; TASK-REQ-025-01
**Status:** `[x] DONE`

---

## Context

- Mẫu testcontainers hai dialect: `issue-status-sync/internal/adapter/mysql/processed_events_test.go` (tag `integration`) và `backend-go/common/testutil`; workflow `backend-go-issue-status-sync.yml` chạy `go test -tags=integration`.
- Đường AI **không** qua `ai-provider-service` (README v6 mục 8 dòng 1): relay `ai.complete` và `agent.execPrompt`. T1 giả ở các cổng của `request-service`: `RequestClassifier` (CR-REQ-005), `SolutionGenerator` (CR-REQ-007), `PlanGenerator` (CR-REQ-012), `AgentReadonlyRunner` (CR-REQ-008); tên cổng chốt theo code thật khi triển khai.
- `task-service` giả: `CreatePlanTree`, `CreateTask`, `ListTasks`, `ListExecutionStates`, `ExecuteTask`, và nơi tiêm kết quả qua `ReportTaskOutcome` (CR-REQ-011 đến 013).
- E01: `change_request` đầy đủ (CR-REQ-025 mục 2.3).

## Việc cần làm

1. `harness_test.go`: build tag `//go:build e2e`; `TestMain` dựng DB theo biến `E2E_DIALECT` (`postgres` hoặc `mysql`) bằng testcontainers, chạy migration của `request-service`, dựng toàn bộ use case với fake, mở gRPC in-process (`bufconn`) cho `RequestService`, `ApprovalService`; cờ bật cho tenant thử. Hàm `newTenant(t)` tạo tenant và người dùng (`reporter`, `approver`) khác nhau.
2. `fakes/`: mỗi fake nhận kịch bản (`Script`) trả JSON hợp lệ (đúng schema của CR) hoặc lỗi; `task_service.go` lưu cây trong bộ nhớ, đánh dấu task xong khi kịch bản gọi `CompleteTask(id)` và gọi `ReportTaskOutcome` của `request-service` (có chế độ giao lặp và đảo thứ tự).
3. `scenario_table_test.go`: `type Scenario struct { ID string; Type string; Size string; Steps []Step; Expect Expectations }` với `Expectations{Statuses []string; Approvals []string; SolutionKind string; PlanShape ...; Events []string; Audits []string}`; hàm `runScenario(t, sc)` thực hiện bước, đọc trạng thái, outbox (`outbox_events`), audit giả; khẳng định.
4. `request_flow_test.go`: E01 (tạo, phân loại, xác nhận, Solution 2 phương án, chọn, duyệt, Plan, duyệt, 2 Phase duyệt từng cái, task chạy xong, `completed`).
5. Quy tắc: không `time.Sleep` cố định; thăm dò có hạn (`require.Eventually`) cho các bước bất đồng bộ (consumer outbox, run AI).

## Kiểm thử

- Tự nó là kiểm thử: `E2E_DIALECT=postgres go test -tags=e2e ./e2e/... -run TestE01` và `E2E_DIALECT=mysql`. Cần Docker.
- Test cho chính harness: kịch bản mẫu "không làm gì" xanh (smoke).

## Tiêu chí hoàn thành

- [x] E01 xanh trên Postgres và MySQL.
- [x] Fake thay được mọi cổng AI và `task-service`; không còn lời gọi mạng ra ngoài.
- [x] Không `Sleep` cố định.

## Rủi ro và lưu ý

- `task-service` giả có thể che lỗi ở đường nối thật; T2 bù.
- Thời gian chạy e2e hai dialect; đặt timeout job CI rõ (task 05).
