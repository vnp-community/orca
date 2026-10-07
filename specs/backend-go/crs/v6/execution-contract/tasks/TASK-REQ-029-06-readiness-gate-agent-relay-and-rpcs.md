# TASK-REQ-029-06: `ReadinessGate` ba tầng, cổng `AgentRelay` và RPC `CheckReadiness`, `GetReadinessReport`, `ListReadiness`

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.D, 2.H
**Priority:** P0
**Service/Area:** `request-service` (mới) / usecase, adapter grpcclient, proto, grpc handler
**File:** `internal/usecase/readiness_gate.go` (mới), `internal/usecase/readiness_gate_tiers.go` (mới), `internal/usecase/check_readiness.go` (mới), `internal/usecase/agent_relay_ports.go` (mới), `internal/adapter/grpcclient/agent_relay.go` (mới), `internal/adapter/grpcclient/worktree_resolver.go` (mới), `internal/adapter/grpc/server_readiness.go` (mới), `backend-go/proto/orca/request/v1/request.proto` (sửa), và các `_test.go`
**Depends on:** TASK-REQ-029-04 (repository báo cáo), TASK-REQ-029-05 (`TaskSpecV2`, `ScopeMatcher`), TASK-REQ-033-04 (`DevServerCapabilityReader`), TASK-REQ-029-02 (`ListExecutionRecords` cho `inputs.from_task`), CR-REQ-028 (`RequestClarification`)
**Status:** [x] DONE

---

## Context

Đã đọc ngày 2026-10-06:
- `agent.exec` nhận `{binary,args[],cwd,stdin,env,timeoutMs}`, **không qua shell**, `timeoutMs` kẹp `[1000,300000]`, trả `{stdout,stderr,exitCode,timedOut}` (`agent/src/relay/agent-rpc-dispatch-agent-exec.ts` dòng 66 đến 140). `fs.stat {path}`, `fs.readFile`. `fs.glob` chỉ khớp tên cuối (`find -name`): **không dùng**. `git.exec` không cho `ls-files` và cấm `\ ! < > | & ; $` trong tham số: chạy `git` qua `agent.exec binary="git"`.
- `project.proto` có `GetWorktree(GetWorktreeRequest) returns (Worktree)` (`path` ở trường 4). `infrafleet.ResolveConnectionResponse` có `repo_path`, `worktree_id`, `connection_id`.
- Kết nối tới dev server: `Relay{connection_id,method,params_json}` hoặc `RelayByDevServer{dev_server_id,...}`; `connection_id = projectID` luôn trượt (BUG-025): dùng bộ giải kết nối của SOL-005/007 (`AIConnectionResolver`, `TASK-REQ-007-04 connection_resolver.go`) trả `{ConnectionID, DevServerID}`.
- Hồ sơ năng lực: `DevServerCapabilityReader` (TASK-REQ-033-04) có `Tool(id)`, `EnvVar(name)`, `ClaudeAuth`, `HasFeature`, `Degraded`.
- Tầng, kết quả và hành động theo CR mục 2.4. `outcome` ∈ `ready|needs_info|spec_defect|env_defect`; ba kết quả không `ready` không tính lần thử. Hành động (sau cổng) do `AdvanceExecution` thực hiện ở task 08; task này chỉ trả báo cáo và ghi nó.
- Task đầu của Plan chưa có `worktree_id` (SOL-013 mục 2.4: `EnsureWorktree` chạy trong `Execute`): cổng chạy cấu trúc và ngữ nghĩa trên `repo_path`, **bỏ** kiểm worktree sạch và Check nền, thêm `finding WORKTREE_NOT_PROVISIONED` mức `info` (SOL-029 mục 1 điều 4; câu hỏi mở Q1).
- Git: chỉ dùng `rev-parse`, `status --porcelain=v1`, `ls-files` (đều có trước Git 2.25) nên không cần `GitCapabilityCache`; mọi lệnh qua `Relay`, SSH và remote chạy như `ai.complete`. Tôn trọng quy tắc đặt tuỳ chọn toàn cục trước subcommand (`guides/reference/git-compatibility.md`).

