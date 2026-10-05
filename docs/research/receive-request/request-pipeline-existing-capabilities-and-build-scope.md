# Orca đã có gì cho pipeline Request → Solution → Plan → Phase → Task, và cần xây thêm gì

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-05 |
| **Loại** | Nghiên cứu (chưa phải CR) |
| **Phương pháp** | Đọc code `backend-go/services/*` và `frontend/src/renderer/src/components/task/` kết hợp docs. Chưa chạy hệ thống. |
| **Liên quan** | [request-to-task-pipeline-gap-analysis.md](./request-to-task-pipeline-gap-analysis.md), [request-classification-and-flows.md](./request-classification-and-flows.md) |

> **Cập nhật 2026-10-05 (sau quyết định của người yêu cầu):** mục 4, 5, 6 đã viết lại theo bốn quyết định: (1) tách `request-service` riêng; (2) Plan và Phase dùng lại Task với `type` mới; (3) xây Approval tổng quát; (4) backlog là view tính toán hiển thị ở frontend, không phải status lưu ở backend. Mục 1 đến 3 giữ nguyên là hiện trạng.

> **Đính chính so với [gap-analysis](./request-to-task-pipeline-gap-analysis.md)** (viết từ docs): (1) Task backlog: docs CR-TG-001 nói có 7 status gồm `backlog` và `todo`, nhưng **code backend và DB không có**. (2) Docs v5 nói chưa có MCP service, nhưng `mcp-service` và các tool pack đã tồn tại trong repo. Hai điều này thay đổi phần "cần xây" bên dưới.

## 1. Trả lời ngắn

**Orca chưa có hệ thống quản trị Request, Solution, Plan hay Phase.** Không có bảng, entity, RPC hay UI cho bốn khái niệm này. Orca chỉ có **Task** (cây cha-con, phụ thuộc, thực thi bằng agent) và các mảnh hạ tầng dùng lại được: liên kết nguồn Jira/GitHub, AI chia task, cổng quyết định, MCP, thông báo, phân quyền.

## 2. Orca đang có (đã xác nhận trong code)

### 2.1 Task (`task-service`)

| Có | Chi tiết |
|---|---|
| Entity Task | Cha-con (`ParentID`), cạnh phụ thuộc (`task_edge`), `Type` (`task|bug|feature|epic`), `Priority`, `Labels`, `AIContext`, `AIPlanJSON`, `PromptTemplate`, người giao, người sở hữu, `ReporterID`, estimate, `ProgressPercent`, `TaskNumber` theo project, `PRURL`. |
| Trạng thái | `open`, `blocked`, `in_progress`, `review`, `done`, `cancelled`. DB có `CHECK` đúng 6 giá trị. **Không có `backlog` hay `todo`.** |
| Nguồn ngoài | `task_sources`: `jira|linear|github|gitlab` + `ref` + `url` + `site`, idempotent theo project. |
| AI chia task | `AIDecompose` trả về `SubtaskProposal` (chưa lưu, có phụ thuộc theo chỉ số, estimate, prompt template); `AIApply` lưu thành Task thật kèm cạnh. Luồng "đề xuất, người xem, rồi mới lưu" đã đúng mẫu cần cho Solution và Plan. |
| Phân quyền | `Grant` theo user/team/role/everyone, kế thừa theo cây, link chia sẻ công khai. |
| Bình luận | `task_comment`. |
| Thực thi | `ExecuteTask` rẽ 3 engine (agent đơn, orchestration, workflow). Claim nguyên tử, lease, phục hồi task kẹt, `ReportTaskExecutionResult` chống callback cũ. Chi tiết: [CR-TG-008](../../crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md). |
| Tính toán DAG | `topological_waves`, `critical_path`, tiến độ cascade từ subtask. |
| Outbox sự kiện | `outbox` cho task. |

### 2.2 Orchestration (`orchestration-service`)

