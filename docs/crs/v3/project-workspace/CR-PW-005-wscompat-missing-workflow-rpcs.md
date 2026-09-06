# CR-PW-005 — `backend-go`'s wscompat chỉ expose 4/11 RPC đã có sẵn của `workflow-service`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-005 |
| **Tên** | Đăng ký 7 wscompat channel còn thiếu cho `WorkflowServiceClient` (không đổi proto) |
| **Loại** | Bug Fix / Feature Completion |
| **Priority** | 🟡 P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-06 |
| **Trạng thái** | ✅ Implemented — xem [BE-SOL-001](../../../../../specs/backend-go/crs/v3/project-workspace/solutions/BE-SOL-001-workflow-wscompat-wiring.md) |
| **Tác giả** | Investigation tiếp nối CR-PW-003 (Web mode gap) |
| **Tác động HLD** | F38 — Project Workspace |
| **Tác động Features** | Workflows tab (Web mode / hosted backend-go) |

---

## Bối cảnh & Vấn đề

`backend-go/proto/orca/workflow/v1/workflow.proto`'s `WorkflowService` có **11 RPC**:
`CreateTemplate, UpdateTemplate, Execute, GetExecution, PauseExecution, ResumeExecution,
ExecuteAdHocStep, CancelExecution, ListTemplates, ResolveTemplate, HasActiveExecutions` — tất cả
đã được generate (`buf generate`), đã có gRPC server implementation
(`services/workflow-service/internal/adapter/grpc/server.go`).

Nhưng `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go` (nơi
frontend Web mode thực sự gọi RPC qua) chỉ đăng ký **4/11**: `workflow.execute`,
`workflow.cancel`, `workflow.template.create`, `workflow.template.update`.

Hậu quả: CR-PW-003 wire `WorkflowMonitor.tsx` gọi `workflow.listExecutions` — RPC này **không tồn
tại trong proto** (chỉ có filter tương đương trong TS legacy backend, xem "Liên quan" bên dưới),
nhưng ngay cả những RPC **đã tồn tại sẵn** trong proto backend-go như `GetExecution`,
`ListTemplates`, `PauseExecution`, `ResumeExecution` cũng chưa gọi được từ Web mode — mọi lời gọi
`workflow.getExecution`/`.pause`/`.resume`/`.template.list`/`.template.resolve`/
`.hasActiveExecutions`/`.executeAdHocStep` từ frontend Web mode đều fail với lỗi "unknown channel"
ở tầng wscompat, dù Electron/local mode (gọi thẳng TS `workflow-rpc-handler.ts`) hoạt động bình
thường.

Đây là 1 phần nguyên nhân của CR-PW-006's architecture gap: Web mode "kẹt" ở model mỏng của
backend-go không chỉ vì model thiếu field, mà còn vì **những RPC đã sẵn sàng cũng chưa được nối
dây** tới wscompat.

## Giải pháp (tóm tắt — chi tiết ở BE-SOL-001)

Đăng ký 7 channel còn thiếu trong `registerWorkflowChannels`, mirror đúng shape
`r.Register(...)` đã dùng cho `workflow.execute`/`.cancel`/`.template.create`/`.template.update`:

| Channel | RPC | Request mapping |
|---|---|---|
| `workflow.getExecution` | `GetExecution` | `{executionId}` → `GetExecutionRequest{Id}` |
| `workflow.pause` | `PauseExecution` | `{executionId}` → `PauseExecutionRequest{Id}` |
| `workflow.resume` | `ResumeExecution` | `{executionId}` → `ResumeExecutionRequest{Id}` |
| `workflow.template.list` | `ListTemplates` | `{scope, pageToken, pageSize}` |
| `workflow.template.resolve` | `ResolveTemplate` | `{templateId}` |
| `workflow.hasActiveExecutions` | `HasActiveExecutions` | `{projectId}` |
| `workflow.executeAdHocStep` | `ExecuteAdHocStep` | `{stepType, stepConfigJson, requestId}` — `tenantId` LUÔN lấy từ `Identity`, không lấy từ args (cùng nguyên tắc bảo mật với `workflow.template.create`) |

Không có thay đổi proto — cả 7 RPC này đã tồn tại, đã build, đã có server implementation từ
trước; đây thuần tuý là nối dây (wiring) ở tầng api-gateway.

## Phát hiện phụ, KHÔNG sửa ở CR này

Cả 4 channel cũ (`workflow.execute`/`.cancel`/`.template.create`/`.template.update`) trả về
**raw `*workflowv1.X` proto message** (`resp.GetExecution()`, `resp.GetTemplate()`), không đi qua
1 camelCase view struct. protoc-gen-go's `encoding/json` struct tag là snake_case
(`json:"template_id,omitempty"` — tag `json=templateId` là protojson-only), và wscompat's envelope
serialize `Result any` bằng `encoding/json` thường, không phải `protojson`. Nghĩa là 4 channel cũ
(và có thể cả 1 số channel cũ khác trong package) **có thể đã và đang ship snake_case key** cho
1 frontend type kỳ vọng camelCase (`WorkflowExecution.templateId`, không phải `template_id`).

Đây là 1 pattern đã được phát hiện và fix ở `channels_dev_server_access_control.go`/
`channels_tenant_project.go` (dùng view struct riêng) nhưng **chưa được áp dụng ngược lại** cho
`channels_workflow.go`. CR-PW-005 mirror đúng shape cũ theo yêu cầu phạm vi hẹp (chỉ nối dây RPC
còn thiếu, không đổi response shape của RPC đã có) — nên 7 channel mới **cũng thừa hưởng cùng vấn
đề** này. Ghi nhận rõ ở đây, không sửa, để tránh mở rộng phạm vi CR ngoài "nối dây" thuần tuý —
follow-up riêng nếu cần fix, ảnh hưởng cả 11 channel cùng lúc.

## Không thuộc phạm vi CR này

- Không sửa response-shape snake_case/camelCase nêu trên (xem mục ngay trên).
- Không thêm `ListExecutions`/`ListStepExecutions` RPC (RPC này **không tồn tại** trong proto —
  cần đổi `.proto` + `buf generate`, không phải "nối dây RPC có sẵn") — đó là CR-PW-006 Phase B.
- Không đổi `agent/` — không liên quan, CR này chỉ đụng `api-gateway`'s wscompat layer.
- Không đổi model dữ liệu (`WorkflowExecution` proto message vẫn giữ nguyên 5 field hiện có:
  `id, template_id, status, root_trace_id, project_id`) — model mỏng vẫn còn đó, CR-PW-006 mới xử
  lý phần này.

## Liên quan

- `backend-go/proto/orca/workflow/v1/workflow.proto`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow_test.go`
- `backend-go/services/workflow-service/internal/adapter/grpc/server.go`
- `backend/src/main/workflow/workflow-rpc-handler.ts` — nơi có `workflow.listExecutions` thật
  (legacy TS, Electron/local mode only) mà CR-PW-003 đã wire vào — RPC này KHÔNG tồn tại ở
  backend-go, xem CR-PW-006.
- [CR-PW-003](./CR-PW-003-workflows-tab-wiring.md), [CR-PW-006](./CR-PW-006-execution-monitoring-architecture.md)
- [BE-SOL-001](../../../../../specs/backend-go/crs/v3/project-workspace/solutions/BE-SOL-001-workflow-wscompat-wiring.md)
