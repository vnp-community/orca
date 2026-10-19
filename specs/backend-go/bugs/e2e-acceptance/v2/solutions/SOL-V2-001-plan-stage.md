# SOL-V2-001: Hoàn thiện giai đoạn Plan của request-service (CR-REQ-012, tasks 03 đến 07)

**Giải quyết:** BUG-V2-001 · **Service:** request-service, api-gateway (đã có kênh), task-service (đã có `CreatePlanTree`)
**Tham chiếu:** [BE-REQ-SOL-012](../../../../crs/v6/plan-phase-task/solutions/BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md), [CONTRACT-request-ui-api.md](../../../../crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md)

## 1. Hiện trạng đã đọc lại (2026-10-09)

- Đã có thật: proto `plan.proto` và RPC `GeneratePlan`/`GetPlanProposal`/`CommitPlan`; `task-service.CreatePlanTree` nguyên tử (idempotent theo `request_id`, hỗ trợ `supersedes_plan_id`); máy trạng thái (`plan_ready`, `plan_approved`, `plan_rejected`, `plan_revision`); `FlowDefinition.PlanKind/PhasesFor`; `TypePolicy.PlanPreconditions` cho mọi loại; khung chạy nền `AnalysisRun` + `AnalysisRunner` + sweeper; `OpenApproval`; thực thi Phase (`StartPhase`, `AdvanceExecution`); kênh gateway `request.generatePlan` (mode `propose` và `commit`) và `request.planProposal`.
- Còn là hàm rỗng: `usecase/generate_and_commit_plan.go`, `domain/plan_shape.go`, nên gRPC trả `Unimplemented` và gateway đổi thành `REQUEST_NOT_IMPLEMENTED`.
- Chưa có: nhãn kế hoạch dùng chung, kiểm đề xuất, prompt, worker AI, đọc đề xuất, commit, `SubjectArtifacts` cho `plan`/`task_list`, cổng `TaskPlanWriter`, ràng buộc DB cho run loại `plan`.

## 2. Giải pháp

### 2.1 Hình dạng Plan theo loại (một nguồn)

`domain.PlanShapeFor(flow, size)` suy từ `FlowDefinition.PlanKind` và `PhasesFor(size)`: `plan`+Phase (change_request mọi size; bug/refactor size L), `plan` không Phase (bug/refactor S/M, security, performance, ops_request), `task_list` (task, docs; một task `plan` vỏ), `single_task` (hotfix), `none` (spike, question: `REQUEST_PLAN_NOT_APPLICABLE`).

### 2.2 Chạy nền, không viết thêm bảng

`GeneratePlan` (PROPOSE) tạo một `AnalysisRun` loại `plan`, chế độ `complete`, rồi trả `run_id` ngay (AI có thể lâu hơn ngân sách WebSocket). Dùng lại hạ tầng của Solution: khoá một run đang chạy mỗi (request, kind), idempotency, lease có heartbeat, sweeper thu hồi run mồ côi. Hai thay đổi nhỏ ở hạ tầng: ràng buộc `kind` chấp nhận `plan` (migration cả hai dialect) và `StartRun` chấp nhận run không có Solution nháp. Kết quả thành công nằm ở `raw_output` dưới dạng JSON chuẩn đã che bí mật; `GetPlanProposal` đọc lại và kiểm lại, nên không có bảng thứ hai để lệch với run.

### 2.3 Kiểm đề xuất hai lần

`domain.ValidatePlanProposal(shape, limits, proposal)` chạy khi worker nhận JSON từ AI (nếu hỏng, thử sửa một lần như Solution) và lại chạy ở `CommitPlan` (không tin client, không tin AI). Mã lỗi: `REQUEST_PLAN_PHASES_NOT_ALLOWED`, `_PHASES_REQUIRED`, `_MIXED_CHILDREN`, `_TOO_LARGE`, `_INVALID_DEPENDENCY` (chỉ số ngoài phạm vi, tự phụ thuộc, vòng), `_INVALID_TASK_TYPE`, `_INVALID_TITLE`, `_REGRESSION_TEST_MISSING` (bug, security); quy tắc nhãn riêng từng loại do `TypePolicy.PlanPreconditions` (đã có).

### 2.4 Commit nguyên tử và chạy lại an toàn

