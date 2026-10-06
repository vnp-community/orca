# BE-REQ-SOL-026: Giao diện `SolutionEngine` với `native` và `openspec` (nhánh proposal, `tasks.md`, archive)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phụ thuộc [BE-REQ-SOL-007](../../solution-analysis/solutions/BE-REQ-SOL-007-solution-generation-options-and-selection.md), [BE-REQ-SOL-008](../../solution-analysis/solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md), [BE-REQ-SOL-012](../../plan-phase-task/solutions/BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md), [BE-REQ-SOL-013](../../plan-phase-task/solutions/BE-REQ-SOL-013-phase-execution-and-feedback-loop.md) và [BE-REQ-SOL-027](../../request-artifact-model/solutions/BE-REQ-SOL-027-artifact-schema-ontology-and-task-specs.md).

**CR:** [CR-REQ-026](../../../../../../docs/crs/v6/solution-engines/CR-REQ-026-openspec-solution-engine.md)
**Service:** `request-service` (`internal/domain`, `internal/usecase`, `internal/adapter/{grpcclient,postgres,mysql,grpc,eventbus}`, `migrations/*`) · `proto/orca/request/v1`. Không đổi `task-service`, `agent`.
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain thuần, cổng ra), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect, outbox), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (consumer bền, giao lặp), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (metric), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) (Relay, preflight dev server)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): CR-REQ-026, `docs/crs/v6/README.md` mục 3 và 8, các solution v6 đã có (`BE-REQ-SOL-002/003/007/008/009/012/013`), `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (`CreateWorktree`, `RemoveWorktree`, `Commit`, `GetStatus`, `Discard`, `BulkDiscard`, `ReadFile`, `WriteFile`, `StatFile`, `ReadDir`), `proto/orca/infrafleet/v1/infrafleet.proto` (`ResolveConnection`, `Relay`, `RelayRequest{connection_id, method, params_json}`), `services/infra-fleet-service/internal/usecase/check_dev_server_preflight.go`, `agent/src/relay/{agent-print-mode-exec.ts, agent-rpc-dispatch-agent-exec.ts, agent-rpc-dispatch-fs.ts, agent-rpc-dispatch-misc.ts}` (tồn tại). `backend-go/services/request-service/` **chưa có mã**; mọi `internal/...` bên dưới là đường dẫn sẽ tạo. Repo hiện không có thư mục `openspec/` và chưa chạy lệnh `openspec` nào: mọi cờ CLI là hiểu biết chung, chưa kiểm chứng.

### Correction relative to CR-REQ-026

| # | CR nói | Mã thật hoặc solution khác | Xử lý |
|---|--------|----------------------------|-------|
| C1 | `CreateWorktree` "với tên nhánh chỉ định, không liên kết issue" | `CreateWorktreeRequest` cần `project_id`, `repo_id`, `branch`, `base_ref`, tuỳ chọn `idempotency_key`, `name`, `path`. `ResolveConnection` trả `repo_path`, `worktree_id`, `connection_id` nhưng **không có `repo_id`** | Cổng `ProposalWorkspace.Ensure` nhận `repo_id` từ `ProjectContextResolver` (SOL-007). Nếu project-service không cấp được `repo_id`, task 026-06 dừng với `REQUEST_ENGINE_REPO_NOT_RESOLVED`. Dùng `idempotency_key = sha256(project_id\|repo_id\|branch)` để gọi lặp an toàn |
| C2 | Đọc ghi tệp bằng agent `fs.readFile`, `fs.writeFile`, `fs.glob` | git-gateway đã có `ReadFile`, `WriteFile`, `ReadDir`, `StatFile` nhận `worktree_id` + `path` | Ưu tiên cổng `ProposalWorkspace` dựng trên git-gateway (đường dẫn tương đối trong worktree, có sẵn kiểm đường dẫn của service). Chưa kiểm chứng rằng git-gateway chặn `..` và liên kết tượng trưng; test chặn ở tầng Orca bằng `change_dir` cố định (xem 2.F). Agent `fs.*` chỉ là phương án thay thế (Q1) |
| C3 | `Discard` mọi thay đổi chưa commit | `DiscardRequest{worktree_id, path}` theo từng đường dẫn; `BulkDiscardRequest{worktree_id, paths[]}` trả `failed_paths` | Dùng `GetStatus` lấy danh sách, rồi `BulkDiscard`; file `untracked` cần xử lý riêng (chưa rõ `Discard` có xoá `untracked` không), nên bước dọn dùng `RemoveWorktree{force:true}` rồi tạo lại khi `Discard` còn sót đường dẫn (Q5) |
| C4 | `analysis_runs` thêm `engine` và `mode='agent_proposal'` "cùng migration" với CR-REQ-007 | SOL-007 đặt `NNNN_analysis_runs`; CHECK `mode` Postgres là inline không tên (`analysis_runs_mode_check` theo quy ước đặt tên mặc định), MySQL phải có tên | Migration riêng `NNNN_solution_engines`: `ALTER` thêm cột và đổi CHECK. Nếu migration của SOL-007 chưa merge lúc làm, gộp vào đó (đọc `ls migrations/postgres` lúc làm). Task 026-02 ghi rõ cách lấy tên ràng buộc thật |
| C5 | `FlowDefinition.OpenSpecProfile` thêm vào CR-REQ-003 | SOL-003 mục B định nghĩa `FlowDefinition` phẳng (`AnalysisKind`, `PlanKind`, `PhaseRule`...), còn SOL-007 viết `FlowFor(type).Analysis.MinOptions` (cấu trúc lồng) | Mâu thuẫn có sẵn giữa SOL-003 và SOL-007; solution này thêm trường phẳng `OpenSpecProfile` vào `FlowDefinition` theo SOL-003 và không giải quyết phần `MinOptions` |
| C6 | `GenerateSolutionRequest.engine_override = 5` | SOL-007 mục D chép proto của CR-REQ-007; số trường 1 đến 4 do SOL-007 dùng (`request_id`, `feedback`, `idempotency_key` và một trường nữa) | Kiểm số trường thật trong `request.proto` lúc làm trước khi cấp 5 (task 026-05) |
| C7 | Archive chạy `agent.exec` với `cwd: worktree_path` | `agent.exec` nhận `{binary,args,cwd,stdin,env,timeoutMs}` trần 5 phút (CR đã đọc `agent-rpc-dispatch-agent-exec.ts`) | Giữ; `openspec archive` lớn có thể chạm trần, nên lỗi `timedOut` được tính là lỗi tạm và thử lại (2.G) |

## 2. Giải pháp

### A. Cây thư mục (mới, trong `backend-go/services/request-service/`)

```
internal/domain/engine_name.go                  # EngineName: native|openspec; ParseEngineName
internal/domain/openspec_profile.go             # OpenSpecProfile full|light|none; OpenSpecProfileFor(RequestType)
internal/domain/engine_settings.go              # ProjectEngineSettings{TenantID, ProjectID, Engine, MinVersion,...}
internal/domain/openspec_change.go              # OpenSpecChange, ChangeStatus, SyncState, NewChangeID(number, title)
internal/domain/tasks_md_parser.go              # ParsePlanRegion(md) (PlanProposal, []Violation)
internal/domain/tasks_md_render.go              # RenderPlanRegion, TickTask, InsertTaskIDs
internal/domain/engine_errors.go                # REQUEST_ENGINE_*, REQUEST_OPENSPEC_*
internal/usecase/solution_engine.go             # interface SolutionEngine, EngineRegistry, input/output
internal/usecase/engine_native.go               # nativeEngine (bọc SOL-007 và SOL-012)
internal/usecase/engine_openspec.go             # openspecEngine
internal/usecase/engine_readiness_gate.go       # Preflight, cache 10 phút
internal/usecase/openspec_solution_prompt.go    # prompt có rào cho design.md
internal/usecase/openspec_plan_prompt.go        # prompt tasks.md
internal/usecase/proposal_scope_check.go        # kiểm đường dẫn đổi nằm dưới openspec/changes/<id>/
internal/usecase/tasks_md_sync.go               # TasksMdSyncer (consumer task_run_outcomes)
internal/usecase/openspec_archive_consumer.go   # consumer request.completed
internal/usecase/pin_solution_engine.go         # ghim engine ở type_confirmed hoặc lần sinh đầu
internal/usecase/manage_project_engine_settings.go
internal/usecase/ports.go (sửa)                 # ProposalWorkspace, DevServerExecutor, EngineSettingsRepository, OpenSpecChangeRepository
internal/adapter/grpcclient/proposal_workspace.go   # git-gateway: Create/Remove worktree, Commit, ReadFile, WriteFile, GetStatus
internal/adapter/grpcclient/dev_server_executor.go  # infra-fleet Relay: agent.exec, agent.execPrompt
internal/adapter/{postgres,mysql}/project_engine_settings_repository.go
internal/adapter/{postgres,mysql}/openspec_change_repository.go
internal/adapter/grpc/engine_settings_server.go
migrations/{postgres,mysql}/NNNN_solution_engines.{up,down}.sql
testdata/openspec/{design_ok.md, design_bad_json.md, tasks_ok.md, tasks_prose_checkbox.md, tasks_cycle.md}
```

### B. Domain thuần (không import ngoài stdlib)

```go
type EngineName string // "native" | "openspec"
type OpenSpecProfile string // "full" | "light" | "none"

