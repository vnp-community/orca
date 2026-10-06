# CR-REQ-026: OpenSpec solution engine sau giao diện `SolutionEngine`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-026 |
| **Tên** | Giao diện `SolutionEngine` ở `request-service` với hai cài đặt `native` và `openspec`; cờ cấp project `solution_engine`; nhánh git riêng cho proposal; parse `tasks.md` thành `PlanProposal`; `openspec validate` và `archive`; cổng kiểm tra điều kiện dev server |
| **Loại** | Feature (tích hợp công cụ ngoài) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-007 (`GenerateSolution`, `analysis_runs`), CR-REQ-008 (đường `agent.execPrompt`, kiểm sau chạy), CR-REQ-012 (`PlanProposal`, `GeneratePlan`), CR-REQ-013 (consumer `ReportTaskOutcome`, `request.completed`), CR-REQ-027 (schema chuẩn và bản chiếu Markdown/YAML), CR-REQ-033 (agent: chế độ chỉ đọc, làm rõ `worktreePath`; chưa đọc được, xem Q1) |
| **Mở khoá** | Không CR nào bắt buộc; CR-REQ-021 hiển thị nhánh proposal (tùy chọn) |
| **Tác động** | `request-service` (domain, usecase, adapter grpcclient, postgres, mysql, migration, proto); không đổi `task-service`, không đổi agent; sửa CR-REQ-002, 003, 007, 008, 012, 013, 016, 017, 024, 025 (mục 9) |

## 1. Bối cảnh và vấn đề

1. CR-REQ-007 sinh Solution bằng `ai.complete`: chỉ có ngữ cảnh văn bản, không đọc repo, và CR-REQ-012 sinh Plan cùng cách. Chất lượng bị giới hạn ở mức thiết kế (CR-REQ-007 mục 6).
2. Nghiên cứu `openspec-and-ai-tooling-integration.md` đề xuất dùng OpenSpec (đề xuất `proposal.md`, `design.md`, `tasks.md`, delta spec trong `openspec/changes/<id>/`, rồi validate, apply, archive) làm một cách sinh nội dung khác. Trình tự của nó khớp các cổng duyệt của Request flow, tài liệu nằm trong git, và `agent.execPrompt` đọc được repo.
3. Repo hiện chưa có gì về OpenSpec: không có thư mục `openspec/`, không có lệnh `openspec` trên máy đã khảo sát. Mô tả OpenSpec trong CR này (cấu trúc thư mục, lệnh `validate`, `archive`, cờ dòng lệnh) là hiểu biết chung, **chưa đối chiếu với bản định dùng**; mọi tên lệnh và cờ dưới đây cần kiểm lại khi triển khai.
4. Đã đọc code agent: `agent.execPrompt` bắt buộc `prompt` và `worktreePath`, chỉ chạy `claude`, không có chế độ chỉ đọc (`agent/src/relay/agent-print-mode-exec.ts`); `agent.exec` nhận `{binary, args, cwd, stdin, env, timeoutMs}` với trần 5 phút (`agent-rpc-dispatch-agent-exec.ts`); `fs.readFile`, `fs.stat`, `fs.glob`, `fs.writeFile`, `fs.mkdir` có sẵn (`agent-rpc-dispatch-fs.ts`). `infra-fleet-service` đã có tiền lệ kiểm tra điều kiện dev server bằng `shell.exec` (`usecase/check_dev_server_preflight.go`: git, node, đĩa, cổng, `gh`).
5. Quyết định của nghiên cứu: giữ luồng Request, giữ agent ở dạng chung, mọi logic OpenSpec nằm ở `request-service` sau một giao diện. CR này hiện thực hóa quyết định đó.

## 2. Giải pháp đề xuất

### 2.1 Giao diện `SolutionEngine` (`internal/usecase/solution_engine.go`, mới)

```go
type SolutionEngine interface {
    Name() EngineName // "native" | "openspec"
    Preflight(ctx context.Context, p ProjectRef) (PreflightReport, error)
    GenerateAnalysis(ctx context.Context, in AnalysisInput) (AnalysisOutput, error) // kind solution
    GeneratePlan(ctx context.Context, in PlanInput) (PlanProposal, PlanRaw, error)
    OnRequestCompleted(ctx context.Context, in CompletionInput) error // native: no-op
}
```