`CommitPlan`: kiểm quyền, trạng thái `planning`, kiểm đề xuất, gọi chính sách loại; **trước** giao dịch gọi `TaskPlanWriter.CreatePlanTree` (task-service idempotent theo `request_id`, trả `already_exists`); rồi một giao dịch: khoá Request, ghi `plan_task_id` (CAS theo version), outbox `orca.request.plan.generated`, chuyển `plan_ready`, mở Approval `plan` hoặc `task_list`. Chết giữa chừng thì chạy lại không tạo trùng (task-service idempotent; `OpenApproval` trả dòng đang chờ khi digest trùng; chuyển trạng thái chỉ áp dụng một lần). Làm lại Plan (từ chối kèm góp ý, hoặc `GeneratePlan` có `feedback` khi đang chờ duyệt) truyền `supersedes_plan_id = plan_task_id` hiện có.

### 2.5 Duyệt Plan

`PlanArtifacts` (`SubjectArtifacts` cho `plan`, `task_list`): `Describe` đọc cây con của Plan qua task-service, kiểm Plan thuộc Request và không bị huỷ, trả digest chuẩn (id, tiêu đề, nhãn, cạnh phụ thuộc) để Approval gắn với đúng nội dung người duyệt thấy. Tác dụng lên Request đi qua `TransitionSubjectHandler` có sẵn (`plan_approved`, `plan_rejected` về backlog theo `ApprovalReturnStage`).

### 2.6 Giới hạn và an toàn

`REQUEST_PLAN_MAX_PHASES=8`, `REQUEST_PLAN_MAX_TASKS_PER_CONTAINER=20`, `REQUEST_PLAN_MAX_TASKS_TOTAL=100`, tiêu đề tối đa 200 ký tự (đọc từ env, giá trị sai giữ mặc định và cảnh báo). Mọi văn bản AI đi qua `RedactDocument` trước khi lưu. Prompt không chứa dữ liệu ngoài Request, Solution đã duyệt và tên project. Lỗi AI dùng chung bảng mã `REQUEST_AI_*` đã có.

## 3. Ngoài phạm vi vòng này (ghi rõ để không bị hiểu là xong)

- **Hotfix (`single_task`)** (TASK-V2-011): **đã làm sau vòng này** (2026-10-09, chưa commit). Khi chẩn đoán hotfix ghi xong (`AnalysisResultWriter.Persist`), `EnsureHotfixFixTask` dựng task sửa duy nhất từ chẩn đoán (`domain.HotfixFixProposal`, qua `ValidatePlanProposal` + `hotfixPolicy.PlanPreconditions` như CommitPlan), gọi task-service **trước** giao dịch (`TaskClient.CreateFixTask`, tức `CreateTask` có `request_id`, không `parent_id`), rồi trong giao dịch chuyển `analysis_ready` -> `awaiting_plan_approval` và mở Approval `pre_deploy` với `subject_id` là task (`PreDeployArtifacts` đã có). `plan_task_id` giữ NULL. Chạy lại an toàn: dùng lại task giống hệt (tên + mô tả), huỷ task cũ lệch nội dung, không đụng task đã chạy/xong; Request chuyển hướng giữa chừng thì huỷ task vừa tạo. Lỗi tạo task hoặc mở Approval làm run `failed` (`REQUEST_HOTFIX_FIX_TASK_FAILED`, `REQUEST_HOTFIX_APPROVAL_FAILED`), Request ở lại `analyzing`. Nối dây ở `bindHotfixFixTask` (`wire_plan.go`), chỉ khi Approval bật.
  - Test: `domain/hotfix_fix_task_test.go`, `usecase/ensure_hotfix_fix_task_test.go`, `usecase/analysis_result_writer_hotfix_test.go`, `adapter/grpcclient/task_fix_task_test.go`, hợp đồng DB `SolutionContract/HotfixFixTaskAndPreDeployApproval` (Postgres và MySQL, qua Approve của admin tới `executing`), e2e E10 (cả hai dialect).
  - Chưa làm: hotfix cũ đã kẹt ở `awaiting_plan_approval` không có task/Approval (tạo trước bản sửa) không tự phục hồi; cần huỷ hoặc trả về backlog rồi mở lại.
- Tối ưu hiển thị Plan ở frontend (xem [tasks frontend](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/README.md)).

## 4. Kiểm thử

Đơn vị: `PlanShapeFor` đủ 11 loại × 3 size; từng dòng `ValidatePlanProposal`; trích JSON và che bí mật; prompt theo hình dạng; `GeneratePlan` (trạng thái sai, không có dev server, idempotency); worker (hợp lệ, JSON hỏng rồi sửa, hết lượt); `CommitPlan` với bản giả `TaskPlanWriter` (mỗi điểm lỗi, chạy lại); `PlanArtifacts`. Hợp đồng repo: run loại `plan` không cần Solution nháp, hai dialect. Tích hợp: gRPC qua bufconn. E2E: `AIS-04` chạy Claude thật qua phiên CLI, thêm `PLN-*` (commit, duyệt, sang `executing`).