func OpenSpecProfileFor(t RequestType) OpenSpecProfile
// change_request, refactor -> full; bug, security, performance -> light; còn lại -> none

type ChangeStatus string // preparing | ready | archived | abandoned
type SyncState string    // in_sync | pending | failed

type OpenSpecChange struct {
    ID, TenantID, RequestID, ProjectID string
    ChangeID, Branch, WorktreeID, BaseRef, OpenSpecVersion string
    Status ChangeStatus; TasksMdDigest string; TasksSyncState SyncState
    LastSyncedAt *time.Time; ArchivedCommit string
    CreatedAt, UpdatedAt time.Time; Version int64
}
var changeIDPattern = regexp.MustCompile(`^req-[0-9]+-[a-z0-9-]{1,40}$`)
func NewChangeID(number int64, title string) string // NFD, bỏ dấu, kebab, cắt 40; tiêu đề rỗng -> "req-<n>-request"
```

`EffectiveEngine(pinned *EngineName, settings *ProjectEngineSettings, profile OpenSpecProfile) EngineName`: `profile=none` luôn `native`; có `pinned` thì dùng `pinned`; còn lại theo `settings` (không dòng nghĩa là `native`). Hàm thuần để bảng quyết định test được hết.

### C. Migration `NNNN_solution_engines` (hai dialect)

```sql
-- Postgres (schema request)
CREATE TABLE request.project_engine_settings (
  tenant_id UUID NOT NULL, project_id UUID NOT NULL,
  solution_engine TEXT NOT NULL CHECK (solution_engine IN ('native','openspec')),
  openspec_min_version TEXT NULL, updated_by UUID NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), version BIGINT NOT NULL DEFAULT 1,
  PRIMARY KEY (tenant_id, project_id));