- `nativeEngine` bọc mã của CR-REQ-007 (`run_solution_generation`, `solution_prompt`, `ai_completion_relay`) và CR-REQ-012 (`PlanGenerator`). Không đổi hành vi; chỉ đổi chỗ gọi: use case của hai CR gọi `engine.GenerateAnalysis` và `engine.GeneratePlan`.
- `openspecEngine` là cài đặt mới (2.3 đến 2.6). Khi kết quả đã có, đường còn lại là chung: `SolutionOptions.Validate` (CR-REQ-007), `ValidateProposal` (CR-REQ-012), kiểm ngữ nghĩa (CR-REQ-027 mục 2.8), `OpenApproval`, `TransitionRequest`, outbox. Máy trạng thái, `Approval`, `solutions`, `CreatePlanTree` không đổi.
- `openspec validate` chỉ kiểm cấu trúc của OpenSpec; nó **bổ sung** chứ không thay `ValidateProposal` (nghiên cứu nêu "thay một phần"; CR này quyết định không thay, vì validate của công cụ ngoài không biết giới hạn Phase, nhãn, kích thước của Orca).

### 2.2 Cờ cấp project `solution_engine` và hồ sơ theo loại

- Bảng `project_engine_settings` (mới, `request-service`): `tenant_id`, `project_id`, `solution_engine` (CHECK `native|openspec`, mặc định không có dòng nghĩa là `native`), `openspec_min_version` NULL, `updated_by`, `updated_at`, `version`; khóa chính `(tenant_id, project_id)`; Postgres RLS `tenant_isolation`, MySQL lọc `tenant_id` ở mọi truy vấn. RPC `GetProjectEngineSettings`, `SetProjectEngineSettings` (chỉ `role=admin`); đổi cờ ghi audit.
- **Ghim theo Request:** cột mới `requests.solution_engine` (`TEXT` / `VARCHAR(10)`, NULL) được đặt một lần khi Request vào `analyzing` hoặc `planning` lần đầu (tại `type_confirmed`); mọi lần sinh sau của Request dùng giá trị đã ghim. Đổi cờ project không ảnh hưởng Request đang chạy.
- Hồ sơ theo loại (`OpenSpecProfileFor(type)`, mã Go tĩnh, thêm vào `FlowDefinition` của CR-REQ-003):

| Loại | Hồ sơ | Bước dùng OpenSpec |
|---|---|---|
| `change_request`, `refactor` | `full` | Solution (`proposal.md`, `design.md`, delta spec) và Plan (`tasks.md`) |
| `bug`, `security`, `performance` | `light` | chỉ Plan: `proposal.md` (từ Chẩn đoán đã duyệt của CR-REQ-008) và `tasks.md`; Chẩn đoán vẫn `agent_readonly` kiểu CR-REQ-008 |
| `task`, `docs`, `ops_request`, `hotfix`, `spike`, `question` | `none` | không dùng, luôn `native` (đúng khuyến nghị nghiên cứu mục 1.2); không tính là lỗi |

Loại `none` ở project bật `openspec` tự chạy `native`; ghi `provenance.generator.kind=native` (CR-REQ-027).

### 2.3 Cổng kiểm tra điều kiện (`EngineReadinessGate`)

`openspecEngine.Preflight` chạy trước mỗi `GenerateSolution` và `GeneratePlan` có `engine=openspec`; kết quả cache 10 phút theo `(dev_server_id, repo_path)`. Dùng `ResolveConnection(project_id)` rồi `Relay`:

| Kiểm tra | Cách làm | Lỗi |
|---|---|---|
| Có kết nối dev server | `ResolveConnection` | `REQUEST_ENGINE_NO_CONNECTION` |
| CLI `openspec` có mặt, đủ phiên bản | `agent.exec` `{binary:"openspec", args:["--version"]}`; so với `REQUEST_OPENSPEC_MIN_VERSION` hoặc `openspec_min_version` | `REQUEST_ENGINE_OPENSPEC_MISSING`, `REQUEST_ENGINE_OPENSPEC_VERSION` |
| Repo đã khởi tạo OpenSpec | `fs.stat` trên `<repo_path>/openspec`; không tự chạy `openspec init` (làm đổi repo) | `REQUEST_ENGINE_OPENSPEC_NOT_INITIALIZED` |
| CLI `claude` có mặt | `agent.exec` `{binary:"claude", args:["--version"]}` | `REQUEST_ENGINE_CLAUDE_MISSING` |
| `claude` đã đăng nhập | **chưa biết lệnh kiểm không tốn token** (Q3). Phương án: lần đầu mỗi dev server chạy một `agent.execPrompt` prompt tối thiểu, đọc `exitCode`; kết quả cache 10 phút | `REQUEST_ENGINE_CLAUDE_NOT_AUTHENTICATED` |
| Git sạch trên repo gốc | `GetStatus` của git-gateway cho `repo_path` (không dùng để chặn: chỉ cảnh báo) | cảnh báo trong `PreflightReport` |

