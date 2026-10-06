# Change Requests v6 — Request → Solution → Plan → Phase → Task

> **Mục tiêu:** đưa yêu cầu từ Jira, GitHub, MCP hoặc nhập tay vào một thực thể **Orca Request**, phân loại (AI đề xuất, người xác nhận), rồi đi qua đúng luồng theo loại: Solution (hoặc Chẩn đoán, Findings, Answer) → Plan → Phase → Task. Mỗi cấp có cổng duyệt; agent thực thi từng task và phản hồi ngược lên Phase, Plan, Solution, Request; không thực thi được thì Request về Request backlog kèm lý do.
>
> **Phạm vi:** service mới `backend-go/services/request-service`; thay đổi nhỏ ở `task-service`, `api-gateway`, `mcp-service`, `issue-status-sync`, `notification-service`; frontend `frontend/src/renderer/src`.
>
> **Nguồn:** nghiên cứu trong [`docs/research/receive-request/`](../../research/receive-request/): [phân loại và luồng](../../research/receive-request/request-classification-and-flows.md), [hiện trạng và phạm vi xây](../../research/receive-request/request-pipeline-existing-capabilities-and-build-scope.md), [đối chiếu pipeline](../../research/receive-request/request-to-task-pipeline-gap-analysis.md). Sơ đồ trực quan: artifact "Orca Request Flow" (riêng tư).

> **Trạng thái toàn series:** 📝 Đề xuất, chưa triển khai. Mọi CR được viết từ khảo sát code ngày 2026-10-05; chưa chạy hệ thống. Người triển khai phải đọc lại code liên quan trước khi sửa.

## 1. Hiện trạng đã xác nhận (khảo sát code 2026-10-05)

| Hạng mục | Thật sự có gì | Bằng chứng |
|---|---|---|
| Request, Solution, Plan, Phase | ❌ Không có bảng, entity, RPC hay UI | grep `backend-go`, `frontend/src` |
| Task | ✅ Cây cha-con, `task_edge`, `Type`, `Priority`, `Labels`, `AIContext`, `AIPlanJSON`, tiến độ cascade, `Grant` kế thừa theo cây | `task-service/internal/domain/task.go` |
| Status của Task | 6 giá trị: `open, blocked, in_progress, review, done, cancelled`. **Không có `backlog`** dù docs CR-TG-001 và frontend khai báo | `migrations/postgres/0003_task_fields_and_comments.up.sql` |
| `task_type` | `CHECK IN ('task','bug','feature','epic')` | cùng migration |
| `in_progress` | Chỉ `ExecuteTask` được đặt (`ErrCannotSetInProgress`) | `domain/task.go` |
| Nguồn ngoài | `task_sources` (`jira|linear|github|gitlab`, `ref`, `url`, `site`) | `domain/task_source.go`, migration `0012`, `0014` |
| AI chia task | `AIDecompose` → `SubtaskProposal` (chưa lưu) → `AIApply` | `usecase/ai_decompose.go`, `ai_apply.go` |
| Thực thi | `ExecuteTask` rẽ 3 engine, lease, recovery, `ReportTaskExecutionResult` | `usecase/execute_task.go`, CR-TG-008 |
| Cổng quyết định | `DecisionGate` (`CreateGate`, `ResolveGate`, `ListPendingDecisionGates`), HTTP `/v1/orchestration/gates`, **không có UI** | `orchestration-service`, `api-gateway/.../orchestration_routes.go` |
| MCP | `mcp-service` đã có; tool pack có `task.*`, `workflow.*`, `terminal.*`, `project.*`; chưa có `request.*` | `api-gateway/.../mcpserver/tools/` |
| Hai DB | `task-service` và `notification-service` có cả `adapter/postgres` và `adapter/mysql`; `mcp-service` chỉ Postgres | `services/*/internal/adapter` |
| Frontend | `components/task/*`, `hooks/useTask*.ts`; không có gì cho Request, Solution, Plan, cổng duyệt | `frontend/src/renderer/src` |

## 2. Quyết định đã chốt (người yêu cầu, 2026-10-05)