## Việc cần làm

1. `agent_relay_ports.go`: cổng `AgentRelay` (`RunCommand`, `ReadFile`, `Stat`), `AgentTarget`, `CommandSpec`, `CommandResult{Stdout, Stderr string; ExitCode int; TimedOut bool}`, `FileStat{Exists, IsDir bool}`; lỗi có kiểu `ErrAgentUnavailable` (mất kết nối) để cổng phân biệt `env_defect` với lỗi nội bộ. Cổng này cũng được SOL-030 dùng lại (không đổi tên).
2. `agent_relay.go`: cài bằng `Relay`/`RelayByDevServer` với `method="agent.exec"|"fs.readFile"|"fs.stat"`, `params_json` dựng từ struct (không nối chuỗi), `withTenantMetadata(ctx)`:
   - timeout của gRPC = `TimeoutMS + 10s`
   - giải `{stdout,stderr,exitCode,timedOut}`
   - `exitCode` null thì `-1`
   - **không bao giờ** log `Env` hay `Stdout` ngoài 200 ký tự đầu của lỗi. `Env` chỉ nhận khoá khớp danh sách cho phép (`CI`, `NO_COLOR`, `LANG`)
   - khoá khác bị từ chối.
3. `worktree_resolver.go`: `WorktreeResolver.Path(ctx, worktreeID string) (string, error)` gọi `project-service.GetWorktree` (client của TASK-REQ-012-04 hoặc 013-04 nếu đã có; nếu chưa, thêm client mỏng); lỗi `NotFound` thì `ErrWorktreeGone` (cổng: `env_defect`, mã `WORKTREE_MISSING`).
4. `readiness_gate.go`: kiểu `ReadinessGate` và `Check(ctx, GateInput) (domain.TaskReadinessReport, error)` theo solution 2.D. Khung chung: ghi giờ bắt đầu bằng `Clock`, chạy tầng theo thứ tự `structure -> semantic -> environment`, dừng khi tầng sinh ít nhất một finding **chặn** (`severity=block`), tính `outcome` (ưu tiên `needs_info` nếu mọi finding chặn thuộc loại thiếu input, ngược lại `spec_defect` cho tầng cấu trúc/ngữ nghĩa, `env_defect` cho tầng môi trường), ghi `Reports.Insert` ở mọi trường hợp (kể cả `DryRun`), trả báo cáo.
5. Tầng **cấu trúc** (`readiness_gate_tiers.go`): `GetTaskSpecs`; cờ `REQUEST_EXECUTION_CONTRACT_ENABLED` bật mà không có spec: `SPEC_MISSING`; parse + `Validate` (task 05) cho mỗi `SpecViolation` một finding; phủ `AC-n` từ `request_coverage` (CR-REQ-027) nếu task không `exempt`: `COVERAGE_GAP`.
6. Tầng **ngữ nghĩa**: lấy danh sách file một lần bằng `RunCommand{Binary:"git", Args:["ls-files","-z"], Cwd, TimeoutMS:60000}` (cache theo `(worktree_id|repo_path, head_sha)` trong bộ nhớ, TTL 5 phút; `head_sha` bằng `git rev-parse HEAD`):
   - danh sách quá 4 MB thì cắt và ghi `FILE_LIST_TRUNCATED` (mức `warn`, tắt kiểm khớp glob)
   - mỗi `scope.include` không glob: phải có trong danh sách hoặc thuộc `create`, nếu không `SCOPE_PATH_MISSING` (`spec_defect`)
   - mỗi glob khớp 0 file và không có `create` thì `SCOPE_GLOB_EMPTY` (`warn` nếu `create` không rỗng)
   - số file khớp (ước lượng) > `max_files` thì `SCOPE_TOO_BROAD`
   - lệnh Check: `binary` đầu tiên của `command` (tách bằng `shlex`-kiểu đơn giản, không hỗ trợ `&&`, `|`: lệnh chứa toán tử shell thì cần `sh -c`, đánh `CHECK_SHELL_SYNTAX` mức `warn`) phải tồn tại bằng `command -v` qua `sh -c` (Linux/macOS) hoặc `where.exe` (`platform=win32`), tên khớp `^[A-Za-z0-9._+-]{1,64}$` trước khi ghép, nếu hồ sơ năng lực nói `Known && Installed` thì bỏ qua gọi
   - `pnpm|npm run <script>` thì `ReadFile package.json` kiểm `scripts.<script>`
   - `make <target>` thì `ReadFile Makefile` kiểm regex `^<target>:`
   - phụ thuộc `depends_on` chưa `done`: đây là điều kiện chọn task của `AdvanceExecution` (SOL-013 mục 2.4 bước 2), không phải khiếm khuyết spec, nên cổng trả lỗi Go `ErrDependencyNotDone` và **không** ghi báo cáo
   - `inputs[].from_task` chưa có output: `INPUT_NOT_AVAILABLE` (`needs_info`) dựa trên `ListExecutionRecords(latest_only)` có `parse_status=ok` và `result.outputs` chứa tên.
