# Change Requests v7 — Workflow Engine: định nghĩa tuỳ ý và chạy trên Orca

> **Mục tiêu:** cho phép người dùng **tự định nghĩa bất kỳ workflow nào** từ các khối dựng có sẵn (bước, rẽ nhánh, dữ liệu, cổng duyệt của người, workflow con, trigger) và **chạy nó đáng tin cậy** trên Orca: có điều khiển lỗi, tạm dừng/huỷ thật, quan sát theo từng bước, phiên bản hoá định nghĩa.
>
> **Nguyên tắc bất biến của series:** các CR **không gắn với một quy trình nghiệp vụ cụ thể nào**. Mọi ví dụ trong tài liệu chỉ để minh hoạ cơ chế (bước A, bước B, cổng duyệt). Nếu một yêu cầu chỉ phục vụ một quy trình riêng thì không thuộc series này; nó phải được diễn đạt bằng cách cấu hình trên các khối dựng chung.
>
> **Phạm vi:** `backend-go/services/workflow-service` (domain, usecase, executor, proto, migration), `api-gateway` (`wscompat` kênh `workflow.*`), `agent` (RPC mà bước gọi), `automation-service` và `task-service` (nơi khởi chạy workflow), `frontend` (trình soạn, màn hình chạy).
>
> **Trạng thái toàn series:** 📝 Đề xuất, chưa triển khai. Viết từ khảo sát code ngày 2026-10-08; **chưa chạy hệ thống, chưa viết code**. Người triển khai phải đọc lại code trước khi làm vì docs và code từng lệch nhau (mục 3).
>
> **Quan hệ với tài liệu có sẵn:** mở rộng, không thay thế, [`docs/crs/v4/workflow/`](../../v4/workflow/) (CR-WF-001..007), [BL-WF-01..03](../../../logic/workflow-orchestration/), [ADR-009](../../../adrs/v1/ADR-009-workflow-dag-orchestration.md), [ADR-024](../../../adrs/v2/ADR-024-dual-workflow-engines-migration.md), BACKLOG-021/026/027. CR nào trùng phạm vi sẽ ghi rõ "thay thế một phần" và dẫn CR cũ.

## 1. Danh sách CR

| CR | Tên | Ưu tiên | Effort | Phụ thuộc |
|---|---|---|---|---|
| [CR-WFD-001](./CR-WFD-001-definition-schema-v2.md) | Lược đồ định nghĩa workflow v2: điều khiển bước, kiểm tra khi lưu, phiên bản bất biến | 🔴 P0 | Large | — |
| [CR-WFD-002](./CR-WFD-002-control-flow.md) | Điều khiển luồng: rẽ nhánh thật (`when`), trạng thái `skipped`, workflow con, lặp có giới hạn, ngôn ngữ biểu thức | 🔴 P0 | Large | 001, 003 |
| [CR-WFD-003](./CR-WFD-003-data-and-artifacts.md) | Dữ liệu và artifact: input có kiểu, output có kiểu, biến chạy, artifact có phiên bản | 🔴 P0 | Large | 001 |
| [CR-WFD-004](./CR-WFD-004-human-gate.md) | Cổng của người: bước `gate`, trạng thái `waiting`, phân quyền, hết hạn, tiếp tục từ cổng | 🟠 P1 | Large | 001, 003, 006 |
| [CR-WFD-005](./CR-WFD-005-step-executors-and-registry.md) | Executor bước: sổ đăng ký loại bước, bước `action`/`mcp_call`/`sub_workflow`, cấp worktree cho bước agent | 🟠 P1 | Large | 001, 003 |
| [CR-WFD-006](./CR-WFD-006-execution-lifecycle.md) | Vòng đời chạy: tạm dừng/huỷ thật, retry, timeout, idempotency, giới hạn đồng thời | 🔴 P0 | Large | 001 |
| [CR-WFD-007](./CR-WFD-007-triggers.md) | Trigger: thủ công có input, lịch, sự kiện, webhook, workflow hoàn tất | 🟠 P1 | Medium | 001, 003 |
| [CR-WFD-008](./CR-WFD-008-observability-and-api.md) | Quan sát và API: stream sự kiện, trạng thái từng bước, kênh `wscompat` còn thiếu | 🔴 P0 | Medium | 006 |
| [CR-WFD-009](./CR-WFD-009-frontend-editor-and-run-views.md) | Frontend: trình soạn, form input lúc chạy, theo dõi trực tiếp, cổng duyệt, lịch sử phiên bản | 🟠 P1 | Large | 001–008 |
| [CR-WFD-010](./CR-WFD-010-security-and-governance.md) | An toàn và quản trị: quyền, secret, SSRF, hạn mức, audit, feature flag | 🔴 P0 | Medium | 001 |
| [CR-WFD-011](./CR-WFD-011-migration-and-compatibility.md) | Di trú và tương thích: template v1, engine TS/Go, task engine 3, automation | 🟠 P1 | Medium | 001 |