| # | Quyết định | Hệ quả |
|---|---|---|
| D1 | Tách **`request-service`** riêng để quản trị | Service mới, proto `orca.request.v1`, migration hai DB, gateway |
| D2 | **Plan và Phase dùng lại Task** với `type` mới `plan`, `phase` | Migration `CHECK`, lọc theo type, xử lý `in_progress` và `TaskNumber` |
| D3 | Xây **Approval tổng quát** | Bảng và API ở `request-service`; `DecisionGate` giữ riêng cho điều phối agent |
| D4 | **Backlog là view tính toán** hiển thị ở frontend, không thêm status backend | Ba view; gỡ `backlog` khỏi `TaskStatus` frontend |
| D5 | Phân loại bằng **AI đề xuất + người xác nhận**; chỉ `change_request` bắt buộc đi đủ Solution → Plan → Phase; đổi loại được và giữ phần đã làm | CR-REQ-003, 005 |

> Cách hiểu D2 và D4 là suy luận từ câu trả lời của người yêu cầu; cần xác nhận khi duyệt series này.

### Mặc định đề xuất cho các điểm còn mở (cần xác nhận)

| # | Điểm mở | Mặc định trong series này |
|---|---|---|
| O1 | Trạng thái `in_progress`, `review`, `done` của task `plan`/`phase` | **Suy ra từ con** (cascade trong `task-service`, CR-REQ-011). Không đặt qua `UpdateTask`. |
| O2 | `TaskNumber` (`#TG-N`) cho Plan và Phase | **Bỏ qua**: Plan, Phase không nhận số. |
| O3 | Cột Board `backlog` ở frontend | **Gỡ** `backlog` khỏi `TaskStatus` và Board (CR-REQ-018). |
| O4 | Hợp nhất `DecisionGate` với Approval | **Không** trong v6. Hộp duyệt chỉ hiện `Approval`. |

## 3. Mô hình chung (hợp đồng giữa các CR)

Mọi CR trong series phải dùng đúng tên và giá trị dưới đây. Nếu cần đổi, sửa tại đây trước.

### 3.1 Kiến trúc

```
Nguồn (Jira | GitHub | MCP | thủ công | webhook)
        │
        ▼
request-service  ── sở hữu: Request, RequestTypeHistory, Solution, RequestLink, Approval
        │  gRPC, không FK chéo service
        ├──────────────▶ task-service: Plan (type=plan) → Phase (type=phase) → Task (task|bug|feature)
        ├──────────────▶ ai-provider-service: phân loại, Solution, Plan, Phase
        ├◀──────────────  outbox của task-service: task đã chạy, lỗi, xong
        ├──────────────▶ outbox → notification-service, issue-status-sync, automation-service
        ▼
api-gateway (wscompat + HTTP)  ──▶  frontend, mcp-service tool pack
```

### 3.2 Loại Request (11)

`change_request`, `bug`, `hotfix`, `task`, `spike`, `question`, `refactor`, `security`, `performance`, `docs`, `ops_request`.

Thuộc tính phụ: `size` ∈ {`S`,`M`,`L`}; `urgency` ∈ {`normal`,`urgent`}; `type_source` ∈ {`ai`,`human`}; `confidence` ∈ [0,1].

### 3.3 Trạng thái Request

`new` → `classifying` → `awaiting_type_confirmation` → `analyzing` → `awaiting_analysis_approval` → `planning` → `awaiting_plan_approval` → `executing` → `completed`; ngoài ra `request_backlog` và `cancelled`.

- `analyzing`: Solution, Chẩn đoán, Findings hoặc Answer tuỳ loại.
- Loại bỏ qua trạng thái nào xem bảng 3.4. Loại luôn đi qua `new → classifying → awaiting_type_confirmation`.
- `request_backlog` ghi `returned_from_stage` ∈ {`classification`, `analysis`, `plan`, `phase`, `task`} và `return_reason`.
- Đổi loại từ mọi trạng thái đang xử lý quay về `awaiting_type_confirmation`; Solution, Plan, Task đã có giữ làm tham chiếu.

### 3.4 Luồng theo loại (registry chính thức)

`A` = bước phân tích, `P` = lập Plan, `Ph` = có Phase, `G` = các cổng duyệt (theo `subject_type`).