CREATE TABLE request.openspec_changes (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL, project_id UUID NOT NULL,
  change_id TEXT NOT NULL, branch TEXT NOT NULL, worktree_id TEXT NOT NULL DEFAULT '', base_ref TEXT NOT NULL DEFAULT '',
  openspec_version TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('preparing','ready','archived','abandoned')),
  tasks_md_digest TEXT NOT NULL DEFAULT '',
  tasks_sync_state TEXT NOT NULL DEFAULT 'in_sync' CHECK (tasks_sync_state IN ('in_sync','pending','failed')),
  last_synced_at TIMESTAMPTZ NULL, archived_commit TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), version BIGINT NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, request_id));
ALTER TABLE request.requests ADD COLUMN solution_engine TEXT NULL CHECK (solution_engine IN ('native','openspec'));
ALTER TABLE request.analysis_runs ADD COLUMN engine TEXT NOT NULL DEFAULT 'native';
ALTER TABLE request.analysis_runs DROP CONSTRAINT analysis_runs_mode_check;
ALTER TABLE request.analysis_runs ADD CONSTRAINT analysis_runs_mode_check CHECK (mode IN ('complete','agent_readonly','agent_proposal'));
-- + RLS tenant_isolation (NULLIF(current_setting('app.tenant_id', true), '')::uuid) cho hai bảng mới
```

MySQL: `CHAR(36)`, `VARCHAR(n)` cho cột trong khoá, `TIMESTAMP(6)`, `DROP CHECK` rồi `ADD CONSTRAINT` (hiệu lực từ 8.0.16), cột `solution_engine VARCHAR(10) NULL` kèm `CHECK`. Down: xoá hai bảng và cột; trước khi khôi phục CHECK `mode`, `UPDATE analysis_runs SET mode='complete' WHERE mode='agent_proposal'` (ghi chú chỉ dùng khi rollback toàn bộ v6).

### D. Giao diện và đăng ký

```go
type SolutionEngine interface {
    Name() domain.EngineName
    Preflight(ctx context.Context, p ProjectRef) (PreflightReport, error)
    GenerateAnalysis(ctx context.Context, in AnalysisInput) (AnalysisOutput, error)
    GeneratePlan(ctx context.Context, in PlanInput) (domain.PlanProposal, PlanRaw, error)
    OnRequestCompleted(ctx context.Context, in CompletionInput) error // native: nil
}
type EngineRegistry struct{ engines map[domain.EngineName]SolutionEngine }
func (r *EngineRegistry) For(name domain.EngineName) (SolutionEngine, error)
```

`GenerateSolution` (SOL-007) và `GeneratePlan` (SOL-012) gọi `registry.For(effectiveEngine)`; phần sau kết quả (`SolutionOptions.Validate`, `ValidateProposal`, `OpenApproval`, `TransitionRequest`, outbox) **không đổi** và nằm ngoài engine. `nativeEngine` chỉ di chuyển mã SOL-007 `run_solution_generation` (đoạn gọi `AICompleter`) và SOL-012 `PlanGenerator`, không đổi hành vi (bộ test cũ chạy qua giao diện, phải xanh y nguyên).

### E. Cổng điều kiện (`EngineReadinessGate`)

`Preflight` chạy trước `GenerateAnalysis` và `GeneratePlan` có `engine=openspec`; kết quả cache 10 phút theo `(dev_server_id, repo_path)` (đồng hồ `Clock` của SOL-007, test được). Các kiểm tra, mã lỗi và `fix_hint` như bảng CR mục 2.3. Điểm triển khai:

- `DevServerExecutor.Exec(ctx, connectionID string, binary string, args []string, cwd string, timeout time.Duration) (ExecResult, error)` gọi `Relay{method:"agent.exec", params_json}`; **không** ghép chuỗi shell. `ExecResult{Stdout, Stderr string; ExitCode int; TimedOut bool}`.
- `openspec --version` đọc bằng regexp `(\d+)\.(\d+)\.(\d+)`; so với `REQUEST_OPENSPEC_MIN_VERSION` (mặc định rỗng, nghĩa là không so) hoặc `openspec_min_version` của project; `domain.CompareSemver(a, b string) (int, error)` thuần.
- Kiểm repo đã `openspec init`: `StatFile` thư mục `openspec` trong **repo gốc** (qua `ResolveConnection.repo_path`), chỉ đọc.
- `claude` đã đăng nhập: chưa có lệnh không tốn token (Q3). Phương án triển khai: một `agent.execPrompt` prompt `"reply with OK"` có `timeoutMs` 60 000, chỉ cache kết quả thành công 10 phút, thất bại không cache.
- Git bẩn là cảnh báo (`PreflightReport.Warnings`), không chặn.

### F. Nhánh, worktree và phạm vi ghi

`ProposalWorkspace` (cổng ra, `adapter/grpcclient/proposal_workspace.go`):

```go
type ProposalWorkspace interface {
    Ensure(ctx context.Context, in EnsureWorkspaceInput) (Workspace, error) // CreateWorktree idempotent
    ReadFile(ctx context.Context, ws Workspace, relPath string, maxBytes int) ([]byte, error)
    WriteFile(ctx context.Context, ws Workspace, relPath string, content []byte) error
    ChangedPaths(ctx context.Context, ws Workspace) ([]string, error)   // GetStatus
    DiscardAll(ctx context.Context, ws Workspace) error
    Commit(ctx context.Context, ws Workspace, message string, paths []string) (sha string, err error)
    Remove(ctx context.Context, ws Workspace) error
}
```

Nhánh `request/<number>-proposal`, `base_ref` là nhánh mặc định của repo; **không** gọi `CreateWorktreeFromIssue` (nó gắn issue, có thể kích hoạt `issue-status-sync`). Mọi đường dẫn ghi hoặc đọc do Orca dựng từ `change_id` đã qua `changeIDPattern`; `relPath` được `path.Clean` rồi kiểm tiền tố `openspec/changes/<change_id>/` ở `proposal_scope_check.go` (từ chối `..`, tuyệt đối, ký tự NUL). Đường dẫn đổi sau chạy agent mà ngoài tiền tố thì run `failed` `REQUEST_OPENSPEC_OUT_OF_SCOPE_CHANGE` và `DiscardAll`.

### G. Sinh Solution, Plan, đồng bộ và archive

- **Solution (`full`)**: `Preflight` → `Ensure` + `OpenSpecChangeRepository.Upsert(status=preparing)` → `openspec_solution_prompt` → `agent.execPrompt{prompt, worktreePath, trustPreset:"default", env:{ORCA_REQUEST_ID, ORCA_PROJECT_ID}, timeoutMs}` → `proposal_scope_check` → đọc `proposal.md`, `design.md`, delta spec (≤ 256 KB mỗi tệp), `RedactSecrets` (SOL-008) → parse khối `orca-json` (SOL-027) → `agent.exec openspec validate <change_id>` → `SolutionOptions.Validate` + kiểm ngữ nghĩa SOL-027 → `RenderArtifact` ghi lại vùng `options` → `Commit` → trả `AnalysisOutput{OptionsJSON, Provenance, Raw}`. Sai: thử lại đúng một lần (`attempt=2`) với thông điệp lỗi cụ thể trong cùng worktree, rồi `REQUEST_OPENSPEC_INVALID_OUTPUT`.
- **Plan (`full` và `light`)**: cùng khung, prompt `tasks.md`, parser `ParsePlanRegion` chặt (văn phạm CR mục 2.5), kết quả `PlanProposal` + `PlanRaw` (nội dung `tasks.md` thô). `light` (bug, security, performance) bắt đầu từ Chẩn đoán `approved` (SOL-008), tạo `proposal.md` rồi `tasks.md`, không tạo `design.md`.
- **`TasksMdSyncer`**: consumer của `task_run_outcomes` (SOL-013) khi `outcome=succeeded`; gộp lô tối đa mỗi 30 giây mỗi Request; `TickTask` chỉ đổi `[ ]` thành `[x]`, không bao giờ ngược lại; lỗi đặt `tasks_sync_state=failed`, không ảnh hưởng luồng. Drift: so `tasks_md_digest` với bản Orca vừa ghi; lệch thì ghi đè vùng Orca, đếm `request_engine_drift_total`.
- **Archive**: consumer bền của `orca.request.request.completed` (khử trùng `processed_events`) gọi `agent.exec openspec archive <change_id> --yes` ở worktree, kiểm `ChangedPaths`, `Commit`, `status=archived`, `archived_commit`. Lỗi thử lại backoff tối đa 5 lần rồi `tasks_sync_state=failed`. `cancelled`: `status=abandoned`, giữ nhánh; không tự merge hay mở PR (Q6).

### H. Proto, lỗi, sự kiện, quyền

```proto
rpc GetProjectEngineSettings(GetProjectEngineSettingsRequest) returns (ProjectEngineSettings);
rpc SetProjectEngineSettings(SetProjectEngineSettingsRequest) returns (ProjectEngineSettings); // admin
message ProjectEngineSettings { string project_id=1; string solution_engine=2; string openspec_min_version=3; int64 version=4; }
message SetProjectEngineSettingsRequest { string project_id=1; string solution_engine=2; string openspec_min_version=3; int64 expected_version=4; }
// GenerateSolutionRequest: thêm string engine_override = <số kế tiếp, kiểm file thật>; chỉ nhận "native"
// Solution/AnalysisRun message: thêm string engine, string proposal_branch
```

Mã lỗi theo CR mục 2.7 (`REQUEST_ENGINE_*`, `REQUEST_OPENSPEC_*`) cộng `REQUEST_ENGINE_REPO_NOT_RESOLVED`, `REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT`. Không thêm subject sự kiện; đổi cờ ghi audit qua `auditclient.Append` (`AppendDetailed` của CR-REQ-024 khi có). Metric: `request_engine_runs_total{engine,kind,result}`, `request_engine_preflight_failures_total{code}`, `request_openspec_sync_state{state}`, `request_engine_drift_total`. Quyền: `Set/Get` chỉ `tenant.Role(ctx)=="admin"`; `engine_override` chỉ `reporter_id` hoặc admin (`REQUEST_ENGINE_OVERRIDE_NOT_ALLOWED`).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|-----------|-------|
| 1 | Giao diện bao cả Solution lẫn Plan, nhưng kết quả sau đó dùng đường chung | Máy trạng thái, `Approval`, `solutions`, `CreatePlanTree` không đổi |
| 2 | `ProposalWorkspace` dựng trên git-gateway thay vì agent `fs.*` | Có sẵn `worktree_id`, đường dẫn tương đối; giảm bề mặt agent. Phải kiểm chặn `..` (Q1) |
| 3 | `EffectiveEngine` là hàm thuần; ghim ở `requests.solution_engine` | Đổi cờ project không làm lệch Request đang chạy; test bảng đầy đủ |
| 4 | Preflight không đạt thì dừng, không lùi về `native` | Hai engine cho kết quả khác nhau |
| 5 | `openspec validate` bổ sung cho `ValidateProposal` | Công cụ ngoài không biết giới hạn Phase, nhãn của Orca |
| 6 | Migration riêng `solution_engines` thay vì sửa migration SOL-007 | Không sửa tài liệu đã duyệt; gộp nếu SOL-007 chưa merge |
| 7 | Đồng bộ `tasks.md` một chiều, không bao giờ bỏ tick | Orca là nguồn chính; tránh xung đột hai chiều |
| 8 | Chỉ dẫn OpenSpec đặt trong prompt, không dùng slash command | `claude --print` với slash command chưa kiểm chứng |

## 4. Phụ thuộc và thứ tự

Cần: SOL-007 (`analysis_runs`, `GenerateSolution`), SOL-012 (`PlanGenerator`, `ValidateProposal`), SOL-013 (`task_run_outcomes`, `request.completed`), SOL-027 (`RenderArtifact`, parser khối `orca-json`, kiểm ngữ nghĩa), SOL-003 (`FlowDefinition`). Mềm: CR-REQ-033 (agent). Thứ tự task: 01 domain, 02 migration và repository (song song với 01), 03 parser `tasks.md`, 04 preflight, 05 giao diện và `nativeEngine` (hồi quy), 06 `GenerateAnalysis`, 07 `GeneratePlan` + sync + archive, 08 RPC và metric. Mở khoá: không CR nào bắt buộc.

## 5. Kiểm thử

- **Unit domain:** `NewChangeID` (emoji, tiếng Việt có dấu, rỗng, dài), `OpenSpecProfileFor` đủ 11 loại, `EffectiveEngine` mọi tổ hợp, `ParsePlanRegion` (golden đúng, dòng lạ, chỉ số lệch, vòng phụ thuộc, prose chứa checkbox giả), `TickTask` không bỏ tick, `CompareSemver`.
- **Unit usecase:** fake `DevServerExecutor`, `ProposalWorkspace`, `Clock`: từng mã Preflight và cache 10 phút, phạm vi ghi sai, thử lại một lần, tick, drift, archive giao lặp.
- **Integration hai dialect (`-tags=integration`):** `project_engine_settings`, `openspec_changes` (UNIQUE `(tenant_id, request_id)`), `analysis_runs.engine` và CHECK `mode` mới, Postgres RLS với role `NOSUPERUSER NOBYPASSRLS`.
- **Hợp đồng:** `buf breaking`; test hồi quy SOL-007 và SOL-012 chạy qua `nativeEngine`.
- **Thủ công, chưa kiểm chứng:** OpenSpec thật trên dev server mẫu (bắt buộc trước khi bật cờ).

## 6. Rủi ro và điểm chưa kiểm chứng

- Mọi hành vi OpenSpec (cấu trúc, `validate`, `archive`, định dạng) là hiểu biết chung; đổi phiên bản làm vỡ prompt và parser. Giảm: `openspec_min_version`, golden, CI engine.
- Agent không có chế độ chỉ đọc; kiểm sau chạy chỉ phát hiện. Tệp `untracked` do agent tạo ngoài phạm vi có thể không bị `Discard` dọn (C3).
- `CreateWorktree` có thể phát `worktree.created` làm `issue-status-sync` đổi trạng thái Jira; chưa kiểm chứng.
- Worktree proposal tồn tại tới khi Request kết thúc: tăng số worktree trên dev server.
- Thời gian một run có thể sát trần 15 phút của `agent.execPrompt`.

## 7. Câu hỏi mở

1. `ProposalWorkspace` dùng git-gateway (đề xuất) hay agent `fs.*` như CR.
2. Bật theo project (đề xuất) hay theo loại; cờ tenant có là điều kiện cần không (CR-REQ-025).
3. Lệnh kiểm `claude` đã đăng nhập không tốn token.
4. `repo_id` lấy ở đâu cho `CreateWorktree` (project-service?).
5. `Discard` có dọn tệp `untracked` không; nếu không, dùng `RemoveWorktree{force}` rồi tạo lại.
6. Sau `archive`, tự mở PR bằng `GeneratePullRequestFields` hay để người dùng.
7. Chọn một giữa OpenSpec và Spec Kit.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/solution-engines/CR-REQ-026-openspec-solution-engine.md`; `/opt/repos/orca/docs/crs/v6/README.md` mục 3, 8
- `/opt/repos/orca/docs/research/receive-request/openspec-and-ai-tooling-integration.md`
- `/opt/repos/orca/backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (`CreateWorktree` dòng 736, `Commit` 206, `GetStatus` 182, `WriteFile` 531, `BulkDiscard` 1108)
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`ResolveConnectionResponse` 562, `RelayRequest` 819)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/check_dev_server_preflight.go`
- `/opt/repos/orca/agent/src/relay/agent-print-mode-exec.ts`, `agent-rpc-dispatch-agent-exec.ts`, `agent-rpc-dispatch-fs.ts`
- Solution liên quan: `../../solution-analysis/solutions/BE-REQ-SOL-007-*.md`, `BE-REQ-SOL-008-*.md`; `../../plan-phase-task/solutions/BE-REQ-SOL-012-*.md`, `BE-REQ-SOL-013-*.md`; `../../request-lifecycle/solutions/BE-REQ-SOL-003-*.md`
