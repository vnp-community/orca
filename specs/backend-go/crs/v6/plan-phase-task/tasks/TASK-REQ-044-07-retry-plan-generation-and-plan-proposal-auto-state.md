# TASK-REQ-044-07: `RetryPlanGeneration`, khối `auto_plan` của `GetPlanProposal`, proto `plan.proto` và gRPC

**From Solution:** [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/usecase/retry_plan_generation.go` (mới), `internal/usecase/retry_plan_generation_test.go` (mới), `internal/usecase/get_plan_proposal.go` (sửa), `internal/usecase/get_plan_proposal_auto_test.go` (mới), `internal/adapter/grpc/server_plan_auto.go` (mới), `internal/adapter/grpc/server_plan.go` (sửa), `internal/adapter/grpc/server_plan_auto_test.go` (mới), `internal/adapter/grpc/flow_gate.go` (sửa), `internal/domain/rpc_catalog.go` (sửa), `cmd/server/wire_plan.go`, `cmd/server/wire_plan_auto.go` (sửa), `backend-go/proto/orca/request/v1/plan.proto`, `backend-go/proto/orca/request/v1/request.proto` (sửa) + mã sinh
**Depends on:** TASK-REQ-044-06
**Status:** [ ] TODO

---

## Context

- `GetPlanProposal.Execute` (`get_plan_proposal.go`) trả `REQUEST_PLAN_RUN_NOT_FOUND` khi Request chưa có run Plan nào (`find`, cuối hàm); giao diện (`usePlanRunInFlight`) coi đó là "không có run". Khối mới phải cho phép giao diện biết một job đang chờ khi chưa có run (hoãn vì bận, hoặc giữa hai lượt).
- `GetPlanProposalResponse` dùng tới trường 12 (`plan.proto:60-76`); trường mới là 13.
- RPC mới đăng ký ở `request.proto` cạnh `GeneratePlan` (`request.proto:357`), `domain/rpc_catalog.go:107` (mẫu `GeneratePlan`: `onRequest(GroupPlan, "request_id", RateAI, false)`), `adapter/grpc/flow_gate.go:114` (`classAdvance`).
- `PlanAutoSwitch.GloballyAvailable()` (task 04) cho `enabled`; `enabled` theo dự án cần thêm `Settings.Get`.

## Việc cần làm

1. `plan.proto`: message `PlanAutoState {bool enabled = 1; string state = 2; string origin = 3; int32 attempt = 4; int32 max_attempts = 5; string reason_code = 6; string next_attempt_at = 7;}` (state: `pending|running|committing|done|gave_up|skipped|superseded`, rỗng khi không có job; `next_attempt_at` RFC3339, chỉ điền khi `pending` sau một lượt thất bại hoặc hoãn); `GetPlanProposalResponse.auto_plan = 13`; `RetryPlanGenerationRequest {string request_id = 1;}` và `RetryPlanGenerationResponse {PlanAutoState auto_plan = 1;}`; `rpc RetryPlanGeneration` trong `request.proto`. `make proto-gen && make proto-lint`.
2. `RetryPlanGeneration.Execute(ctx, requestID)` theo solution mục 2.6: tenant; `AuthorizePlan` (người dùng thật); công tắc env/commit tắt → `domain.ErrPlanAutoDisabled()`; Request phải `planning` (`ErrPlanWrongState`); job hoạt động có sẵn → trả nó; run Plan mới nhất `succeeded` và `PlanTaskID == ""` → chèn job `retry` ở `committing` với `run_id` đó; ngược lại chèn job `retry` `pending` (`anchor = RetryAnchor(uuid)`, `max_attempts = 1`). Chèn trùng (chỉ mục hoạt động) → đọc lại job hoạt động và trả. Ghi `requested_by`.
3. `GetPlanProposal`: thêm `AutoPlan PlanAutoView` (`WithAutoPlan(jobs PlanAutoJobRepository, sw *PlanAutoSwitch, settings EngineSettingsRepository)`): `Enabled` = công tắc toàn cục và công tắc dự án (lỗi đọc → `false`); job mới nhất của Request chỉ đưa vào khi Request đang `planning` (nếu không, `State` rỗng). Khi không có run Plan nhưng có job, trả view với `RunID ""`, `Status ""` và `AutoPlan`, không lỗi; khi không có cả hai vẫn `REQUEST_PLAN_RUN_NOT_FOUND` (giao diện cũ giữ nguyên).
4. gRPC: `RetryPlanGeneration` trong `server_plan_auto.go` (lỗi qua `apperrors.ToGRPCStatus`; `Unimplemented` khi chưa nối); `GetPlanProposal` ánh xạ `auto_plan`.
5. `rpc_catalog.go`: `RetryPlanGeneration: onRequest(GroupPlan, "request_id", RateAI, false)`; `flow_gate.go`: `classAdvance`. Không thêm vào `agent_rpcs` của `request.rego`.
6. `PlanUseCases` thêm `Retry *usecase.RetryPlanGeneration`; `wirePlanAuto` (task 06) gắn nó và `Get.WithAutoPlan(...)`; khi tính năng tắt, `Retry` là nil nhưng `Get` vẫn trả `auto_plan{enabled=false}` (để giao diện biết dùng luồng thủ công).

## Kiểm thử

- `RetryPlanGeneration`: (a) job `pending` mới với anchor `retry-...`; (b) đã có job hoạt động → trả nó, không chèn; (c) run mới nhất `succeeded` chưa commit → job ở `committing` với đúng `run_id`, **không** gọi `GeneratePlan`; (d) Request không `planning` → `REQUEST_STATE_STALE`; (e) công tắc tắt → `REQUEST_PLAN_AUTO_DISABLED`; (f) không phải người báo cáo/admin → `REQUEST_PLAN_FORBIDDEN`; (g) bấm hai lần liên tiếp → một job.
- `GetPlanProposal`: chỉ có job (không run) → view rỗng + `auto_plan`; có cả run `failed` và job `gave_up` → `status=failed`, `error_code` và `auto_plan.state=gave_up`; Request không `planning` → `auto_plan.state` rỗng; `enabled=false` khi công tắc dự án tắt; không run và không job → vẫn `REQUEST_PLAN_RUN_NOT_FOUND`.
- gRPC: ánh xạ, `Unimplemented` khi chưa nối; test `rpc_catalog` (mỗi RPC có mục) và `flow_gate` xanh.
- `cd backend-go && make proto-gen proto-lint && cd services/request-service && go test ./internal/usecase/... ./internal/adapter/grpc/... ./internal/domain/... -run 'RetryPlan|GetPlanProposal|RPCCatalog|FlowGate' -v` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Retry sau lỗi commit không tốn AI lần nữa; Retry sau lỗi AI tạo job mới một lượt.
- [ ] Bấm Retry hai lần không sinh hai job hay hai run.
- [ ] `GetPlanProposal` cũ (không job) trả đúng như trước; client không biết `auto_plan` không bị ảnh hưởng.
- [ ] RPC mới có mục trong `rpc_catalog` và `flow_gate`, agent không gọi được.

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `GetPlanProposal.Execute` (gateway và nhiều test dùng) và `NewGetPlanProposal`; `PlanProposalView` thêm trường.
- Trả `auto_plan` cho mọi Request tốn thêm một lần đọc cài đặt dự án; chấp nhận được (cùng pattern `RequestEngineResolver` đọc một lần mỗi dự án).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