| Loại | Phân tích (`Solution.kind`) | Plan | Phase | Cổng duyệt (`Approval.subject_type`) |
|---|---|---|---|---|
| `change_request` | `solution` (≥2 phương án) | ✔ | ✔ | `request_type`, `solution`, `plan`, `phase` (mỗi phase) |
| `bug` | `diagnosis` | ✔ (Fix plan) | chỉ khi `size=L` | `request_type`, `solution`, `plan` |
| `hotfix` | `diagnosis` nhanh (không cổng) | – (1 task) | – | `request_type` (bắt buộc người), `pre_deploy` |
| `task` | – | `task_list` | – | `request_type`, `task_list` |
| `spike` | `findings` | – | – | `request_type`, `findings` |
| `question` | `answer` | – | – | `request_type`, `answer` (người dùng chấp nhận) |
| `refactor` | `solution` (hướng) | ✔ | chỉ khi `size=L` | `request_type`, `solution`, `plan` |
| `security` | `diagnosis` (phạm vi ảnh hưởng) | ✔ (Fix plan) | – | `request_type` (xác nhận mức độ), `solution`, `pre_deploy` |
| `performance` | `diagnosis` (đo baseline) | ✔ (Plan tối ưu) | – | `request_type`, `solution`, `plan` |
| `docs` | – | `task_list` | – | `request_type`, `task_list` |
| `ops_request` | – | ✔ (runbook + rollback) | – | `request_type`, `plan`, `pre_deploy` (trước bước không đảo ngược) |

Đường nâng cấp và Request con: `bug|task|refactor|performance|docs → change_request`; `security → hotfix`; `spike|question → change_request|task` (Request con); `hotfix → bug|task` (Request theo dõi). Chi tiết ở CR-REQ-003 và CR-REQ-006.

### 3.5 Mô hình dữ liệu

**`request-service`** (schema/DB `request`, Postgres và MySQL):

| Bảng | Trường chính |
|---|---|
| `requests` | `id`, `tenant_id`, `project_id`, `number`, `title`, `body`, `source_provider` (`jira|github|gitlab|linear|mcp|manual|webhook`), `source_ref`, `source_url`, `source_site`, `type`, `type_source`, `size`, `urgency`, `confidence`, `classification_reason`, `status`, `returned_from_stage`, `return_reason`, `plan_task_id`, `reporter_id`, `created_at`, `updated_at`, `version` |
| `request_type_history` | `id`, `request_id`, `from_type`, `to_type`, `actor_id`, `actor_kind` (`ai|user`), `reason`, `at` |
| `solutions` | `id`, `request_id`, `kind` (`solution|diagnosis|findings|answer`), `status` (`draft|proposed|approved|rejected|superseded`), `options` (JSON), `chosen_option`, `content_ref`, `generation_run_id`, `created_at` |
| `request_links` | `parent_request_id`, `child_request_id`, `reason` (`spawned_by_spike|spawned_by_question|followup_hotfix|escalation`) |
| `approvals` | `id`, `tenant_id`, `request_id`, `subject_type`, `subject_id`, `stage`, `status` (`pending|approved|rejected|cancelled|expired`), `requested_by`, `decided_by`, `decided_at`, `comment`, `due_at`, `version` |
| `request_idempotency` | `tenant_id`, `source_provider`, `source_site`, `source_ref`, `request_id` (khoá duy nhất) |
| `outbox` | theo `common/outbox` |

**`task-service`:** `task_type` thêm `plan`, `phase`; liên kết ngược trong `Task.Labels` hoặc cột mới `request_id` (CR-REQ-011 chốt cột `request_id` nullable, không FK).

### 3.6 RPC và kênh

- gRPC `orca.request.v1.RequestService`: `CreateRequest`, `GetRequest`, `ListRequests`, `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType`, `GenerateSolution`, `ListSolutions`, `ChooseSolutionOption`, `GeneratePlan`, `StartPhase`, `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListBacklog`, `ReportTaskOutcome` (nội bộ).
- gRPC `orca.request.v1.ApprovalService`: `RequestApproval`, `Approve`, `Reject`, `Cancel`, `GetApproval`, `ListApprovals`, `ListPendingForUser`.
- Kênh WS (`api-gateway/wscompat`): `request.*`, `solution.*`, `approval.*`, `backlog.*`.
- Tool MCP: `request_*`, `solution_*`, `approval_*` (tên kênh thay `.` bằng `_`, quy ước của CR-MCP-007).

