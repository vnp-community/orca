# BE-REQ-SOL-007: Sinh Solution nhiều phương án, chọn và duyệt

> **📋 Proposed** (chưa triển khai). Tài liệu thiết kế thực thi ở backend cho CR-REQ-007. Chạy hoàn toàn trong `request-service` (service chưa tồn tại, xem mục 1).

**CR:** [CR-REQ-007](../../../../../../docs/crs/v6/solution-analysis/CR-REQ-007-solution-generation-options-and-selection.md)
**Service:** `request-service` (mới) · `proto` · không đổi `task-service`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (lớp domain/usecase/adapter), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (DB-per-service, outbox, hai dialect), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (tenant, dữ liệu không tin cậy), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (subject, outbox, Relay), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (lease, phục hồi), [`services/task-service`](../../../../tdd/services/task-service.md), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `docs/crs/v6/README.md` (mục 3 và 8), CR-REQ-007, `task-service/internal/adapter/grpcclient/aidecompose_relay.go` (`AICompleter.Complete`, Relay `ai.complete`, params `{prompt}`, kết quả `{content}`), `task-service/internal/usecase/execution_lease.go` (`WithExecutionLeases`, `RecoverInterruptedExecutions.RunRecoveryLoop`), `task-service/internal/adapter/postgres/execution_leases.go` và `adapter/mysql/execution_leases.go` (mẫu `FOR UPDATE SKIP LOCKED` hai dialect), `proto/orca/infrafleet/v1/infrafleet.proto` (`RelayRequest{connection_id, method, params_json}`, `ResolveConnectionResponse{connected, dev_server, repo_path, worktree_id}`), `mcp-service/internal/domain/params_hash.go`.

Kiểm tra bằng `ls`: `backend-go/services/request-service` **không tồn tại**. Mọi đường dẫn dưới đây là "(mới)". `proto/orca/request/` cũng chưa có.

**Correction relative to CR-REQ-007**
1. **Số migration.** Service chưa có thư mục `migrations`. Theo CR-REQ-001/002, `0001` là hạ tầng và `0002_request_core` là lõi dữ liệu; số của `analysis_runs` phải đọc lại thư mục `migrations/postgres` và `migrations/mysql` của `request-service` lúc làm task 01 rồi lấy số kế tiếp (các CR 004, 006, 009, 010 cũng thêm migration, thứ tự theo ngày merge). Trong tài liệu gọi là `NNNN_analysis_runs`.
2. **Digest phải tính trên dạng chuẩn tắc, không trên văn bản JSON lưu trong DB.** Postgres `JSONB` đổi thứ tự khoá và khoảng trắng, MySQL `JSON` cũng chuẩn hoá khác; `sha256(options)` lấy từ cột sẽ khác giữa hai DB. Dùng cách của `mcp-service/internal/domain/params_hash.go` (phân tích chặt, từ chối khoá trùng, in chuẩn tắc) nhưng sao chép vào `request-service/internal/domain/canonical_json_digest.go` (mới) vì repo nhân bản theo từng service (`internal` không import chéo được).
3. **Prompt không lấy `credential_ref`**: `buildDecomposePrompt` của task-service nhận `providerCtx` chứa `credential_ref`; bản của request-service không có tham số đó.
4. `ResolveProvider` dùng chỉ để fail sớm, hành vi khi không có account chưa kiểm chứng (giữ như CR).

## 2. Giải pháp

### A. Cây thư mục (tất cả trong `backend-go/services/request-service/`, mới)

```
internal/domain/
    solution.go                     # Solution, SolutionKind, SolutionStatus, chuyển trạng thái
    solution_options.go             # SolutionOptions + Validate(minOptions)
    analysis_run.go                 # AnalysisRun, AnalysisMode, RunStatus
    canonical_json_digest.go        # DigestOptions(options, chosen) -> hex sha256
internal/usecase/
    ports.go (sửa)                  # AnalysisRunRepository, SolutionRepository, AICompleter, ProjectContextResolver, Clock
    generate_solution.go
    run_solution_generation.go      # worker: prompt -> Relay -> extract -> persist
    recover_interrupted_analysis_runs.go
    choose_solution_option.go
    list_solutions.go
    solution_approval_handler.go    # SubjectHandler cho subject_type=solution
    solution_prompt.go              # buildSolutionPrompt
    solution_output_extraction.go   # trích khối JSON ngoài cùng, chịu code fence
internal/adapter/grpcclient/ai_completion_relay.go, project_context_resolver.go
internal/adapter/postgres/analysis_run_repository.go, solution_repository.go
internal/adapter/mysql/analysis_run_repository.go, solution_repository.go
internal/adapter/grpc/solution_server.go
migrations/{postgres,mysql}/NNNN_analysis_runs.{up,down}.sql
proto/orca/request/v1/request.proto (sửa: thêm message/RPC mục D)
```