`PreflightReport.failures[]` mỗi mục có `code`, `message`, `fix_hint`. Không đạt thì `REQUEST_ENGINE_PREFLIGHT_FAILED` (FailedPrecondition, kèm danh sách), **không tạo run, không tự lùi về `native`** (kết quả hai engine khác nhau; lùi âm thầm làm khó điều tra). Người dùng chọn tay: `GenerateSolutionRequest.engine_override="native"` (trường mới, số 5; chỉ nhận `native`, chỉ cho người báo cáo hoặc admin). Giá trị đã ghim `requests.solution_engine` cập nhật theo lựa chọn và ghi lịch sử.

Bước kiểm tương tự của infra-fleet (`check_dev_server_preflight.go`) dùng `shell.exec`; CR này dùng `agent.exec` vì cần `binary` và `args` có cấu trúc, không ghép shell (tránh chèn lệnh).

### 2.4 Nhánh git và worktree cho proposal

Sinh giải pháp bằng OpenSpec **ghi file vào repo**, nên không được lẫn vào nhánh làm việc của task.

- Mỗi Request dùng một nhánh `request/<number>-proposal` (ví dụ `request/142-proposal`) và một worktree riêng, tạo bằng git-gateway `CreateWorktree` (đã có, `gitgateway.proto`) với tên nhánh chỉ định và **không** liên kết issue. Việc `CreateWorktree` có phát `worktree.created` làm đồng bộ Jira sang "In Progress" qua `issue-status-sync` cho Request nguồn Jira chưa kiểm chứng (CR-REQ-008 mục 2.3 ghi cùng nguy cơ); phải kiểm trước khi bật.
- Không dùng `repo_path` gốc cho engine này (nghiên cứu mục 1.4 hỏi việc đó có an toàn không; chưa kiểm chứng, nên chọn đường an toàn hơn). Worktree proposal tạo một lần, tái dùng cho lần sinh Solution, Plan và đồng bộ `tasks.md`; xóa bằng `RemoveWorktree` khi Request `completed` sau `archive` hoặc `cancelled`.
- Chỉ Orca commit lên nhánh này, bằng `Commit` của git-gateway, **sau** khi kiểm tra đạt (không commit nội dung chưa qua kiểm). Không có lệnh Git mới: chỉ các RPC git-gateway hiện có; tuân [`git-compatibility.md`](../../../../guides/reference/git-compatibility.md).
- `change_id` do Orca tạo, không do agent chọn: `req-<number>-<slug>` với `slug` là kebab-case ASCII từ tiêu đề, tối đa 40 ký tự, khớp `^req-[0-9]+-[a-z0-9-]{1,40}$`. Đường dẫn duy nhất agent được ghi: `openspec/changes/<change_id>/**`.
- Bảng `openspec_changes` (mới): `id`, `tenant_id`, `request_id` (UNIQUE `(tenant_id, request_id)`), `project_id`, `change_id`, `branch`, `worktree_id`, `base_ref`, `openspec_version`, `status` (CHECK `preparing|ready|archived|abandoned`), `tasks_md_digest`, `tasks_sync_state` (CHECK `in_sync|pending|failed`), `last_synced_at` NULL, `archived_commit` NULL, `created_at`, `updated_at`, `version`. Kiểu cột theo quy ước hai dialect của CR-REQ-002.

### 2.5 Sinh Solution (hồ sơ `full`) và Plan

Chạy trong `analysis_runs` của CR-REQ-007 (lease, heartbeat, vòng quét phục hồi, một run `running` mỗi `(request, kind)`). Thêm cột `analysis_runs.engine` (`TEXT` / `VARCHAR(16)`, mặc định `native`) và giá trị `mode='agent_proposal'` vào CHECK (agent ghi file, khác `complete` và `agent_readonly`). Giới hạn đồng thời cùng `REQUEST_AGENT_READONLY_MAX_PER_PROJECT` (đếm cả hai mode).