### 3.7 Sự kiện (outbox, subject `orca.request.*`)

`request.created`, `request.classified`, `request.type_confirmed`, `request.type_changed`, `request.status_changed`, `request.returned`, `request.completed`, `solution.proposed`, `solution.approved`, `approval.requested`, `approval.decided`, `plan.generated`, `phase.started`, `phase.completed`.

### 3.8 Ba view backlog (D4)

| View | Điều kiện |
|---|---|
| Request backlog | `requests.status = request_backlog` |
| Task backlog | Task làm việc dưới Plan chưa có Approval `approved`, hoặc Plan chưa chia Phase |
| Execute backlog | Task làm việc dưới Phase đã `approved`, status `open` hoặc `blocked`, hoặc `execution_link` gần nhất `failed` |

## 4. Danh sách feature và CR

| Feature (folder) | CR | Nội dung | Priority | Effort |
|---|---|---|---|---|
| [`request-service-foundation`](./request-service-foundation/README.md) | CR-REQ-001 | Dựng `request-service` (module, proto, DB hai dialect, outbox, wiring, CI) | 🔴 P0 | Medium |
| | CR-REQ-002 | Mô hình dữ liệu, migration và repository cho Request, lịch sử loại, Solution, liên kết | 🔴 P0 | Medium |
| [`request-lifecycle`](./request-lifecycle/README.md) | CR-REQ-003 | Máy trạng thái Request và registry luồng theo loại | 🔴 P0 | Large |
| | CR-REQ-004 | Tiếp nhận Request từ Jira, GitHub, thủ công, webhook, MCP | 🔴 P0 | Large |
| | CR-REQ-005 | Phân loại bằng AI, xác nhận, đổi loại, lịch sử | 🔴 P0 | Large |
| | CR-REQ-006 | Trả về Request backlog, mở lại, hủy, Request con | 🟠 P1 | Medium |
| [`solution-analysis`](./solution-analysis/README.md) | CR-REQ-007 | Sinh Solution nhiều phương án, chọn, duyệt | 🔴 P0 | Large |
| | CR-REQ-008 | Chẩn đoán, Findings, Answer; chạy agent không cần worktree | 🟠 P1 | Large |
| [`approval`](./approval/README.md) | CR-REQ-009 | Approval tổng quát: domain, API, vòng đời | 🔴 P0 | Large |
| | CR-REQ-010 | Chính sách quyền duyệt, thông báo, hết hạn | 🟠 P1 | Medium |
| [`plan-phase-task`](./plan-phase-task/README.md) | CR-REQ-011 | `task-service`: `type` plan/phase, lọc, cascade trạng thái, không số task | 🔴 P0 | Large |
| | CR-REQ-012 | Sinh Plan, Phase và Task từ Solution đã duyệt | 🔴 P0 | Large |
| | CR-REQ-013 | Thực thi theo Phase và phản hồi ngược lên Plan, Solution, Request | 🔴 P0 | Large |
| | CR-REQ-014 | Chính sách theo loại: hotfix, security, performance, ops_request, refactor | 🟠 P1 | Large |
| [`backlog-views`](./backlog-views/README.md) | CR-REQ-015 | API đọc ba view backlog | 🟠 P1 | Medium |
| [`gateway-and-mcp`](./gateway-and-mcp/README.md) | CR-REQ-016 | Kênh WS/HTTP của `api-gateway` cho request, solution, approval, backlog | 🔴 P0 | Medium |
| | CR-REQ-017 | Tool MCP `request_*`, `solution_*`, `approval_*` và MCP làm nguồn Request | 🟠 P1 | Medium |
| [`request-frontend`](./request-frontend/README.md) | CR-REQ-018 | Nền frontend: kiểu, hook, store, định tuyến; gỡ `backlog` khỏi `TaskStatus` | 🔴 P0 | Medium |
| | CR-REQ-019 | Danh sách, chi tiết, xác nhận phân loại; "Tạo Request" từ Tasks | 🔴 P0 | Large |
| | CR-REQ-020 | Xem, so sánh và duyệt Solution | 🔴 P0 | Large |
| | CR-REQ-021 | Cây Plan → Phase → Task và duyệt | 🔴 P0 | Large |
| | CR-REQ-022 | Hộp duyệt chờ xử lý | 🟠 P1 | Medium |
| | CR-REQ-023 | Màn hình Backlog (ba view) | 🟠 P1 | Medium |
| [`request-quality-rollout`](./request-quality-rollout/README.md) | CR-REQ-024 | Đồng bộ trạng thái Jira theo Request, audit, observability | 🟠 P1 | Medium |
| | CR-REQ-025 | Kiểm thử đầu cuối, feature flag, rollout, tài liệu | 🔴 P0 | Medium |
| [`solution-engines`](./solution-engines/README.md) | CR-REQ-026 | OpenSpec solution engine sau giao diện `SolutionEngine` | 🟡 P2 | Large |
| [`request-artifact-model`](./request-artifact-model/README.md) | CR-REQ-027 | Lược đồ và ontology có phiên bản, bản chiếu Markdown/YAML, bảng phủ yêu cầu | 🔴 P0 | Large |
| | CR-REQ-028 | Clarification, Decision, trạng thái `awaiting_information` | 🔴 P0 | Large |
| [`execution-contract`](./execution-contract/README.md) | CR-REQ-029 | TaskSpec, ExecutionPacket, ReadinessGate, ExecutionResult, phân loại lỗi | 🔴 P0 | Large |
| [`impact-risk`](./impact-risk/README.md) | CR-REQ-030 | Đánh giá tác động và chấm điểm rủi ro | 🟠 P1 | Large |
| [`context-sources`](./context-sources/README.md) | CR-REQ-031 | Source Registry và Context Pack Builder, MCP ngoài | 🟠 P1 | Large |
| [`request-frontend`](./request-frontend/README.md) | CR-REQ-032 | Canvas đồ thị và các lens | 🟠 P1 | Large |
| [`agent-capabilities`](./agent-capabilities/README.md) | CR-REQ-033 | Agent: chế độ chỉ đọc, `workspaceKind`, khối kết quả, báo cáo năng lực (**duy nhất chạm `agent/`**) | 🔴 P0 | Large |
| [`ai-governance`](./ai-governance/README.md) | CR-REQ-034 | Ngân sách AI, chọn model, phiên bản prompt, eval | 🟠 P1 | Large |
| [`security-compliance`](./security-compliance/README.md) | CR-REQ-035 | Quyền mức Request, RLS thật, secretscan, audit, lưu giữ | 🔴 P0 | Large |
| [`request-frontend`](./request-frontend/README.md) | CR-REQ-036 | Giao diện hỏi lại, quyết định, sẵn sàng, tác động | 🟠 P1 | Large |