Thứ tự khuyến nghị: 001 → 010 (song song, nền) → 003 → 006 → 002 → 008 → 005 → 004 → 007 → 011 → 009.

## 2. Hiện trạng đã xác nhận (khảo sát 2026-10-08)

Đường dẫn tính từ gốc repo. `WSI` = `backend-go/services/workflow-service/internal`.

| Hạng mục | Thật sự có gì | Bằng chứng |
|---|---|---|
| Mô hình bước | `Step{id,type,config(raw JSON),dependsOn[]}`; **không có** trường retry, timeout, onError, guard | `WSI/domain/dag.go:39-44` |
| Loại bước | Tập **đóng, biên dịch sẵn**: `agent, shell, notification, webhook, condition, cleanup_worktrees, action, parallel, commit_push` | `WSI/domain/step.go:19-55` (`StepType.Valid`) |
| Lập lịch | Kahn → **wave**; trong wave chạy song song (pool 10); wave N+1 chờ cả wave N; bước nào lỗi thì cả execution lỗi | `WSI/usecase/execute.go`, `wave_dispatcher.go:73,84-91,212-230` |
| Retry / `continueOnError` | **Không có** trong backend (frontend có khai báo `continueOnError`, `timeout` nhưng backend bỏ qua) | `grep` toàn `WSI` chỉ ra một dòng chú thích; `frontend/src/shared/workflow-types.ts:44-45` |
| Nội suy dữ liệu | Có: `{{now()}}`, `{{project.id}}`, `{{user.id}}`, `{{outputs.<stepId>.<path>}}`, `{{<inputKey>}}`; token không giải được **để nguyên chữ, không báo lỗi**; output chỉ ghi khi là JSON object | `WSI/domain/interpolation.go:33-73`, `wave_dispatcher.go:262-311` |
| `condition` | Chỉ `==`/`!=` với `&&`/`||`, không ngoặc, không số; kết quả `{result:bool}` **không ai đọc** nên không rẽ nhánh | `WSI/domain/condition.go:8-35`, `wave_dispatcher.go` (không có tham chiếu) |
| Trạng thái | Execution: `pending, running, paused, completed, failed, cancelled`. Bước: `pending, running, completed, failed` (CHECK ở migration), **không có `skipped`/`waiting`** | `WSI/domain/execution.go:15-20`, `migrations/postgres/0004` |
| Tạm dừng/huỷ | Chỉ **đổi cột status**; dispatcher không kiểm tra `paused`; resume không chạy lại | `usecase/pause_execution.go`, `resume_execution.go:28-55` |
| Stream sự kiện | `StreamExecutionEvents` khai báo trong proto **nhưng chưa implement** | `WSI/adapter/grpc/server.go` (embed `Unimplemented…`) |
| `action` | Executor rỗng, luôn lỗi `ErrNoActionHandlerRegistered` | `WSI/adapter/stepexecutors/action.go:13-21` |
| Cổng duyệt | `Approval` hiện chỉ cho việc **công bố template** lên company/public, không gắn với bước | `WSI/domain/approval.go:41-55` |
| Phiên bản template | Tăng bộ đếm, **ghi đè tại chỗ**, không lưu lịch sử; UI gửi `expectedVersion=0` nên khoá lạc quan vô hiệu | `WSI/domain/template.go:54-95`, `frontend/.../hooks/useWorkflow.ts:70-82` |
| Khởi chạy | Thủ công qua `workflow.execute` (**không chuyển `inputs_json`**); gắn vào task (không input); `automation-service` chỉ chạy **bước lẻ** | `wscompat/channels_workflow.go:47`, `task-service/.../workflow_executor.go:34-41`, `automation-service/.../execute_automation_chain.go:220-229` |
| Giới hạn phía agent | `agent.execPrompt` mặc định 5 phút, tối đa 15; `shell.exec` tối đa 5 phút; workflow-service **không truyền `timeoutMs`** | `agent/src/relay/agent-print-mode-exec.ts:30-32`, `shell-agent-extensions.ts:86-88` |
| Sự kiện | Outbox → NATS: `orca.workflow.step.completed/failed`, `execution.completed/failed`; payload **không chứa output** | `wave_dispatcher.go:288-290`, `execute.go:194-197` |
| UI | Danh sách bước dạng form + đồ thị chỉ đọc; thấy trạng thái cấp execution (poll 4 s), không thấy cấp bước; không có form input, không có cổng duyệt | `frontend/.../components/workflow/*` |