| Có | Chi tiết |
|---|---|
| Điều phối nhiều agent | `OrchestrationTask` (pending, ready, dispatched, completed, failed, blocked), `DispatchContext`, `CoordinatorRun`, heartbeat, circuit breaker. |
| **Decision gate** | `DecisionGate` (pending, resolved, timeout; `Question`, `Options`, `Resolution`). Use case `CreateGate`, `ResolveGate`, `ListPendingDecisionGates`. Phát event `orca.orchestration.decision_gate.opened`. HTTP: `POST /v1/orchestration/gates`, `POST /v1/orchestration/gates/{id}/resolve`. |
| Giới hạn của gate | Gắn với `DispatchContext` của một task điều phối, không phải thực thể duyệt tổng quát. **Không tìm thấy UI frontend** (`frontend/src` chỉ khớp ở `workflow-types.ts` và test). |

### 2.3 Các dịch vụ liên quan

| Dịch vụ | Dùng được cho |
|---|---|
| `issue-tracking-service`, `issue-status-sync`, `scm-integration-service` | Đọc issue Jira, đồng bộ trạng thái ngược (To Do → In Progress → In Review → Done), rút khoá issue từ branch và PR. Nền cho nguồn Jira và GitHub. |
| `mcp-service` + `api-gateway/.../mcpserver` | Đã tồn tại: phiên, OAuth client, consent, approval, kill switch, audit, policy. Tool pack có `task.create`, `task.list`, `task.get`, `task.update`, `task.execute`, `task.aiDecompose`, `task.aiApply`, `task.createFromSource`, `task.addEdge`, `workflow.*`, `terminal.*`, `project.*`. Agent bên ngoài đã có thể tạo và chạy task qua MCP. **Chưa có** MCP làm *nguồn Request* hay tool `request.*`. |
| `workflow-service` | Template và execution, có thực thể `Approval` (pending, approved, rejected) nhưng là duyệt template, không phải cổng của bước chạy. Step `approval` ở frontend không có backend ([BACKLOG-027](../../backlog/BACKLOG-027-workflow-approval-step-type-removal-decision.md)). |
| `notification-service` | Đã subscribe event gate được mở. Dùng được cho "chờ duyệt". |
| `automation-service` | Trigger và hành động tự động. Có thể kích hoạt bước tiếp theo. |
| `project-service` | Project, worktree, `linked_issue_*`, `jira_project_key`, `jira_site_id`. |
| `auth-service`, `tenant-service` | Tenant, user, team, role, audit log. |

### 2.4 Frontend (`components/task/`)

`TaskBoardView`, `TaskTreeView`, `TaskDAGView`, `TaskGraph`, `TaskDetail`, `TaskCreateDialog`, `TaskAIDecompose`, `TaskPromptEditor`, `TaskAccessPanel`, `TaskComments`, `TaskSourceBadge`, `TaskDispatchStatusPanel`, `TaskMergeDialog`, `AttachWorkflowTemplateAction`. Không có component nào cho Request, Solution, Plan, Phase hay cổng duyệt.

> Frontend khai báo `TaskStatus` có `backlog` (cột Board, badge, màu DAG) nhưng backend không nhận giá trị này. Cần xác nhận cách hai bên khớp nhau trước khi dùng `backlog` cho "Task backlog".

## 3. Hiện trạng theo thực thể