### B. Domain

```go
type SolutionKind string // solution|diagnosis|findings|answer
type SolutionStatus string // draft|proposed|approved|rejected|superseded
type AnalysisMode string // complete|agent_readonly
type RunStatus string // running|succeeded|failed

type Solution struct {
    ID, TenantID, RequestID string
    Kind SolutionKind; Status SolutionStatus
    OptionsJSON []byte; ChosenOption *int // chỉ số 0-based trong options.options[]
    ContentRef, GenerationRunID string
    Version int64; CreatedAt time.Time
}
func (s *Solution) Supersede() error  // draft|proposed|rejected -> superseded
func (s *Solution) Propose(opts []byte) error // draft -> proposed
func (s *Solution) Choose(idx int) error     // chỉ khi proposed

type SolutionOptions struct { SchemaVersion int; Options []Option; Recommendation Recommendation; Assumptions, OpenQuestions []string }
func ParseSolutionOptions(raw []byte) (SolutionOptions, error)
func (o SolutionOptions) Validate(minOptions int) error // 2.3 của CR: 1..4 phương án (>=minOptions), id duy nhất opt-N, đúng một recommended trùng recommendation.option_id
func DigestOptions(options []byte, chosen *int) (string, error) // canonical JSON + chosen
```
`minOptions` đến từ registry `FlowFor(type).Analysis.MinOptions` của CR-REQ-003 (`change_request`=2, `refactor`=1, xem câu hỏi mở 3). Domain không import gì ngoài stdlib.

### C. Migration `NNNN_analysis_runs` (hai dialect)

Cột như bảng ở CR-REQ-007 mục 2.2. Điểm hai dialect khác nhau:

```sql
-- postgres
CREATE TABLE request.analysis_runs (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, request_id UUID NOT NULL REFERENCES request.requests(id),
  kind TEXT NOT NULL CHECK (kind IN ('solution','diagnosis','findings','answer')),
  mode TEXT NOT NULL CHECK (mode IN ('complete','agent_readonly')),
  status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed')),
  idempotency_key TEXT NULL, attempt INT NOT NULL DEFAULT 1,
  lease_owner TEXT NULL, lease_expires_at TIMESTAMPTZ NULL,
  error_code TEXT NULL, error_message TEXT NULL, raw_output TEXT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(), finished_at TIMESTAMPTZ NULL);
CREATE UNIQUE INDEX analysis_runs_one_running ON request.analysis_runs (tenant_id, request_id, kind) WHERE status='running';
CREATE UNIQUE INDEX analysis_runs_idem ON request.analysis_runs (tenant_id, request_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX analysis_runs_lease_scan ON request.analysis_runs (status, lease_expires_at);
-- RLS tenant_isolation giống bảng requests của CR-REQ-002
```
```sql
-- mysql 8.0.1+
active_key VARCHAR(80) GENERATED ALWAYS AS (IF(status='running', CONCAT(request_id,':',kind), NULL)) STORED,
UNIQUE KEY analysis_runs_one_running (tenant_id, active_key),
UNIQUE KEY analysis_runs_idem (tenant_id, request_id, idempotency_key), -- NULL không va chạm
raw_output MEDIUMTEXT NULL
```
Down: `DROP TABLE`. Không đổi bảng `solutions` (đã có từ `0002_request_core`).

### D. Proto

Thêm đúng các enum/message ở CR-REQ-007 mục 2.7 vào `request.proto`: `SolutionKind`, `SolutionStatus`, `AnalysisMode`, `Solution`, `AnalysisRun`, `GenerateSolution{Request,Response}`, `ListSolutions{Request,Response}`, `ChooseSolutionOption{Request,Response}`; RPC `GenerateSolution`, `ListSolutions`, `ChooseSolutionOption` trên `RequestService`. `chosen_option` -1 nghĩa là chưa chọn. Số field theo CR; chạy `buf breaking` với nhánh chính.

### E. Use case

