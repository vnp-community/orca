# TASK-REQ-012-04: Proto `request_plan.proto`, adapter `PlanGenerator` (relay `ai.complete`) và `TaskPlanWriter`

**From Solution:** BE-REQ-SOL-012
**Priority:** P0
**Service:** `request-service`
**File:** `backend-go/proto/orca/request/v1/request_plan.proto` (mới), `backend-go/services/request-service/internal/usecase/ports.go`, `internal/adapter/grpcclient/plan_generator_relay.go` (mới), `internal/adapter/grpcclient/task_plan_writer.go` (mới), `internal/adapter/grpcclient/plan_prompt.go` (mới), `internal/adapter/grpcclient/plan_generator_relay_test.go` (mới)
**Depends on:** TASK-REQ-012-01 (proto `CreatePlanTree`), TASK-REQ-012-03
**Status:** `[ ] TODO`

---

## Context

- AI chạy qua relay, không qua `ai-provider-service`: `task-service/internal/adapter/grpcclient/aidecompose_relay.go` (`AICompleter.Complete(ctx, connectionID, prompt)`) gọi `infrafleetv1.InfraFleetServiceClient.Relay` với method `ai.complete`, `ParamsJson={"prompt": ...}`, kết quả `{"content": ...}`. `AIDecompose` lấy `connectionID` từ project của task và trả `TASK_AI_DECOMPOSE_NO_CONNECTION` khi không có (`ai_decompose.go` dòng 82). `request-service` cần cùng đường: BE-REQ-SOL-007 (solution-analysis) đã chốt cổng `AICompleter` ở `request-service/internal/usecase/ports.go` cùng adapter Relay `ai.complete` (timeout `REQUEST_AI_COMPLETE_TIMEOUT`); `PlanGenerator` **dùng lại** `AICompleter`, không tạo client Relay thứ hai. Đọc code BE-REQ-SOL-007 đã merge trước khi viết.
- `withTenantMetadata(ctx)` (`grpcclient/tenant_forwarding.go` ở task-service) chuyển tenant qua metadata: request-service cần cơ chế tương đương.
- Hợp đồng gọi `task-service`: `CreatePlanTree`, `CreateTask`, `ListTasks`, `GetSubtree` đều đã có hoặc do SOL-011/012 thêm. `TaskClient` dùng chung với SOL-013 và 015 (`internal/adapter/grpcclient/task_client.go`, mới ở CR-REQ-013): **tránh hai file định nghĩa kết nối**, task này tạo `task_plan_writer.go` chỉ phần ghi và dùng chung `Dial` do CR-REQ-001 dựng.
- Chưa có bằng chứng relay chạy với prompt kích thước kế hoạch; coi là chưa kiểm chứng.

## Việc cần làm

1. `request_plan.proto`: `PlanProposal`, `PhaseProposal`, `TaskProposal` (trường theo solution 2.3), `GeneratePlanRequest/Response`, `CommitPlanRequest/Response`; thêm hai RPC vào `RequestService` (file `request.proto` do CR-REQ-001 sở hữu: chỉ import, không sửa số trường). `make proto-gen && make proto-lint`.
2. `ports.go`: 
   ```go
   type PlanGenerator interface { Generate(ctx context.Context, in PlanGenerationInput) (domain.PlanProposal, string, error) }
   type TaskPlanWriter interface {
       CreatePlanTree(ctx context.Context, in PlanTreeWrite) (PlanTreeResult, error)
       CreateSingleTask(ctx context.Context, in SingleTaskWrite) (taskID string, err error)
   }
   ```
   `PlanGenerationInput{Request domain.Request, Solution domain.Solution, Shape domain.PlanShape, Feedback string, TechStack string}`.
3. `plan_prompt.go`: dựng prompt yêu cầu JSON đúng cấu trúc `PlanProposal`, theo loại (bug/security: Fix plan có task `test:regression`; ops_request: runbook, mỗi bước một task, `irreversible`, có `rollback`; performance: task `check:baseline` đầu, `check:after` cuối; refactor: `check:tests_before` đầu, `check:tests_after` cuối). Chỉ dựng chuỗi; hằng nhãn lấy từ `domain/plan_labels.go`.
4. `plan_generator_relay.go` (tên giữ vì là adapter của `PlanGenerator`, phần gọi mạng đi qua `AICompleter.Complete`): không có kết nối dev server thì `REQUEST_PLAN_AI_UNAVAILABLE`; parse JSON (loại bỏ code fence ```json nếu có), lỗi parse thì `REQUEST_PLAN_AI_INVALID_JSON` kèm raw.
5. `task_plan_writer.go`: ánh xạ `domain.PlanProposal` sang `CreatePlanTreeRequest` (`ai_context` mỗi task = `REQ-<number>: <title>` + tóm tắt phương án; `description` của Phase là phạm vi và tiêu chí xong; thêm `gate:pre_deploy` khi `Irreversible`); `ai_plan_json` = JSON của proposal cộng raw.
6. Lỗi gRPC của task-service ánh xạ về mã `REQUEST_*` (ví dụ `TASK_PLAN_HAS_RUNNING_TASKS` thành `REQUEST_PLAN_HAS_RUNNING_TASKS`, FailedPrecondition); lỗi `Unavailable` giữ nguyên để retry.
7. Timeout cho `Generate` (mặc định đề xuất 120 giây, chưa kiểm chứng) và cho `CreatePlanTree` (30 giây).

## Kiểm thử

- `TestPlanPrompt_PerType` (golden file nhỏ cho `bug`, `ops_request`, `performance`, `refactor`).
- `TestPlanGeneratorRelay_ParsesFencedJSON`, `_InvalidJSON_ReturnsRaw`, `_NoConnection_Unavailable` (fake `InfraFleetServiceClient`).
- `TestTaskPlanWriter_MapsProposal` (kiểm `ai_context`, nhãn `gate:pre_deploy`, chỉ số phụ thuộc), `_MapsTaskServiceErrors`.
- Hợp đồng: `make proto-lint`; test round-trip JSON mẫu AI từng loại qua `PlanGenerator` giả.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/adapter/grpcclient/...`.

## Tiêu chí hoàn thành

- [ ] Proto biên dịch, `buf lint` xanh, không phá số trường.
- [ ] `Generate` trả `PlanProposal` hợp lệ từ JSON mẫu, và mã lỗi đúng khi JSON hỏng hoặc không có dev server.
- [ ] `TaskPlanWriter` truyền đủ `request_id`, `project_id`, `creator_id`, nhãn, phụ thuộc.
- [ ] Không có hai client Relay trùng trong `request-service`.

## Rủi ro và lưu ý

- Chất lượng JSON của AI chưa đo; prompt cần chỉnh sau khi chạy thật (chưa kiểm chứng).
- CR-REQ-029 (TaskSpec) sẽ đổi trường `TaskProposal`; chỉ thêm trường mới vào proto (additive), không đổi số trường cũ.
- Không log toàn bộ prompt và raw (có thể chứa nội dung riêng tư); log độ dài và id.