| Thực thể cần quản trị | Có chưa | Thứ gần nhất đang có |
|---|---|---|
| **Request** | Chưa | `task_sources` (một dòng liên kết issue ngoài với task), `Task.Type`/`Labels`. |
| **Request type + phân loại** | Chưa | `Task.Type` (`task|bug|feature|epic`) là phân loại thủ công, không có AI, không có xác nhận. |
| **Solution** (nhiều phương án, duyệt) | Chưa | File spec markdown trong worktree (BL-TG-05); `Task.AIPlanJSON`. |
| **Plan** | Chưa | `AIDecompose`/`AIApply` (chia task); `Task.AIPlanJSON`. |
| **Phase** | Chưa | Không có (sẽ là Task `type=phase`). Các nhãn `phase:*` chỉ là pha spec → code của một task. `topological_waves` là nhóm chạy song song, không phải Phase có cổng duyệt. |
| **Task** | **Có** | Đầy đủ, xem 2.1. |
| **Task backlog / Execute backlog** | Chưa | Không có status `backlog` ở backend (và đã chốt không thêm); sẽ là view tính toán. Task thất bại về `previous_status`. |
| **Request backlog + lý do trả về** | Chưa | Không có. |
| **Cổng duyệt tổng quát** | Một phần | `DecisionGate` (chỉ cho điều phối, không UI). |
| **Lịch sử đổi loại, liên kết Request con** | Chưa | Không có. |

## 4. Cần xây thêm

### 4.1 Kiến trúc đã chốt

```
request-service (MỚI, sở hữu Request, Solution, Approval)
   │  gRPC, không FK chéo service (quy ước hiện có)
   ▼
task-service (giữ nguyên vai trò) — Plan, Phase, Task đều là bản ghi Task
   Request ──▶ Plan (task.type=plan) ──▶ Phase (task.type=phase) ──▶ Task làm việc (task.type=task|bug|feature)
              quan hệ bằng task.parent_id; thứ tự giữa các Phase bằng task_edge
```

| Thực thể | Nơi lưu | Trường chính |
|---|---|---|
| `Request` | `request-service` | `id`, `tenant`, `project_id`, `title`, `body`, `source_provider`, `source_ref`, `source_site`, `type`, `type_source` (`ai|human`), `size`, `urgency`, `confidence`, `status`, `return_reason`, `returned_from_stage`, `plan_task_id` |
| `RequestTypeHistory` | `request-service` | `request_id`, `from`, `to`, `actor`, `reason`, `at` |
| `Solution` | `request-service` | `request_id`, `kind` (`solution|diagnosis|findings|answer`), `options[]`, `chosen_option`, `status` |
| `RequestLink` | `request-service` | `parent_request_id`, `child_request_id`, `reason` (Request con từ `spike`, `question`, `hotfix`) |
| `Approval` | `request-service` | `subject_type`, `subject_id`, `stage`, `status`, `requested_by`, `decided_by`, `decided_at`, `comment`, `due_at` (xem 4.2 mục 3) |
| **Plan** | `task-service`, bản ghi Task `type=plan` | Không có `parent_id`; được `Request.plan_task_id` trỏ tới |
| **Phase** | `task-service`, bản ghi Task `type=phase` | `parent_id` = Plan; thứ tự bằng `task_edge` giữa các Phase |
| **Task làm việc** | `task-service` | `parent_id` = Phase; không đổi |

**Vì sao Plan và Phase dùng lại Task.** Cây cha-con, cạnh phụ thuộc, cascade tiến độ (`RecalculateProgress`), kế thừa quyền theo cây (`Grant`), `topological_waves`, `critical_path` và bình luận đều sẵn có và chạy được ở mọi cấp. Entity riêng sẽ phải làm lại từng thứ đó.

**Hệ quả cần xử lý khi dùng lại Task (đã đọc code, chưa thử):**

| Vấn đề | Căn cứ | Hướng xử lý |
|---|---|---|
| `task_type` bị `CHECK IN ('task','bug','feature','epic')` | migration `0003_task_fields_and_comments` | Migration mới thêm `plan`, `phase`, cho cả Postgres và MySQL. Cập nhật comment ở `domain/task.go` và proto. |
| Plan/Phase lẫn vào Board, Tree, danh sách Task | `ListTasks` hiện trả mọi task | Lọc theo `type` ở `ListTasks` và frontend; mặc định ẩn `plan`, `phase` khỏi Board. |
| `in_progress` chỉ `ExecuteTask` được đặt | `ErrCannotSetInProgress` trong `domain/task.go` | Trạng thái của Plan/Phase suy ra từ con (qua cascade) hoặc do `request-service` đặt qua đường nội bộ; chưa chốt, cần thiết kế. |
| `ExecuteTask` trên task có con sẽ chạy engine điều phối cho cả cây con | `selectEngine` (CR-TG-005) | Có thể tận dụng: chạy một Phase là chạy Engine 2 trên cây con. Cần xác nhận hành vi và chặn chạy trực tiếp Plan. |
| `TaskNumber` (`#TG-N`) cấp theo project cho mọi task | `task.go` | Plan/Phase tiêu tốn số. Quyết định bỏ qua hay chấp nhận. |
| Quyền | `Grant` kế thừa theo cây | Quyền Plan kế thừa xuống Phase và Task, phù hợp. Quyền duyệt (Approval) là khái niệm riêng ở `request-service`. |