**Solution (`GenerateAnalysis`):**
1. `Preflight`; bảo đảm worktree và `openspec_changes` (`status=preparing`).
2. Dựng prompt (`openspec_solution_prompt.go`): chỉ dẫn OpenSpec được **đặt trong chính prompt** (đọc `openspec/AGENTS.md` nếu có, tạo `proposal.md`, `design.md`, delta spec dưới `openspec/changes/<change_id>/`), thay vì dựa vào slash command `/openspec:*` (chưa chắc chạy với `claude --print`, Q4). Request nằm trong khối `<request>` có rào, cắt 12 000 ký tự, như CR-REQ-007 mục 2.4; nhắc rõ đó là dữ liệu. Không đưa `credential_ref` hay bí mật. `design.md` phải chứa vùng `<!-- orca:begin options -->` với khối ```` ```orca-json ```` là `SolutionOptions` v1 (CR-REQ-027).
3. `agent.execPrompt` `{prompt, worktreePath, trustPreset:"default", env:{ORCA_REQUEST_ID,ORCA_PROJECT_ID}, timeoutMs}`; không `full`. Timeout `REQUEST_OPENSPEC_TIMEOUT` mặc định đề xuất 600 giây (trần của agent 15 phút).
4. Kiểm sau chạy: `GetStatus` trên worktree; mọi đường dẫn đổi phải nằm dưới `openspec/changes/<change_id>/`, nếu không `REQUEST_OPENSPEC_OUT_OF_SCOPE_CHANGE`, run `failed`, `Discard` các thay đổi.
5. Đọc về: `fs.readFile` các tệp cố định (`proposal.md`, `design.md`, delta spec bằng `fs.glob`), tối đa 256 KB mỗi tệp; che bí mật (dùng `analysis_secret_redaction.go` của CR-REQ-008); parse theo quy tắc CR-REQ-027 mục 2.7 (chỉ khối `orca-json` và phần mở đầu YAML). Chạy `agent.exec` `openspec validate <change_id>` (cờ chưa kiểm chứng). Hai kiểm tra phải cùng đạt; sai thì thử lại một lần (`attempt=2`) trong cùng worktree với thông điệp lỗi cụ thể, sau đó `failed` với `REQUEST_OPENSPEC_INVALID_OUTPUT`. Lỗi `exitCode != 0` hoặc `timedOut`: `REQUEST_OPENSPEC_AGENT_FAILED` / `REQUEST_OPENSPEC_TIMEOUT`.
6. Đạt: `SolutionOptions.Validate` và kiểm ngữ nghĩa; Orca **viết lại** vùng `options` của `design.md` bằng `RenderArtifact` từ mô hình chuẩn (`fs.writeFile`), để tệp trong repo là bản chiếu của Orca chứ không phải lời kể của agent; `Commit`; ghi `solutions` với `provenance.generator.kind=openspec`, `tool="openspec@<version>"`, `model` rỗng (`agent.execPrompt` không trả model); tiếp tục như CR-REQ-007 bước 5 (Approval, chuyển trạng thái, outbox).
7. Lỗi ở bất kỳ bước: giữ nguyên quy tắc CR-REQ-007 (Request giữ `analyzing`, không tự về backlog, xóa Solution `draft`); worktree giữ lại, đã `Discard` thay đổi chưa commit.

**Plan (`GeneratePlan`, cả hồ sơ `full` và `light`):** như trên với prompt `tasks.md`, đầu vào là Solution `approved` (phương án đã chọn) hoặc Chẩn đoán `approved`. Văn phạm `tasks.md` (vùng Orca, parser chặt `tasks_md_parser.go`):

```
<!-- orca:begin plan request=REQ-142 schema=1 -->
## PH-1 Tên Phase
> Mô tả và tiêu chí xong của Phase (dòng bắt đầu bằng "> ")
- [ ] T1.1 Tiêu đề task [type=feature] [h=2.5] [ac=AC-1,AC-2] [labels=test:regression] [depends=T1.2]
  Mô tả (thụt hai dấu cách)
<!-- orca:end plan -->
```

Hồ sơ không Phase dùng một nhóm `## TASKS`. Ánh xạ: `PH-n` → `PhaseProposal`; `Tn.m` → `TaskProposal` (`depends` thành `depends_on_indices` cùng cha; `ac` thành `satisfies`, CR-REQ-027; `irreversible` thành nhãn `gate:pre_deploy`). Dòng lạ trong vùng, chỉ số lệch, vòng phụ thuộc: `Violation` có số dòng, thử lại một lần. Sau parse: `ValidateProposal` (CR-REQ-012 mục 2.4). `PlanRaw` là nội dung `tasks.md` thô, lưu vào `ai_plan_json` như `raw_ai_response`. COMMIT (kể cả khi người dùng sửa đề xuất ở UI) thì Orca render lại vùng `plan` từ bản đã lưu (`fs.writeFile`, `Commit`). `ID` thật của task chèn thêm dạng chú thích `<!-- orca:task T1.1 id=<uuid> -->` sau COMMIT.