**`GenerateSolution.Execute(ctx, in)`**
1. `tenant.RequireTenantID`; tải Request; kiểm quyền ghi (`RequestWriteAuthorizer` của CR-REQ-003/010; tạm: reporter hoặc `tenant.Role(ctx)=="admin"`).
2. `FlowFor(req.Type)`: `Analysis.Kind` phải là `solution`, không thì `REQUEST_SOLUTION_KIND_NOT_ALLOWED`.
3. Trạng thái: `analyzing`, hoặc `awaiting_analysis_approval` với `feedback` không rỗng (đường sinh lại).
4. `ResolveConnection(project_id)` (infra-fleet): `connected=false` thì `REQUEST_SOLUTION_NO_CONNECTION`.
5. Trong một transaction: nếu có run `running` hoặc cùng `idempotency_key` thì trả lại run đó; nếu là sinh lại: huỷ Approval `pending` (`CancelPendingForRequest`, lý do `revision_requested`), Solution cũ `superseded`, `TransitionRequest(analysis_revision)`; rồi chèn `analysis_runs(running, lease_owner, lease_expires_at = now_db + 90s)` và `solutions(draft, generation_run_id)`.
6. Phát worker bất đồng bộ (`go run.Execute(runID)` trong tiến trình có `context` riêng, không theo `ctx` RPC) và trả `{solution_id, run_id}`.

**`RunSolutionGeneration`**: gia hạn lease 30s/lần (`ReleaseLease`/`RenewLease` trong repository, đồng hồ DB); dựng prompt (`buildSolutionPrompt`: chỉ dẫn, khối `<request>` cắt body 12 000 ký tự, ngữ cảnh dự án best-effort, `<prior_artifacts>` mỗi mục cắt 8 000, không `credential_ref`); gọi `AICompleter.Complete` với timeout `REQUEST_AI_COMPLETE_TIMEOUT` (mặc định đề xuất 120s); trích JSON; `Validate`; sai thì lặp một lần (`attempt=2`) kèm lỗi cụ thể; vẫn sai thì `failed` `REQUEST_SOLUTION_INVALID_OUTPUT`. Thành công: một transaction gồm `solutions.options`, `status=proposed`, run `succeeded`, các Solution cũ cùng `(request_id, kind)` ở `proposed|rejected` thành `superseded`, outbox `solution.proposed`, `OpenApproval(subject_type=solution, stage=awaiting_analysis_approval)`, `TransitionRequest(analysis_ready, ExpectedFrom=analyzing)`. Lỗi: run `failed` và xoá Solution `draft` cùng transaction; Request giữ `analyzing`.

**`RecoverInterruptedAnalysisRuns.RunRecoveryLoop(ctx, 30s)`**: `ClaimExpiredRuns(batch)`: Postgres `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING`; MySQL `SELECT ... FOR UPDATE SKIP LOCKED` trong transaction rồi `UPDATE`. Đánh `failed` `REQUEST_SOLUTION_RUN_INTERRUPTED`, xoá `draft`. Dùng lại cách viết của `adapter/{postgres,mysql}/execution_leases.go` ở task-service.

**`ChooseSolutionOption`**: Solution phải `proposed`; `option_id` đổi sang chỉ số; `UPDATE solutions SET chosen_option=?, version=version+1 WHERE id=? AND tenant_id=? AND status='proposed' AND version=?`; cùng transaction gọi `ApprovalRepository.UpdatePendingDigest("solution", id, DigestOptions(options, &idx))`; trả `approval_digest`. Lặp cùng `option_id` là no-op thành công.

**`SolutionApprovalHandler`** (đăng ký cho `subject_type=solution`, dùng cả `diagnosis`): `ValidateForRequest` trả `DigestOptions(options, chosen)`; `kind=solution` mà `chosen_option` NULL thì `REQUEST_SOLUTION_OPTION_NOT_CHOSEN`. `OnApproved`: Solution `approved`, outbox `solution.approved{request_id, solution_id, kind, chosen_option}`, `TransitionRequest(analysis_approved)`. `OnRejected`: Solution `rejected`, `TransitionRequest(analysis_rejected)` (về `request_backlog`, `returned_from_stage=analysis`). `OnClosedWithoutDecision`: `superseded` khi do đổi loại, còn lại giữ `proposed`.

### F. Lỗi, sự kiện, quyền