### 4.2 Backend

**`request-service` (mới).** Cấu trúc theo các service hiện có (`cmd`, `internal/{domain,usecase,adapter}`, `migrations/{postgres,mysql}`, `deploy`), đăng ký trong `backend-go/go.work`, proto `orca/request/v1`, đi qua `api-gateway` (kênh WS và HTTP). Migration cho **cả Postgres và MySQL** (`mcp-service` hiện chỉ có Postgres, không dùng làm mẫu ở điểm này).

1. **Máy trạng thái Request** (xem [request-classification-and-flows.md](./request-classification-and-flows.md) mục 3) kèm luật bỏ qua trạng thái theo loại.
2. **Use case:** `CreateRequestFromSource` (Jira, GitHub, MCP), `ClassifyRequest` (gọi AI, trả `{type, size, urgency, confidence, lý do}`), `ConfirmRequestType`, `ChangeRequestType`, `GenerateSolution` (nhiều phương án), `GeneratePlan` (tạo task `plan` và `phase` qua `task-service`), `ReturnToBacklog`, `ReopenRequest`, `SpawnChildRequest`.
3. **Approval tổng quát (đã chốt xây mới).**
   - Bảng `approvals`: `subject_type` (`request_type|solution|findings|answer|plan|phase|task_list|pre_deploy`), `subject_id` (với `plan`, `phase` là id của task bên `task-service`), `status` (`pending|approved|rejected|cancelled|expired`), người yêu cầu, người quyết định, `comment` bắt buộc khi từ chối.
   - API: `RequestApproval`, `Approve`, `Reject`, `ListPending` (theo người duyệt), `Get`.
   - Phát `approval.requested`, `approval.decided` qua outbox để `notification-service` thông báo.
   - Quyền duyệt theo vai trò hoặc team, tái dùng mô hình của `auth-service` và `tenant-service`.
   - **`DecisionGate` giữ nguyên** cho câu hỏi của agent điều phối. Phiên bản đầu không hợp nhất hai thứ; hộp duyệt chỉ hiện `Approval`.
4. **Phản hồi ngược.** Consumer của outbox task (`task-service`) cập nhật Phase, Plan, Solution, Request khi task xong hoặc lỗi; đồng bộ trạng thái Jira theo Request (qua `issue-status-sync`).
5. **AI.** Prompt và schema đầu ra cho phân loại, Solution, Chẩn đoán, Plan, Phase. `AIDecompose` là khuôn mẫu "đề xuất rồi mới lưu".
6. **Sự kiện** qua outbox: `request.created`, `request.classified`, `request.returned`, `request.completed`, cùng hai sự kiện approval ở trên.
7. **MCP.** Thêm tool `request.*`, `solution.*`, `approval.*` vào tool pack; mọi tool ghi qua chính sách và approval của `mcp-service`. Nguồn MCP cho Request nghĩa là agent ngoài gọi `request.create`.
8. **Loại đặc thù.** `performance` cần đo baseline và đo lại; `ops_request` cần runbook và rollback; `spike`/`question` cần chạy agent không cần worktree. Chưa thấy trong code.