### 2.6 Hai nguồn sự thật, đồng bộ một chiều, archive

- **Orca là nguồn chính** cho trạng thái, nội dung đã duyệt và ID. `tasks.md` và các tệp OpenSpec là bản chiếu.
- `TasksMdSyncer` (consumer của `task_run_outcomes`, CR-REQ-013): khi task có kết quả `succeeded`, đánh `[x]` dòng tương ứng trong vùng `plan` qua `fs.readFile` + `fs.writeFile` rồi `Commit` (gộp theo lô, tối đa mỗi 30 giây mỗi Request). Không bao giờ bỏ tick. Lỗi đồng bộ không ảnh hưởng luồng: `tasks_sync_state=failed`, thử lại theo lần quét kế, cảnh báo ở metric. Đây là best-effort, không nằm trong transaction nào.
- **Trôi (drift):** nếu `tasks_md_digest` của vùng Orca trong repo khác bản Orca vừa ghi (người sửa tay), lần đồng bộ sau ghi đè vùng Orca và ghi `engine_drift_detected` (metric, audit); phần văn bản ngoài vùng không đụng. Không nhập ngược thay đổi tay vào Orca (Q5).
- **Archive:** consumer durable của `orca.request.request.completed` (dedup `processed_events`) chạy `agent.exec` `{binary:"openspec", args:["archive", <change_id>, "--yes"], cwd: worktree_path}` (cờ chưa kiểm chứng), kiểm `GetStatus`, `Commit`, đặt `openspec_changes.status=archived`. Lỗi thì thử lại với backoff tối đa 5 lần rồi `tasks_sync_state=failed` kèm cảnh báo; **không** đổi trạng thái Request (đã `completed`). Request `cancelled`: `status=abandoned`, giữ nhánh, không archive. Nhánh `request/<number>-proposal` không được tự merge hay mở PR; người dùng quyết (Q6).
- Task thực thi **không** đọc `tasks.md` hay `proposal.md` từ repo: bối cảnh task do Orca dựng (CR-REQ-012 mục 2.7), nên worktree của task (nhánh khác) không cần có thư mục `openspec/changes`.

### 2.7 Ranh giới tin cậy, lỗi, quyền, sự kiện