Mã lỗi theo CR mục 2.8 (`REQUEST_SOLUTION_*`, `REQUEST_AI_NO_PROVIDER`). Sự kiện `orca.request.solution.proposed` và `.approved` qua `outbox_events`; payload không chứa `options`. Mọi use case gọi `tenant.RequireTenantID`; truy vấn chéo tenant trả `REQUEST_SOLUTION_NOT_FOUND`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | `ai.complete` qua Relay, không `agent.execPrompt` | Không cần worktree; cùng đường `AIDecompose` đã chạy |
| 2 | `analysis_runs` bền kèm lease thay vì goroutine trần | Engine 1 từng mất việc khi restart (CR-TG-008) |
| 3 | Chỉ mục "một run `running`" mỗi `(request, kind)` | Chặn bấm nhiều lần |
| 4 | Digest tính từ JSON chuẩn tắc + `chosen_option` | JSONB/JSON đổi khoá; chống duyệt nội dung đã đổi |
| 5 | Worker không dùng `ctx` của RPC | RPC trả ngay; huỷ RPC không được giết run |
| 6 | Lỗi không tự về backlog | Quyền trả backlog thuộc người |

## 4. Phụ thuộc và thứ tự

Cần trước: CR-REQ-001 (khung, outbox), CR-REQ-002 (`solutions`, `requests`), CR-REQ-003 (`FlowFor`, `TransitionRequest`), CR-REQ-005 (loại đã xác nhận), CR-REQ-009 (`OpenApproval`, `SubjectHandler`; có thể dùng cổng no-op tạm như `ApprovalRecorder` của CR-REQ-005). Mở khoá CR-REQ-008, 012, 020. Ghi tiến (không phụ thuộc): sau này CR bổ sung (Clarification ở CR-REQ-028) có thể chèn bước hỏi lại trước `analyzing`; solution này không thay đổi vì chỉ đọc `Request.Status`.

## 5. Kiểm thử

- Unit domain: `SolutionOptions.Validate` (bảng lỗi), `DigestOptions` ổn định theo thứ tự khoá, từ chối khoá trùng; bộ trích JSON (fence, văn bản thừa).
- Unit usecase (fake Relay, clock, repo): thành công, lỗi AI, JSON sai rồi đúng, sai hai lần, không kết nối, run trùng, mất lease, chọn lặp, sinh lại có `feedback`.
- Integration hai DB (Postgres và MySQL, `common/dbcapability`): chỉ mục duy nhất của run, `SKIP LOCKED` hai worker, rollback transaction "solution + approval + outbox", Unicode tiếng Việt trong cột JSON.
- Hợp đồng: `buf breaking`; `testdata/solution_options/*.json`; test `SubjectHandler` chạy cùng bộ test hợp đồng của SOL-009.
- Chưa kiểm chứng: `ai.complete` thật (thời gian, giới hạn, tuân thủ schema), cần dev server thật.

## 6. Rủi ro và điểm chưa kiểm chứng

- `ai.complete` không đọc repo; chất lượng phương án giới hạn. `assumptions` và `open_questions` bắt buộc hiển thị.
- 120s, 256 KB, 64 KB là mặc định đề xuất, chưa đo.
- Worker trong tiến trình: agent có thể chạy tiếp sau khi tiến trình chết (tốn token, mất kết quả); chấp nhận ở v1.
- `ListSolutionsResponse.runs` là cách duy nhất UI thấy lỗi run; kiểm với CR-REQ-016/020.

## 7. Câu hỏi mở

1. `min_options` của `refactor` (CR đề xuất 1, README chỉ nêu "≥2" cho `change_request`).
2. `options` mặc định `'[]'` ở CR-REQ-002 nhưng tài liệu là đối tượng; ứng dụng luôn ghi giá trị, cần CR-REQ-002 đổi mặc định `'{}'`.
3. `content_ref` không dùng ở `kind=solution`.
4. Quyền ghi mức Request: tạm reporter hoặc admin (CR-REQ-003/010 chốt).
5. Tech stack lấy qua git-gateway hay bỏ để đỡ dependency (CR hỏi); đề xuất v1 chỉ dùng project-service.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/solution-analysis/CR-REQ-007-solution-generation-options-and-selection.md`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/aidecompose_relay.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/execution_lease.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/execution_leases.go`, `adapter/mysql/execution_leases.go`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/domain/params_hash.go`
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`ResolveConnection`, `Relay`)
- `/opt/repos/orca/backend-go/common/{outbox,dbcapability,tenant,apperrors}`