**`task-service` (thay đổi nhỏ).** Mở rộng `task_type` (xem bảng ở 4.1), lọc `ListTasks` theo `type`, chặn chạy trực tiếp task `plan`, bảo đảm cascade tiến độ chạy qua ba cấp, cung cấp truy vấn cho view backlog (xem 4.5).

### 4.3 Frontend

| Màn hình | Nội dung |
|---|---|
| **Danh sách Request + Request backlog** | Lọc theo loại, trạng thái, nguồn; hiện lý do trả về |
| **Chi tiết Request** | Nội dung gốc, loại do AI đề xuất (kèm confidence, lý do) và nút xác nhận/sửa, lịch sử đổi loại, Request con |
| **Xem và duyệt Solution** | So sánh nhiều phương án, chọn, duyệt, từ chối kèm lý do |
| **Xem và duyệt Plan, Phase** | Cây Plan → Phase → Task, duyệt theo cấp |
| **Hộp duyệt chờ xử lý** | Gom mọi cổng đang chờ (loại, Solution, Plan, Phase, trước deploy) |
| **Task backlog / Execute backlog** | Gom task theo Plan và Phase |
| **Sơ đồ tiến trình Request** | Request đang ở trạng thái nào, ai chặn |
| **Trang Tasks hiện có** | Thêm "Tạo Request" từ issue Jira/GitHub thay cho chỉ "Start work" |

Tuân thủ [STYLEGUIDE](../../STYLEGUIDE.md) và dùng shadcn primitives. Cần hỗ trợ cả chế độ SSH và remote theo AGENTS.md.

### 4.4 Nguồn Request

- **Jira, GitHub:** mở rộng `task_sources` hoặc tạo `request_sources`; thêm bước "Tạo Request" ở trang Tasks. Dùng issue type, label làm gợi ý phân loại.
- **MCP:** xem 4.2 mục 10.
- Cân nhắc nhập thủ công và webhook (Jira/GitHub tự đẩy Request vào).

### 4.5 Backlog là view tính toán, hiển thị ở frontend

> **Cách hiểu quyết định (4):** backlog không phải status lưu ở backend (DB tiếp tục chỉ có 6 status), mà là các view tính từ dữ liệu có sẵn và hiển thị chi tiết ở frontend. Nếu bạn muốn khác, cho tôi biết.

Backend cung cấp một truy vấn đọc cho mỗi view (đặt ở `request-service`, gọi `task-service` để lấy task), frontend chỉ hiển thị. Không cho frontend tự ghép từ nhiều RPC.

| View | Điều kiện | Gom theo | Cột hiển thị |
|---|---|---|---|
| **Request backlog** | `Request.status = request_backlog` | Lý do trả về | Request (kèm link Request gốc), nguồn (Jira/GitHub/MCP), loại, `returned_from_stage`, lý do, người trả, thời điểm, nút mở lại hoặc hủy |
| **Task backlog** (theo planning) | Task làm việc dưới một Plan chưa có Approval `approved`, hoặc Plan chưa chia Phase | Plan | Task, Plan chứa nó, trạng thái Approval của Plan, estimate, phụ thuộc |
| **Execute backlog** | Task làm việc dưới Phase đã `approved`, status `open` hoặc `blocked`, hoặc task có `execution_link` gần nhất `failed` | Phase | Task, Phase, bị chặn bởi task nào, lý do lỗi gần nhất (từ link), số lần thử, engine |

Việc cần làm ở frontend:

- Màn hình **Backlog** với ba phân đoạn tương ứng ba view, lọc theo Request, Plan, Phase, loại, người được giao.
- **Gỡ hoặc đổi nghĩa `backlog`** trong `TaskStatus` của frontend (cột Board, `TaskStatusBadge`, màu `TaskDAGView`). Hiện frontend khai báo giá trị mà backend không nhận. Cần xác nhận cột Board `backlog` đang hiển thị gì khi backend không bao giờ trả giá trị này.
- Hộp duyệt chờ xử lý gom mọi `Approval` ở trạng thái `pending` của người dùng.