- Nội dung Request, và tệp agent viết ra, đều là dữ liệu không tin cậy: bọc trong khối rào, cắt kích thước, loại ký tự điều khiển, kiểm schema trước khi lưu, che bí mật trước khi lưu và trước khi đưa vào prompt lần sau. `change_id`, đường dẫn đọc và đường dẫn ghi do Orca xác định; không bao giờ lấy đường dẫn từ đầu ra agent. Không `eval`, không chạy lệnh từ nội dung tệp. Việc `fs.stat` có báo liên kết tượng trưng hay không chưa kiểm chứng; nếu không, đọc tệp qua `fs.glob` với mẫu cố định và kiểm đường dẫn thực nằm trong worktree.
- Cả dev server không đáng tin hơn Orca: repo có thể chứa `openspec/AGENTS.md` hay tệp khác làm chỉ dẫn gài; hạn chế bằng kiểm sau chạy ở 2.5 bước 4 và duyệt của người ở mọi cổng.
- Mã lỗi (`apperrors`, tiền tố `REQUEST_`): `REQUEST_ENGINE_PREFLIGHT_FAILED`, `REQUEST_ENGINE_NO_CONNECTION`, `REQUEST_ENGINE_OPENSPEC_MISSING`, `REQUEST_ENGINE_OPENSPEC_VERSION`, `REQUEST_ENGINE_OPENSPEC_NOT_INITIALIZED`, `REQUEST_ENGINE_CLAUDE_MISSING`, `REQUEST_ENGINE_CLAUDE_NOT_AUTHENTICATED` (FailedPrecondition); `REQUEST_ENGINE_OVERRIDE_NOT_ALLOWED` (PermissionDenied); `REQUEST_OPENSPEC_INVALID_OUTPUT`, `REQUEST_OPENSPEC_AGENT_FAILED`, `REQUEST_OPENSPEC_TIMEOUT`, `REQUEST_OPENSPEC_OUT_OF_SCOPE_CHANGE` (ghi ở run, không ném ra RPC, như CR-REQ-007).
- Quyền: `Set/GetProjectEngineSettings` chỉ `role=admin`; còn lại theo quyền của `GenerateSolution` và `GeneratePlan`.
- Sự kiện: không thêm subject mới cho luồng chính. Metric (CR-REQ-024): `request_engine_runs_total{engine,kind,result}`, `request_engine_preflight_failures_total{code}`, `request_openspec_sync_state{state}`, `request_engine_drift_total`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | OpenSpec là một cài đặt sau giao diện, không đổi máy trạng thái | Nghiên cứu mục 2.1: hotfix, spike, question không dùng; đổi phiên bản OpenSpec không được làm vỡ luồng |
| 2 | Không thêm method `openspec.*` vào agent; dùng `agent.exec`, `agent.execPrompt`, `fs.*` | Tránh triển khai lại agent trên mọi dev server và gắn agent với một công cụ |
| 3 | Nhánh và worktree riêng cho proposal, không dùng repo gốc | Sinh giải pháp làm đổi repo; không lẫn với nhánh task; hiệu ứng lên repo gốc chưa kiểm chứng |
| 4 | Không tự lùi về `native` khi preflight lỗi | Hai engine cho kết quả khác nhau; lùi âm thầm làm khó điều tra và so sánh chất lượng |
| 5 | Ghim engine theo Request | Đổi cờ giữa chừng làm một Request có Solution `openspec` và Plan `native` không chủ ý |
| 6 | Orca viết lại vùng bản chiếu từ mô hình chuẩn | Chặn agent làm lệch giữa prose và dữ liệu duyệt |
| 7 | `openspec validate` bổ sung, không thay `ValidateProposal` | Validate của công cụ ngoài không biết quy tắc Orca |
| 8 | Đồng bộ `tasks.md` một chiều, best-effort | Hai nguồn sự thật; đồng bộ hai chiều cần giải quyết xung đột, không đáng ở v1 |
| 9 | Chỉ dẫn OpenSpec nằm trong prompt, không dựa slash command | Slash command với `claude --print` chưa kiểm chứng |

## 4. Tiêu chí chấp nhận

- [ ] Không có dòng `project_engine_settings` thì `change_request` chạy `native` đúng như CR-REQ-007 và 012 (test hồi quy toàn bộ bộ test của hai CR chạy qua `SolutionEngine`).
- [ ] Project bật `openspec`, Request `change_request`: `GenerateSolution` tạo run `engine=openspec`, `mode=agent_proposal`, nhánh `request/<n>-proposal` có `openspec/changes/<change_id>/` đã commit, Solution `proposed` với `provenance.generator.kind=openspec`, Approval `pending`, Request `awaiting_analysis_approval` (đủ trong một transaction phần DB).
- [ ] Loại `none` (`hotfix`, `task`, ...) ở project bật `openspec` chạy `native`, không lỗi, provenance ghi `native`.
- [ ] Thiếu CLI `openspec`, thiếu `openspec/`, `claude` chưa đăng nhập: `REQUEST_ENGINE_PREFLIGHT_FAILED` kèm đúng mã và `fix_hint`, không tạo run, không có worktree mới.
- [ ] `engine_override=native` từ người không phải báo cáo hoặc admin bị `REQUEST_ENGINE_OVERRIDE_NOT_ALLOWED`; override hợp lệ chạy `native` và cập nhật giá trị ghim.
- [ ] Agent ghi file ngoài `openspec/changes/<change_id>/`: run `failed` `REQUEST_OPENSPEC_OUT_OF_SCOPE_CHANGE`, thay đổi bị bỏ, không có commit.
- [ ] `design.md` sai khối `orca-json` hoặc `openspec validate` lỗi: thử lại đúng một lần với thông điệp lỗi, rồi `failed` `REQUEST_OPENSPEC_INVALID_OUTPUT`; không còn Solution `draft`.
- [ ] `tasks.md` hợp lệ cho `change_request` size M parse ra `PlanProposal` có Phase, khớp `ValidateProposal`; dòng lạ trong vùng Orca gây `Violation` có số dòng; văn bản ngoài vùng không đổi kết quả (test với prose chứa checkbox giả).
- [ ] COMMIT Plan viết lại vùng `plan` từ bản đã lưu (kể cả bản người dùng sửa) và chèn `id` task thật.
- [ ] Task `succeeded` làm tick `[x]` đúng dòng sau tối đa một chu kỳ đồng bộ; không bao giờ bỏ tick; sửa tay vùng Orca bị ghi đè và đếm `engine_drift_total`.
- [ ] `request.completed` chạy `openspec archive` đúng một lần dù consumer giao lặp; lỗi archive không đổi trạng thái Request.
- [ ] Đổi `solution_engine` của project sau khi Request đã ghim không đổi engine của Request đó.
- [ ] Prompt không chứa `credential_ref` hay `env` nhạy cảm; Request chứa "bỏ qua mọi chỉ dẫn trước" nằm trong khối `<request>`.