7. Tầng **môi trường**: `Caps.Get(ctx, ref, false)`: không kết nối (`ErrCapabilityNotAvailable`) thì `DEV_SERVER_OFFLINE`:
   - `ClaudeAuth=="logged_out"` thì `CLAUDE_NOT_LOGGED_IN` (`block`), `unknown` thì `CLAUDE_AUTH_UNVERIFIED` (`warn`)
   - `MissingTools` (task 033-04) cho `requires.tools`: `TOOL_MISSING` (`block`, mã chứa tên công cụ), `unverified` thì chạy đường dự phòng `command -v`
   - `MissingEnv` cho `requires.env_names`: `ENV_MISSING` (`block`, chỉ tên)
   - worktree: `git status --porcelain=v1` qua `RunCommand`, output không rỗng thì `WORKTREE_DIRTY` (`block`)
   - `git rev-parse --abbrev-ref HEAD` khác `task/<id>` hay nhánh kỳ vọng thì `WORKTREE_WRONG_BRANCH` (`block`)
   - `base_sha` = `git rev-parse HEAD`
   - Check nền chỉ cho Check `baseline=true` (mặc định `true` cho `test|typecheck|lint` của task đầu mỗi Phase): tra `LatestBaseline(worktreeID, head_sha, now-REQUEST_READINESS_BASELINE_TTL)`, không có thì chạy tuần tự, mỗi lệnh `timeout_seconds`, tổng ≤ 8 phút (`REQUEST_VERIFY_BUDGET`), lưu vào `baseline`
   - Check nền đỏ thì `BASELINE_RED` (`block`, `env_defect`).
8. `check_readiness.go`: `CheckReadiness.Execute(ctx, taskID) (TaskReadinessReport, error)` chạy `Check` với `DryRun=true` (quyền đọc Request); `GetReadinessReport`, `ListReadiness(phaseID)` (lấy `task_ids` của Phase qua `task-service.ListTasks`, rồi `ListByTasks`).
9. Proto `request.proto`: `rpc CheckReadiness(CheckReadinessRequest{task_id}) returns (TaskReadinessReportMessage)`, `rpc GetReadinessReport(...)`, `rpc ListReadiness(ListReadinessRequest{phase_id}) returns (ListReadinessResponse)`; message `TaskReadinessReportMessage{id, request_id, task_id, attempt, outcome, tier, repeated Finding findings, base_sha, head_sha, spec_digest, duration_ms, dry_run, created_at}`. Tên kênh WS `readiness.check|get|list` do CR-REQ-016 chốt (không thêm ở đây). Handler tự kiểm quyền (README v6 mục 8 điều 13).