## 5. Thứ tự thực thi

```
CR-001 ─▶ CR-002 ─▶ CR-003 ─┬─▶ CR-004 ─▶ CR-005 ─▶ CR-006
                            │                │
                            │                ▼
                            ├─▶ CR-009 ─▶ CR-010
                            │        │
                            │        ▼
                            ├─▶ CR-007 ─▶ CR-008
                            │        │
                            ▼        ▼
                         CR-011 ─▶ CR-012 ─▶ CR-013 ─▶ CR-014
                                                │
                                                ▼
                                             CR-015
CR-016 (sau 003..009) ─▶ CR-017
CR-018 ─▶ CR-019 ─▶ CR-020 ─▶ CR-021 ─▶ CR-022, CR-023     (frontend theo sau khi RPC chạy, nguyên tắc "UI sau cùng")
CR-024, CR-025: sau khi lõi (001–016) ổn định
```

Đợt gợi ý:

| Đợt | CR | Kết quả có thể chạy được |
|---|---|---|
| 1 | 001, 002, 003, 004, 005, 009 | Tạo Request, phân loại, xác nhận loại, duyệt (chưa có Solution) |
| 2 | 007, 008, 016, 018, 019, 020 | Sinh và duyệt Solution; UI Request và Solution |
| 3 | 011, 012, 013, 021 | Plan, Phase, Task; thực thi và phản hồi ngược |
| 4 | 006, 010, 014, 015, 022, 023 | Request backlog, loại đặc thù, hộp duyệt, backlog |
| 5 | 017, 024, 025 | Nguồn MCP, đồng bộ Jira, rollout |

## 6. Quy ước chung cho mọi CR trong series