## 3. Điểm lệch docs ↔ code cần biết trước khi làm

| Tài liệu nói | Code thật | Hệ quả |
|---|---|---|
| README workflow-service: truyền output giữa bước "chưa implement" | Đã có `domain.Interpolate` | README lỗi thời, CR-WFD-003 xây trên cái đã có |
| CR-WF-003 mô tả bước `action` | Không có handler nào được đăng ký | `action` hiện vô dụng; CR-WFD-005 giải quyết |
| README: timeout bước mặc định 30 phút | Không có mã thực thi; trần thật do agent (5/15 phút) | CR-WFD-006 định nghĩa lại và truyền xuống agent |
| BACKLOG-021: streaming bị chặn | Đã có sự kiện NATS và cột `step_executions` | Backlog có thể đã cũ; CR-WFD-008 xử lý |
| Frontend có kiểu `approval` | Backend không có | CR-WFD-004 trả lời BACKLOG-027 bằng cách cung cấp bước `gate` thật |

## 4. Quyết định đề xuất (cần xác nhận khi duyệt)

| # | Quyết định | Hệ quả |
|---|---|---|
| D1 | **Giữ `workflow-service` làm nơi duy nhất định nghĩa và chạy workflow**; không viết engine mới | Mọi CR sửa trong service này |
| D2 | **Định nghĩa là dữ liệu có phiên bản bất biến**; bản chạy ghim một `definition_version` | CR-WFD-001, 011 |
| D3 | **Mở rộng bằng sổ đăng ký loại bước** thay vì `switch` biên dịch cứng; loại bước mới không cần sửa lõi lập lịch | CR-WFD-005 |
| D4 | **Biểu thức có kiểu, chạy trên dữ liệu có cấu trúc** (không nội suy chuỗi mù); vẫn giữ `{{…}}` cho tương thích | CR-WFD-002, 003 |
| D5 | **Điểm chờ (gate) là trạng thái hạng nhất** của execution và của bước, không phải trường hợp đặc biệt của bước agent | CR-WFD-004, 006 |
| D6 | **Mỗi bước có quyền kiểm soát lỗi riêng** (retry, timeout, onError) thay cho "một bước lỗi là cả luồng lỗi" | CR-WFD-006 |
| D7 | **Tính năng nằm sau cờ theo tenant, mặc định tắt**; template v1 chạy y như cũ khi cờ tắt | CR-WFD-010, 011 |

## 5. Hợp đồng chung giữa các CR

### 5.1 Định nghĩa v2 (rút gọn, chi tiết ở CR-WFD-001)

