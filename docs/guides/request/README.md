# Luồng Request trong Orca: hướng dẫn người dùng, quản trị và vận hành

**Cập nhật:** 2026-10-08 · Đối chiếu với: `backend-go/services/request-service` (README, `internal/domain/request_flow_registry.go`,
`internal/adapter/grpc/flow_gate.go`, `cmd/server/wire_approval.go`), [CR v6](../../crs/v6/README.md),
[CR-REQ-024](../../crs/v6/request-quality-rollout/CR-REQ-024-jira-status-sync-audit-observability.md),
[CR-REQ-025](../../crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md).

Một **Request** là một yêu cầu công việc (từ Jira, GitHub, nhập tay, agent qua MCP, webhook) đi qua một luồng có cổng duyệt
theo loại của nó. Tài liệu này chỉ mô tả điều **đã có trong mã**; chỗ chưa có được ghi "chưa có" kèm task chặn.

## Trạng thái hiện thực (đọc trước)

Luồng hiện chạy được **tới hết bước xác nhận loại**: tạo Request, AI đề xuất loại, người xác nhận hoặc đổi loại, rồi Request vào
`analyzing` (hầu hết loại) hoặc `planning` (`task`, `docs`). Từ đó trở đi **chưa có**:

| Giai đoạn | Tình trạng | Task giao |
|---|---|---|
| Solution / Chẩn đoán / Findings / Answer (`GenerateSolution`, `ListSolutions`, `ChooseSolutionOption`) | chưa có, RPC trả `Unimplemented` | CR-REQ-007, 008 |
| Plan (`GeneratePlan`, `GetPlanProposal`, `CommitPlan`) | chưa có | CR-REQ-012 |
| Phase và thực thi (`StartPhase`, `ReportTaskOutcome`) | chưa có | CR-REQ-013 |
| `ListBacklog` (ba view backlog) | chưa có | CR-REQ-006, 015 |
| Làm rõ, quyết định, artifact, impact, readiness, context, compliance, `AiBudgetAdminService` | chưa có | các CR tương ứng |

Hệ quả: một Request mới dừng ở `analyzing` hoặc `planning` cho tới khi các task trên xong; chưa có đường đi tiếp tới `executing`
và `completed` bằng RPC thật. Có thể đưa Request ra bằng `ReturnToBacklog` hoặc `CancelRequest`.

## Ai cần đọc gì

| Bạn là | Đọc |
|---|---|
| Người tạo Request, cần phân loại | [Tạo và phân loại Request](./creating-and-classifying-requests.md) |
| Người duyệt | [Phê duyệt Request](./approving-requests.md) |
| Cần hiểu backlog | [Backlog của Request và Task](./request-and-task-backlogs.md) |
| Dùng agent / MCP | [Request từ agent qua MCP](./requests-from-agents-mcp.md) |
| Admin tenant / ops muốn bật luồng | [Bật luồng Request](./admin-enable-request-flow.md) |
| Trực vận hành | [Runbook](./runbook-request-flow.md) |
| Đồng bộ Jira | [Jira ↔ Orca](../jira/jira-orca-mapping.md) (mục "Request sở hữu issue") |

## 11 loại Request

`change_request`, `bug`, `hotfix`, `task`, `spike`, `question`, `refactor`, `security`, `performance`, `docs`, `ops_request`.
Bảng luồng theo loại nằm ở `internal/domain/request_flow_registry.go` (`flowDefinitions`):

| Loại | Phân tích | Kế hoạch | Phase | Cổng đầu thực thi | Ghi chú |
|---|---|---|---|---|---|
| `change_request` | Solution (duyệt `solution`) | Plan | luôn | `plan` | cổng `phase` trong lúc thực thi |
| `bug` | Chẩn đoán (duyệt `solution`) | Plan | khi size L | `plan` | |
| `hotfix` | Chẩn đoán, không cổng | một task | không | `pre_deploy` | bắt buộc người xác nhận loại; phải `urgent` |
| `task` | không | danh sách task | không | `task_list` | |
| `spike` | Findings (duyệt `findings`) | không | không | không | xong sau phân tích |
| `question` | Answer (duyệt `answer`) | không | không | không | xong sau phân tích |
| `refactor` | Solution | Plan | khi size L | `plan` | |
| `security` | Chẩn đoán (duyệt `solution`) | Plan | không | `pre_deploy` | bắt buộc người xác nhận loại; cần lý do |
| `performance` | Chẩn đoán | Plan | không | `plan` | |
| `docs` | không | danh sách task | không | `task_list` | |
| `ops_request` | không | Plan | không | `plan` | thêm cổng `pre_deploy` trong lúc thực thi |