## 5. Kiểm thử

- **Unit (domain, không mạng):** `tasks_md_parser` (bảng đúng và sai, tiếng Việt có dấu, dòng lạ, vòng phụ thuộc), `change_id` sinh từ tiêu đề lạ (emoji, tiếng Việt, rỗng), bảng hồ sơ theo loại, ghim engine, `RenderArtifact` vùng Orca (khớp CR-REQ-027).
- **Unit (usecase):** `Preflight` từng mã lỗi và cache; run thành công, kiểm sau chạy phát hiện ghi ngoài phạm vi, thử lại một lần, đồng bộ tick, drift, archive giao lặp; dùng fake Relay (`agent.exec`, `agent.execPrompt`, `fs.*`), fake git-gateway, fake clock.
- **Integration hai dialect:** `project_engine_settings`, `openspec_changes` (UNIQUE `(tenant_id, request_id)`), `analysis_runs` với `engine` và `mode` mới, chỉ mục một run `running` không đổi.
- **Hợp đồng:** `buf breaking`; mẫu vàng `testdata/openspec/*.md` cho `design.md` và `tasks.md`.
- **Thủ công trên dev server thật (chưa kiểm chứng, bắt buộc trước khi bật cờ):** cài `openspec`, chạy không đổi agent trên một project mẫu, xem agent tạo đúng cấu trúc, `openspec validate` qua, và Jira có nhận `worktree.created` hay không. Bản OpenSpec định dùng chưa được chốt.

## 6. Rủi ro và điểm chưa kiểm chứng

- Mọi hành vi và cờ của OpenSpec (cấu trúc thư mục, `validate`, `archive`, định dạng `tasks.md`) là hiểu biết chung, chưa đối chiếu tài liệu hiện hành. Phiên bản đổi có thể làm vỡ prompt và parser; giảm thiểu bằng `openspec_min_version`, mẫu vàng và `openspec validate` ở CI của engine.
- Agent ghi vào repo của dự án, không có chế độ chỉ đọc; kiểm sau chạy chỉ phát hiện, không ngăn. Thất bại giữa chừng có thể để tệp dở trong worktree (đã `Discard`).
- `CreateWorktree` có thể kích hoạt đồng bộ Jira cho Request nguồn Jira; chưa kiểm chứng (cần đọc `issue-status-sync`).
- `agent.execPrompt` chỉ ghi `stdout`; không trả danh sách file đổi, nên kiểm sau chạy dựa `GetStatus`. CR-REQ-033 có thể bổ sung; chưa đọc.
- Thời gian một run `agent_proposal` (đọc repo, viết nhiều tệp) có thể sát trần 15 phút; `REQUEST_OPENSPEC_TIMEOUT` 600 giây chỉ là đề xuất.
- Worktree proposal tồn tại lâu (đến khi Request kết thúc) làm tăng số worktree và khối lượng dọn dẹp ở dev server; có thể ảnh hưởng quota.
- `agent/` và `desktop/src/relay/` có file trùng tên; chưa xác nhận đồng bộ. CR này không sửa agent nên chỉ liên quan khi CR-REQ-033 sửa.

## 7. Câu hỏi mở

1. CR-REQ-033 chưa tồn tại trong cây `docs/crs/v6` lúc soạn; tham số chế độ chỉ đọc và `worktreePath` do nó định nghĩa cần đối chiếu. CR này không phụ thuộc cứng (không cần chỉ đọc vì chủ ý ghi trong worktree riêng).
2. Bật theo project (đề xuất) hay thêm theo loại Request.
3. Lệnh kiểm `claude` đã đăng nhập mà không tốn token.
4. `claude --print` có chạy được slash command và hook của repo không (chỉ ảnh hưởng nếu muốn dùng lại lệnh `/openspec:*`).
5. Có nhập ngược chỉnh sửa tay vùng Orca không, hay chỉ cảnh báo trôi.
6. Nhánh `request/<n>-proposal` sau `archive`: tự mở PR (qua `GeneratePullRequestFields` của git-gateway) hay để người dùng quyết.
7. Chọn một giữa OpenSpec và Spec Kit (nghiên cứu: chọn một). CR này chỉ làm OpenSpec; giao diện cho phép thêm engine khác sau.
8. Cờ cấp tenant (`tenant_settings` của CR-REQ-025) có là điều kiện cần của cờ cấp project không.