```jsonc
{ "schemaVersion": 2,
  "inputs":  [ { "name": "string", "type": "string|number|boolean|json|artifact", "required": true, "default": null } ],
  "steps": [
    { "id": "a", "type": "<registered type>", "config": { },
      "dependsOn": ["..."],
      "when": "<expression>",                       // CR-WFD-002
      "retry": { "max": 0, "backoffMs": 0, "on": ["transient"] },   // CR-WFD-006
      "timeoutMs": 0,                                // CR-WFD-006
      "onError": "fail|continue|skip_dependents",   // CR-WFD-006
      "outputs": [ { "name": "string", "type": "..." } ] } ],
  "outputs": [ { "name": "string", "from": "steps.a.outputs.x" } ] }
```

### 5.2 Trạng thái

| Đối tượng | Giá trị |
|---|---|
| Execution | `pending, running, waiting, paused, completed, failed, cancelled` |
| Bước | `pending, running, waiting, completed, failed, skipped, cancelled` |

`waiting` = đang chờ một tác nhân bên ngoài (người, sự kiện); `skipped` = guard `when` sai hoặc phụ thuộc bị bỏ qua; không tính là lỗi.

### 5.3 Ngữ cảnh biểu thức

`inputs.<name>`, `steps.<id>.status`, `steps.<id>.outputs.<name>`, `run.id`, `run.startedAt`, `project.id`, `user.id`, `now()`. Chi tiết ngữ pháp ở CR-WFD-002.

### 5.4 Mã lỗi (tiền tố `WORKFLOW_`)

`WORKFLOW_DEFINITION_INVALID`, `WORKFLOW_VERSION_CONFLICT`, `WORKFLOW_INPUT_INVALID`, `WORKFLOW_EXPRESSION_INVALID`, `WORKFLOW_STEP_TIMEOUT`, `WORKFLOW_STEP_TYPE_UNKNOWN`, `WORKFLOW_GATE_FORBIDDEN`, `WORKFLOW_GATE_EXPIRED`, `WORKFLOW_RUN_NOT_RESUMABLE`, `WORKFLOW_QUOTA_EXCEEDED`, `WORKFLOW_DISABLED`.

## 6. Điểm còn mở (cần quyết định khi duyệt)

| # | Điểm mở | Mặc định trong series |
|---|---|---|
| O1 | Ngôn ngữ biểu thức: tự viết tập con hay dùng thư viện CEL | **Tự viết tập con** (so sánh, `&&`, `||`, `!`, ngoặc, số, `in`, `exists`); thêm thư viện cần duyệt riêng |
| O2 | Nơi lưu artifact lớn | **Nội tuyến ≤ 256 KiB trong DB; lớn hơn chỉ lưu tham chiếu tới tệp trên dev server kèm hash** (không sao chép) |
| O3 | Hai dialect DB | **Postgres và MySQL** theo `common/dbcapability`, như `workflow-service` hiện có |
| O4 | Cổng của người dùng chung `request-service` hay tự có | **Tự có trong `workflow-service`**, thiết kế để một `GateDecider` bên ngoài cắm vào sau; không phụ thuộc `request-service` |
| O5 | Lặp (`foreach`) | **P2**, giới hạn số vòng và đồng thời; không làm ở MVP |
| O6 | Chạy lại từ bước lỗi (resume-from-failure) | **Có**, tạo run mới ghim cùng phiên bản và tái dùng output đã xong |
| O7 | Hạn mức mặc định | Chỉ là tham số cấu hình; số cụ thể nêu ở CR-WFD-010, cần xác nhận |

## 7. Chưa kiểm chứng

- Hình dạng config của bước `webhook`, mức hoàn thiện chống SSRF (README ghi là "stub").
- Có hay không nơi nào kiểm tra config theo từng loại bước lúc lưu template (chỉ thấy `dag.Validate`).
- Các trigger sự kiện của `automation-service` có chạy trọn vẹn đầu-cuối không.
- Trạng thái triển khai thực tế của CR-REQ-009/010 (approval chung) trong `request-service`.
- Hành vi thật của huỷ/tạm dừng với goroutine đang chạy (chỉ suy ra từ việc không thấy tham chiếu).