## Kiểm thử

- Tầng cấu trúc: `TestGate_Structure_SpecMissing`, `_V1SpecRejected`, `_NoCheck`, `_CoverageGap`.
- Tầng ngữ nghĩa với fake `AgentRelay`: `_ScopePathMissing_SpecDefect`, `_PathInCreateOk`, `_GlobMatches`, `_GlobEmptyWithCreateWarns`, `_TooBroad`, `_CheckBinaryMissing`, `_WhereExeOnWindows`, `_ShellInjectionNameRejected` (tên có `;`), `_PackageScriptMissing`, `_MakeTargetMissing`, `_InputNotAvailable_NeedsInfo`, `_FileListTruncated`.
- Tầng môi trường với fake `DevServerCapabilityReader`: `_ToolMissing_EnvDefect` (thiếu `go`), `_ToolUnverifiedFallsBackToCommandV`, `_EnvMissingNeverEchoesValue`, `_ClaudeLoggedOut`, `_DevServerOffline`, `_WorktreeDirty`, `_WrongBranch`, `_BaselineGreenCached`, `_BaselineRed_EnvDefect`, `_FirstTaskNoWorktreeSkipsWorktreeChecks`.
- `TestGate_StopsAtFirstFailingTier` (tầng sau không gọi relay), `_ReadyWhenAllGreen`, `_WritesOneReportPerRun`, `_DryRunFlagged`, `_NoSideEffectsOnRelay` (fake chỉ nhận lệnh đọc: `ls-files`, `rev-parse`, `status`, `command -v`).
- `TestAgentRelay_BuildsAgentExecParams` (không qua shell; `Env` ngoài danh sách bị từ chối), `_UsesRelayByDevServerWhenNoConnection`.
- Handler: `TestServer_CheckReadiness_Authz`, `_ListReadiness_ByPhase`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/usecase/... -run Gate` và `go test ./services/request-service/internal/adapter/...`.

## Tiêu chí hoàn thành

- [x] Thiếu Check: `spec_defect`; đường dẫn `scope` không tồn tại ngoài `create`: `spec_defect`; thiếu `go` trong hồ sơ: `env_defect`; input chưa có: `needs_info`; đủ điều kiện: `ready`.
- [x] Mỗi lần chạy một dòng `task_readiness_reports`; `AdvanceExecution` lặp không tạo hai Clarification (kiểm ở task 08).
- [x] Không finding nào chứa giá trị biến môi trường; không log `Env`.
- [x] Mọi lệnh qua `agent.exec` có `binary`/`args` tách; không chuỗi shell nối từ dữ liệu người dùng.
- [x] Cổng không thực hiện lệnh ghi (kiểm bằng fake chỉ cho danh sách lệnh đọc).

## Rủi ro và lưu ý

- `ErrDependencyNotDone` (mục 6) là quyết định của task này, chưa được SOL-013 xác nhận; nếu SOL-013 đã lọc phụ thuộc trước khi gọi cổng thì nhánh này không bao giờ chạy và có thể bỏ.
- Độ trễ cổng (vài lệnh `agent.exec`, có thể một lượt Check nền tới 5 phút) chưa đo; `relay-ssh` đẩy `agent.js` mỗi lần dựng phiên nên lần đầu chậm.
- Hồ sơ năng lực chỉ biết `claude auth` ở dev server có agent mới; agent cũ thì `unverified` (không chặn).
- `git ls-files` trên monorepo lớn có thể vài giây và hàng chục MB; giới hạn 4 MB và cache theo `head_sha` là biện pháp, chưa đo.
- Độc lập nhánh: nhánh kỳ vọng `task/<id>` là giả định theo `EnsureWorktree`; đọc `WorktreeProvisioner` ở `task-service` để biết tên nhánh thật trước khi cứng hoá.