Các cột Solution, Plan, Phase mô tả **thiết kế của registry**; phần thực hiện chúng chưa có (xem bảng trạng thái ở trên).

## 11 trạng thái

`new` → `classifying` → `awaiting_type_confirmation` → `analyzing` → `awaiting_analysis_approval` → `planning` →
`awaiting_plan_approval` → `executing` → `completed`; ngoài ra `request_backlog` và `cancelled`.

| Trạng thái | Nghĩa |
|---|---|
| `new` | vừa tạo |
| `classifying` | AI đang đề xuất loại, size, urgency (chạy nền) |
| `awaiting_type_confirmation` | có đề xuất, chờ người xác nhận (cổng `request_type`) |
| `analyzing` | đang phân tích (chưa có bước thật, xem trên) |
| `awaiting_analysis_approval` | chờ duyệt Solution/Findings/Answer (chưa có) |
| `planning` | đang lập kế hoạch (chưa có) |
| `awaiting_plan_approval` | chờ duyệt Plan/task list/pre-deploy (chưa có) |
| `executing` | đang thực thi (chưa có) |
| `completed` | xong |
| `request_backlog` | trả về backlog kèm lý do |
| `cancelled` | đã huỷ |

## Ba view backlog

| View | Định nghĩa (thiết kế, README v6 mục 3.8) | Tình trạng |
|---|---|---|
| Request backlog | `requests.status = request_backlog` | trạng thái và các lệnh vào/ra có; RPC `ListBacklog` chưa có |
| Task backlog | Task dưới Plan chưa được duyệt, hoặc Plan chưa chia Phase | chưa có (CR-REQ-015) |
| Execute backlog | Task dưới Phase đã duyệt, `open`/`blocked`, hoặc `execution_link` gần nhất `failed` | chưa có (CR-REQ-015) |

Chi tiết: [Backlog của Request và Task](./request-and-task-backlogs.md).

## Cờ `request_flow_enabled`

Mặc định **tắt**. Cần bật ở hai tầng (biến môi trường của `request-service` và cài đặt của tenant). Xem
[Bật luồng Request](./admin-enable-request-flow.md).

## Kiểm thử và công cụ kiểm tra

| Thứ | Nơi | Ghi chú |
|---|---|---|
| T1: e2e trong tiến trình | `backend-go/services/request-service/e2e` (thẻ build `e2e`, `E2E_DIALECT=postgres` hoặc `mysql`) | chặn PR |
| T2: e2e trên stack dev | `tests/request`, `backend-go/ci/request-e2e/run-request-e2e.sh`, workflow `request-e2e.yml` | **không** chặn PR; chưa chạy trên stack dev thật |
| T3: AI thật | cùng T2 với `ORCA_REQUEST_E2E_REAL_AI=true` | không chặn, chỉ báo cáo |
| Stub agent cho đường AI | `backend-go/services/request-service/e2e/stubs`, `e2e/cmd/agent-stub` | trả lời relay `ai.complete` của `infra-fleet-service` theo dấu `[e2e:type=bug size=S]` trong tiêu đề |
| Kiểm đăng ký service | `backend-go/ci/check-request-service-wiring.sh` (+ `_test.sh`) | báo lỗi nếu thiếu `request` ở go.work, Makefile, compose, migrate... |

Trong T1, các giai đoạn phụ thuộc RPC chưa có sẽ bị bỏ qua có ghi task chặn, và tự chạy khi RPC được hiện thực.