## 8. Tham chiếu

- `/opt/repos/orca/docs/research/receive-request/openspec-and-ai-tooling-integration.md`; `ai-steps-and-dev-server-connection-flows.md` mục 5, 6; `artifact-formats-ontology-and-execution-readiness.md` mục 4
- `/opt/repos/orca/docs/crs/v6/README.md` mục 3, 8; CR-REQ-003, 007, 008, 012, 013, 027
- `/opt/repos/orca/agent/src/relay/agent-print-mode-exec.ts`, `agent-rpc-dispatch-agent-exec.ts` (`agent.exec`), `agent-rpc-dispatch-fs.ts` (`fs.readFile`, `fs.stat`, `fs.glob`, `fs.writeFile`), `agent-rpc-dispatch-misc.ts` (`shell.exec`)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/check_dev_server_preflight.go`
- `/opt/repos/orca/backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (`CreateWorktree`, `RemoveWorktree`, `Commit`, `GetStatus`, `Discard`, `GeneratePullRequestFields`)
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/aidecompose_relay.go`, `simple_executor.go` (mẫu Relay)
- Mới: `request-service/internal/usecase/solution_engine.go`, `engine_native.go`, `engine_openspec.go`, `engine_readiness_gate.go`, `openspec_solution_prompt.go`, `openspec_plan_prompt.go`, `tasks_md_parser.go`, `tasks_md_sync.go`, `openspec_archive_consumer.go`; `internal/domain/openspec_profile.go`, `openspec_change.go`, `engine_settings.go`; `internal/adapter/{postgres,mysql}/openspec_change_repository.go`, `project_engine_settings_repository.go`

## 9. Tác động tới CR hiện có (không sửa trong CR này; người duyệt series sửa theo bảng)

| CR | Cần sửa gì |
|---|---|
| README v6 | Thêm feature `solution-engines` vào mục 4 và đợt thực thi; mục 3.5: `project_engine_settings`, `openspec_changes`, cột `requests.solution_engine`, `analysis_runs.engine`; mục 3.6: `Get/SetProjectEngineSettings` |
| CR-REQ-002 | Cột `requests.solution_engine`; hai bảng mới ở trên |
| CR-REQ-003 | `FlowDefinition` thêm `OpenSpecProfile` (`full|light|none`); ghim engine ở `type_confirmed` |
| CR-REQ-007 | Mục 2.5: `run_solution_generation` gọi `SolutionEngine.GenerateAnalysis`, `ai_completion_relay` thuộc `nativeEngine`; `analysis_runs`: thêm cột `engine`, thêm `agent_proposal` vào CHECK `mode`; `GenerateSolutionRequest` thêm `engine_override = 5`; thêm mã lỗi `REQUEST_ENGINE_*` vào 2.8 |
| CR-REQ-008 | Đếm giới hạn đồng thời gộp `agent_proposal`; dùng lại `analysis_secret_redaction.go`; Chẩn đoán vẫn `agent_readonly` ở hồ sơ `light` |
| CR-REQ-012 | `PlanGenerator` trở thành `nativeEngine.GeneratePlan`; `PROPOSE` gọi `engine.GeneratePlan`; `raw_ai_response` có thể là `tasks.md`; COMMIT kích hoạt viết lại vùng `plan` khi `engine=openspec` |
| CR-REQ-013 | Consumer `task_run_outcomes` phát tín hiệu cho `TasksMdSyncer`; `request.completed` kích hoạt archive; ghi chú rằng task không đọc `tasks.md` từ repo |
| CR-REQ-016, 017 | Kênh `request.engineGet`, `request.engineSet`; thêm `ToolSpec` hoặc dòng loại trừ cho `parity_test.go` (CR-016 mục 1 điểm 4) |
| CR-REQ-024 | Bốn metric ở 2.7; audit khi đổi cờ engine |
| CR-REQ-025 | Ma trận e2e thêm chiều engine; cờ project không thay cờ `request_flow_enabled` |
| CR-REQ-019, 020, 021 (frontend) | Hiển thị engine và nhánh proposal ở chi tiết Solution; nhãn "`tasks.md` chưa đồng bộ" |
