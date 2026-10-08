# TASK-REQ-012-07: Test tích hợp và e2e luồng sinh Plan (hai dialect, hai service)

**From Solution:** BE-REQ-SOL-012
**Priority:** P1
**Service:** `request-service`, `task-service`
**File:** `backend-go/services/request-service/internal/adapter/grpcclient/task_plan_writer_integration_test.go` (mới), `backend-go/services/request-service/internal/usecase/plan_flow_integration_test.go` (mới), `backend-go/services/task-service/internal/adapter/grpc/create_plan_tree_contract_test.go` (mới)
**Depends on:** TASK-REQ-012-01 đến 06
**Status:** [ ] TODO

---

## Context

- Mẫu integration của repo: testcontainers với tag `integration` (`task-service/internal/adapter/postgres/repository_test.go`, `mysql/repository_test.go`); CI chạy theo từng service.
- E2E ở `request-service` cần task-service thật hoặc một fake gRPC dựng từ cùng proto; ưu tiên chạy task-service thật với DB test để bắt lỗi ánh xạ thật (chưa kiểm chứng khả năng CI dựng hai service).
- Kịch bản tiêu chí chấp nhận của CR-REQ-012 mục 4 là nguồn của các test này.
- Chưa chạy test nào ở thời điểm viết.

## Việc cần làm

1. Dựng khung test: khởi tạo DB task-service (Postgres và MySQL) + `request-service` repository hai dialect, và gRPC server task-service thật bằng `bufconn`.
2. Kịch bản `change_request`: Request `planning` + Solution `approved` → `GeneratePlan` (PlanGenerator giả trả JSON mẫu) → `CommitPlan` → kiểm: có Plan, các Phase, task, cạnh, `request_id`, `plan_task_id` đã đặt, một Approval `plan` `pending`, Request `awaiting_plan_approval`, một dòng outbox `orca.request.plan.generated`.
3. Kịch bản idempotent: `CommitPlan` hai lần liên tiếp và 8 lần đồng thời: một Plan, một Approval.
4. Kịch bản `task`/`docs`: Plan vỏ và Approval `task_list`; `hotfix`: một task, `plan_task_id` NULL.
5. Kịch bản replan: Plan cũ và con chưa `done` thành `cancelled`, Approval cũ `cancelled`; Plan cũ có con `in_progress` thì `REQUEST_PLAN_HAS_RUNNING_TASKS`.
6. Kịch bản quyền: người tạo có quyền `read` và `execute` trên mọi task của cây (`ResolvePermission` qua grant kế thừa).
7. Test hợp đồng: JSON mẫu của PlanGenerator cho từng loại (11 loại, mỗi loại một file JSON trong `testdata/`) đi qua `ValidateProposal` và `CreatePlanTree` thành công hoặc bị từ chối đúng mã.
8. Ghi vào PR danh sách "chưa kiểm chứng" nếu CI không dựng nổi Docker.

## Kiểm thử

- `go test -tags=integration ./services/request-service/... ./services/task-service/internal/adapter/... -run 'PlanFlow|CreatePlanTree' -v` từ `/opt/repos/orca/backend-go`.
- Tên: `TestPlanFlow_ChangeRequest_E2E`, `TestPlanFlow_CommitTwiceConcurrent_OnePlan`, `TestPlanFlow_TaskListShell`, `TestPlanFlow_Hotfix_SingleTask`, `TestPlanFlow_Replan`, `TestPlanFlow_CreatorHasInheritedGrants`, `TestPlanProposalFixtures_AllTypes`.

## Tiêu chí hoàn thành

- [ ] Toàn bộ tiêu chí mục 4 của CR-REQ-012 có ít nhất một test tương ứng.
- [ ] Chạy xanh trên Postgres và MySQL (hoặc ghi rõ phần chưa chạy được).
- [ ] Test lỗi giữa chừng không để lại Plan hoặc task mồ côi.
- [ ] Không dữ liệu thật hoặc khoá bí mật trong `testdata/`.

## Rủi ro và lưu ý

- Test hai service chậm; đặt sau tag `integration` và tách job CI.
- Phụ thuộc cổng của CR-REQ-003, 009: nếu chưa merge, dùng fake nhưng ghi rõ.
- Không dùng `sleep` để chờ; kiểm trạng thái sau lời gọi đồng bộ.