- **Hai DB:** mọi migration và repository viết cho cả Postgres và MySQL, dùng `common/dbcapability`; kiểm tra theo CR-DB-002 (`SKIP LOCKED` cần MySQL ≥ 8.0.1).
- **Không FK chéo service:** liên kết giữa `request-service` và `task-service` bằng id, kiểm tra ở tầng ứng dụng.
- **Tenant:** mọi bảng có `tenant_id`; mọi use case gọi `tenant.RequireTenantID`.
- **Phản hồi idempotent:** callback và consumer outbox chịu được giao lặp (at-least-once), theo mẫu `ReportTaskExecutionResult`.
- **Quyền:** kế thừa mô hình `Grant`/RBAC; mọi lệnh ghi qua gateway kiểm quyền; mọi tool MCP ghi qua chính sách của `mcp-service`.
- **SSH và remote:** không giả định thực thi cục bộ (AGENTS.md). Agent chạy qua đường `task.execute` hiện có.
- **Git:** không thêm lệnh git mới trong series này; nếu cần, theo [`git-compatibility.md`](../../../guides/reference/git-compatibility.md).
- **Provider:** tên không gắn riêng GitHub; GitLab và Linear dùng cùng `source_provider`.
- **UI:** theo [`STYLEGUIDE.md`](../../../guides/STYLEGUIDE.md), dùng shadcn primitives; tên file theo khái niệm cụ thể, không dùng `utils`, `helpers`, `common`.
- **Không thêm `max-lines` disable**; tách file khi quá dài.
- **Feature flag:** toàn bộ luồng sau cờ `request_flow_enabled` (CR-REQ-025), mặc định tắt.

## 7. Rủi ro toàn series

- Docs và code lệch nhau (đã gặp: status `backlog`, MCP service). Mỗi CR yêu cầu đọc lại code trước khi sửa.
- Dùng lại Task cho Plan và Phase làm mọi nơi giả định "Task là việc làm được" cần rà (Board, thống kê, thông báo, `ExecuteTask`).
- Luồng Jira chưa chạy trên Jira thật (`task_sources` trên server dev có 0 dòng): nguồn Request còn chưa kiểm chứng.
- `performance`, `ops_request`, `spike`, `question` cần năng lực agent mà code hiện chưa có (đo baseline, runbook có rollback, chạy không cần worktree).
- Quyền duyệt chưa được thiết kế ở mức tổ chức; CR-REQ-010 chỉ đặt mặc định.

## 8. Điều chỉnh hợp đồng sau khi soạn các CR (cần chốt khi duyệt)

Các CR được soạn song song và phát hiện chỗ README này thiếu hoặc lệch code. Các điều chỉnh dưới đây **đã được áp dụng trong CR liên quan**; mục 3 ở trên giữ nguyên bản gốc để đối chiếu. Khi hai nơi khác nhau, theo CR.