## 5. Dùng lại được gì (để giảm công)

| Cần | Tái dùng |
|---|---|
| Đề xuất rồi mới lưu | Mẫu `AIDecompose` → `SubtaskProposal` → `AIApply` |
| Cây Plan → Phase → Task | Cha-con, `task_edge`, cascade tiến độ, `topological_waves`, `critical_path` của Task (đã chốt) |
| Phân quyền theo cây | `Grant` kế thừa theo cây |
| Thực thi và phản hồi | `ExecuteTask`, `ReportTaskExecutionResult`, lease và recovery |
| Thông báo duyệt | `notification-service` (đã subscribe event gate; thêm event approval) |
| Liên kết nguồn | `task_sources` (ý tưởng idempotent theo project + site) |
| Đồng bộ Jira ngược | `issue-status-sync` |
| Agent ngoài tạo Request | `mcp-service` (approval, kill switch, audit sẵn) |
| Khung service mới | Cấu trúc của các service hiện có trong `backend-go/services/` |

Không tái dùng: `DecisionGate` (giữ cho điều phối agent), step `approval` của workflow (không có backend).

## 6. Quyết định

**Đã chốt (người yêu cầu, 2026-10-05):**

| # | Quyết định | Hệ quả chính |
|---|---|---|
| 1 | Tách `request-service` riêng để quản trị | Service mới, proto, migration hai DB, gateway; `task-service` chỉ đổi nhỏ |
| 2 | Plan và Phase dùng lại Task với `type` mới (`plan`, `phase`) | Migration `CHECK` hai DB, lọc theo type, xử lý `in_progress`, `TaskNumber` (xem 4.1) |
| 3 | Xây `Approval` tổng quát | Bảng và API ở `request-service`; `DecisionGate` giữ riêng |
| 4 | Backlog hiển thị chi tiết ở frontend | View tính toán, không thêm status backend (xem 4.5, **cách hiểu cần xác nhận**) |

**Còn mở:**

1. Trạng thái của task `plan`/`phase` (`in_progress`, `review`, `done`) do cascade suy ra hay do `request-service` đặt.
2. `TaskNumber`: Plan và Phase có tiêu tốn số `#TG-N` hay được bỏ qua.
3. Cột Board `backlog` hiện tại của frontend: gỡ hay đổi nghĩa.
4. Hợp nhất hộp duyệt với `DecisionGate` ở phiên bản sau hay không.
5. Thứ tự triển khai gợi ý: (a) `request-service` + Request + phân loại + xác nhận (Approval loại `request_type`); (b) Solution + duyệt; (c) mở rộng Task (`plan`, `phase`) + sinh Plan, Phase + duyệt; (d) vòng phản hồi ngược + view backlog; (e) nguồn MCP và các loại đặc thù. UI sau khi dữ liệu và RPC chạy.
6. Bốn điểm cũ trong [request-classification-and-flows.md](./request-classification-and-flows.md) mục 7 (bug size L, cổng hotfix, ngưỡng confidence, loại bổ sung).

## 7. Rủi ro

- Docs và code lệch nhau ở ít nhất hai chỗ (status `backlog`, MCP service). Cần đối chiếu code trước khi tin docs ở các phần khác.
- Mô hình mới chạm nhiều service (task, orchestration, gateway, notification, mcp, frontend) và hai DB (Postgres, MySQL).
- Dùng lại Task cho Plan và Phase làm Task có thêm hai loại "đặc biệt": mọi nơi giả định Task là việc làm được (Board, thống kê, `ExecuteTask`, thông báo) cần rà lại.
- `DecisionGate` chưa có UI nên chưa có bằng chứng nó chạy được với người dùng thật.
- Luồng Jira chưa chạy trên Jira thật (`task_sources` trên server dev có 0 dòng), nên nền nguồn Request còn chưa kiểm chứng.
- Các loại `spike`, `question`, `performance`, `ops_request` đòi hỏi năng lực agent mà docs chưa mô tả.