| # | Nội dung | CR nắm quyết định |
|---|---|---|
| 1 | Đường AI không đi qua `ai-provider-service` (chỉ có `ResolveProvider` và quản lý account). Dùng relay `ai.complete` và `agent.execPrompt` của dev server agent, cần project có dev server đang kết nối | 005, 007, 008, 012 |
| 2 | `spike` và `question` chưa chạy được không cần worktree (`ExecuteTask` luôn `EnsureWorktree`); thêm `AgentReadonlyRunner` trong `request-service`; chưa ép được chỉ đọc, chỉ giảm thiểu | 008 |
| 3 | Tên bảng outbox là `outbox_events`; mọi bảng có `tenant_id`; thêm `request_counters`, `processed_events`, `analysis_runs`, `request_return_history`, `phase_starts`, `task_run_outcomes`, `request_checks`, `tenant_settings`; `requests.type` nullable tới khi có đề xuất đầu tiên; cột bổ sung cho `approvals` (`subject_digest`, `self_approval_allowed`, `idempotency_key`, `reminded_at`) | 002, 004, 006, 007, 009, 013, 025 |
| 4 | Subject sự kiện theo mẫu repo: `orca.request.<entity>.<event>` (ví dụ `orca.request.request.status_changed`); không có `request.reopened` và `request.cancelled`, dùng `status_changed` kèm `trigger` | 003, 006 |
| 5 | Mã lỗi tiền tố `REQUEST_`; `chosen_option` là chỉ số nguyên; `GenerateSolution` nhận `feedback` khi sinh lại | 003, 007 |
| 6 | Cổng `phase` và `pre_deploy` chặn trong `executing`, trừ `pre_deploy` của `hotfix`/`security` chiếm `awaiting_plan_approval`. Orca không có bước deploy; `pre_deploy` là cổng trước khi chạy task có nhãn `gate:pre_deploy` hoặc task fix | 003, 014 |
| 7 | Execute backlog mở rộng bằng "cổng của task" theo `FlowFor`, vì nhiều loại không có Phase | 015 |
| 8 | Task xong chỉ ở `review`; cần cờ `REQUEST_AUTO_COMPLETE_TASKS` để task sau được mở khoá. Request ở lại `executing` khi chờ duyệt Phase kế tiếp | 013 |
| 9 | `task-service` hiện không phát sự kiện cho `ClaimForExecution`, `CompleteExecution`, `ReleaseExecution`, `UpdateStatus`; bổ sung cho task có `request_id`. Số migration tiếp theo là 0015 (hai dialect) | 011, 013 |
| 10 | `Approval` của `mcp-service` đã tồn tại; phía Request dùng subject `orca.request.approval.*`. Vai trò chỉ có `admin|user` và team, người duyệt đặc biệt dựng bằng `team:<id>` | 009, 010 |
| 11 | Frontend: `TaskStatus` còn `todo` backend không có; `TaskType` frontend (`epic|story|task|subtask|bug|spike`) lệch backend. CR-018 chỉ thêm `plan\|phase`; việc dọn lệch cần CR riêng | 018 |
| 12 | Thiếu RPC trong 3.6: `GetRequestFlow`, `ListRequestTypeHistory`, `ListRequestLinks`, `CommitPlan`, `RecordRequestCheck`, `ListRequestChecks`, `GetRequestFlowSettings`, `SetRequestFlowSettings`, `LookupRequestBySource`; ở `task-service`: `CreatePlanTree`, `ListExecutionStates`. Tên kênh WS chốt ở CR-016 (`request.generatePlan`, `request.startPhase`) | 003, 006, 012, 013, 016, 024 |
| 13 | Gateway không kiểm quyền OPA trước định tuyến: mọi RPC của `request-service` tự kiểm quyền. `parity_test.go` của MCP đỏ nếu thêm channel mà không có `ToolSpec` hoặc dòng loại trừ | 016, 017 |
| 14 | `common/auditclient.Append` không mang `actor_type`, `target_type`; đề xuất `AppendDetailed`. `issue-status-sync` chưa có `/metrics` | 024 |
| 15 | Đường dẫn đúng: `guides/STYLEGUIDE.md` và `guides/reference/git-compatibility.md` (AGENTS.md và các README cũ trỏ `docs/...` không tồn tại) | tất cả |

Điểm chưa ai chốt: Request chưa có `Grant` nên chưa rõ ai có quyền ghi ở mức Request (CR-003 hoặc CR-010 chốt); bật cờ chỉ theo tenant, chưa theo loại (CR-025).


## 9. Bộ solution và task thực thi

Mỗi CR có solution và task ở từng khu vực mà nó chạm tới, theo thư mục feature trùng tên:

| Khu vực | Vị trí | Solution | Task |
|---|---|---|---|
| Backend | [`specs/backend-go/crs/v6/`](../../../specs/backend-go/crs/v6/README.md) | 28 | 199 |
| Frontend | [`specs/frontend/crs/v6/`](../../../specs/frontend/crs/v6/README.md) | 8 | 54 |
| Agent | [`specs/agent/crs/v6/`](../../../specs/agent/crs/v6/README.md) | 3 | 13 |

Mỗi README ở các nơi trên có danh sách "điểm cần chốt" riêng. Các điểm chéo khu vực quan trọng nhất: số migration của `request-service` chồng nhau, timeout WebSocket 25 giây so với lời gọi AI đồng bộ, RLS thật ở `request-service`, token màu rủi ro ở frontend, và chế độ chỉ đọc của agent chưa kiểm chứng với `claude` thật.

CR-REQ-027 đến 036 được soạn sau mục 3 của README này; nếu mâu thuẫn với mục 3 hoặc mục 8, theo CR (xem mục "Tác động tới CR hiện có" của từng CR để biết CR cũ nào cần sửa gì).
