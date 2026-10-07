# TASK-REQ-026-04: Cổng điều kiện `EngineReadinessGate` và adapter `DevServerExecutor`

**From Solution:** BE-REQ-SOL-026
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/engine_readiness_gate.go`, `internal/usecase/ports.go` (sửa), `internal/adapter/grpcclient/dev_server_executor.go`, `internal/domain/semver_compare.go` và test (mới)
**Depends on:** TASK-REQ-026-01, TASK-REQ-007-04 (`ProjectContextResolver`, adapter Relay), TASK-REQ-008-02 (adapter `agent.execPrompt`)
**Status:** [x] DONE

---

## Context

Đã đọc: `proto/orca/infrafleet/v1/infrafleet.proto` (`ResolveConnectionResponse{connected, dev_server, repo_path, worktree_id, connection_id}` dòng 562; `RelayRequest{connection_id, method, params_json}` dòng 819, `RelayResponse{result_json}`), `services/infra-fleet-service/internal/usecase/check_dev_server_preflight.go` (tiền lệ: kiểm git, node, đĩa, cổng, `gh` bằng `shell.exec`) và `agent/src/relay/agent-rpc-dispatch-agent-exec.ts` (`agent.exec` nhận `{binary, args, cwd, stdin, env, timeoutMs}`, trần 5 phút). CR-REQ-026 chọn `agent.exec` chứ không `shell.exec` vì cần `binary`/`args` có cấu trúc, tránh chèn lệnh.

Adapter Relay của SOL-007 (`ai_completion_relay.go`) đã dựng khung gọi `ResolveConnection` rồi `Relay`; task này thêm một adapter **tổng quát hơn** cho `agent.exec` mà không đổi adapter cũ. Tên: `DevServerExecutor` (không `utils`). Chưa kiểm chứng: dạng chính xác của `result_json` của `agent.exec` (CR nói `{stdout, stderr, exitCode, timedOut}` cho `execPrompt`; `agent.exec` phải đọc `agent-rpc-dispatch-agent-exec.ts` lúc làm, không suy đoán).

## Việc cần làm

1. `ports.go`: `type DevServerExecutor interface { Exec(ctx context.Context, connectionID string, in ExecInput) (ExecResult, error); ExecPrompt(ctx context.Context, connectionID string, in ExecPromptInput) (ExecResult, error) }`;
   - `ExecInput{Binary string; Args []string; Cwd string; Env map[string]string; TimeoutMs int}`
   - `ExecPromptInput{Prompt, WorktreePath, TrustPreset string; Env map[string]string; TimeoutMs int}`
   - `ExecResult{Stdout, Stderr string; ExitCode int; TimedOut bool}`.
   - Thêm `ConnectionResolver{Resolve(ctx, projectID string) (Connection, error)}` với `Connection{Connected bool; ConnectionID, DevServerID, RepoPath string}` nếu SOL-007 chưa có (đọc `ports.go` trước, dùng lại nếu đã có).
2. `adapter/grpcclient/dev_server_executor.go`: gọi `infrafleet.Relay` với `method:"agent.exec"` hoặc `"agent.execPrompt"`, `params_json` dựng bằng `encoding/json` từ struct (không nối chuỗi), parse `result_json` vào `ExecResult`;
   - `deadline` của ctx = `TimeoutMs + 10s`
   - lỗi gRPC `Unavailable` trả `REQUEST_ENGINE_NO_CONNECTION`.
3. `domain/semver_compare.go`: `ParseSemver(s string) (Semver, error)` (lấy `\d+\.\d+\.\d+` đầu tiên trong chuỗi, bỏ tiền tố `v`), `CompareSemver(a, b Semver) int`.
4. `engine_readiness_gate.go`: `type EngineReadinessGate struct{ conns ConnectionResolver; exec DevServerExecutor; stat RepoPathStatter; clock Clock; cache *preflightCache; minVersion string; ttl time.Duration }`;
   - `Check(ctx, p ProjectRef, settings *domain.ProjectEngineSettings) (PreflightReport, error)`.
   - Thứ tự: (a) `Resolve`
   - `Connected=false` thì `REQUEST_ENGINE_NO_CONNECTION`
   - (b) `Exec{Binary:"openspec", Args:["--version"], Cwd: RepoPath, TimeoutMs:10000}`: `ExitCode!=0` hoặc lỗi chạy thì `REQUEST_ENGINE_OPENSPEC_MISSING`
   - so phiên bản với `settings.MinVersion` (nếu rỗng thì dùng biến môi trường `REQUEST_OPENSPEC_MIN_VERSION`, rỗng nữa thì không so)
   - thấp hơn thì `REQUEST_ENGINE_OPENSPEC_VERSION`
   - (c) `RepoPathStatter.IsDir(ctx, connectionID, RepoPath+"/openspec")` (dùng Relay `fs.stat`, hoặc `StatFile` của git-gateway nếu có `worktree_id`): không có thì `REQUEST_ENGINE_OPENSPEC_NOT_INITIALIZED`, **không** tự `openspec init`
   - (d) `Exec{Binary:"claude", Args:["--version"]}`: thiếu thì `REQUEST_ENGINE_CLAUDE_MISSING`
   - (e) đăng nhập `claude`: một `ExecPrompt{Prompt:"Reply with the single word OK.", WorktreePath: RepoPath, TrustPreset:"default", TimeoutMs:60000}`, `ExitCode!=0` thì `REQUEST_ENGINE_CLAUDE_NOT_AUTHENTICATED`
   - chỉ chạy khi bước (d) đạt và cache không còn hiệu lực
   - (f) cảnh báo git bẩn: không chặn, bỏ qua trong task này nếu `git-gateway` chưa có client (ghi `Warnings` rỗng).
5. `PreflightReport{OK bool; Failures []PreflightFailure; Warnings []string; CheckedAt time.Time; OpenSpecVersion string}`, `PreflightFailure{Code, Message, FixHint string}`.
   - Không dừng ở lỗi đầu tiên với bước (b) đến (d): thu thập hết (trừ khi bước (a) lỗi)
   - bước (e) chỉ chạy khi (b) đến (d) đều đạt (tránh tốn token).
6. Cache: `preflightCache` khoá `(devServerID, repoPath)`, TTL 10 phút theo `Clock`;
   - chỉ cache báo cáo **đạt** (báo cáo lỗi không cache để người dùng sửa xong thử lại ngay)
   - bảo vệ bằng `sync.Mutex`, singleflight theo khoá để hai `GenerateSolution` đồng thời không chạy hai lượt `execPrompt`.
7. `Preflight(...)` của engine trả `REQUEST_ENGINE_PREFLIGHT_FAILED` (`FailedPrecondition`, `details` là danh sách `PreflightFailure` dạng `errdetails` hoặc chuỗi JSON theo cách `apperrors` đang hỗ trợ; kiểm `common/apperrors/apperrors.go`) khi `!OK`.
8. Không ghi cache hay log nội dung `Stderr` dài: cắt 2 KB, che bí mật bằng `RedactSecrets` (TASK-REQ-008-01) trước khi đưa vào `Message`.

## Kiểm thử

- `TestGate_AllPass_CachesFor10Minutes` (fake clock tiến 9 phút còn cache, 11 phút chạy lại)
- `TestGate_NoConnection`
- `TestGate_OpenSpecMissing`
- `TestGate_OpenSpecVersionTooLow` (bảng `0.9.9` < `1.0.0`
- `v1.2.3`, chuỗi nhiễu `openspec 1.2.3 (build x)`)
- `TestGate_NotInitialized`
- `TestGate_ClaudeMissing`
- `TestGate_ClaudeNotAuthenticated_OnlyAfterOtherChecksPass`.
- `TestGate_FailureNotCached`
- `TestGate_ConcurrentCheck_SingleExecPrompt` (20 goroutine, đếm lần gọi fake `ExecPrompt` bằng 1; chạy `-race`).
- `TestGate_BinaryAndArgsNeverJoinedIntoShell` (fake ghi lại `ExecInput`, khẳng định `Binary` là `openspec` và `Args` là slice).
- `TestDevServerExecutor_ParsesResult` (fake `infrafleet.Relay` trả `result_json`)
- `TestDevServerExecutor_UnavailableMapsToNoConnection`.
- `TestCompareSemver_Table`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test -race ./services/request-service/internal/usecase/... -run 'Gate|Semver' && go test ./services/request-service/internal/adapter/grpcclient/... -run DevServerExecutor`.

## Tiêu chí hoàn thành

- [x] Từng mã lỗi ở bảng CR 2.3 có test và `fix_hint` không rỗng.
- [x] Không có chuỗi shell nào được ghép; `Binary` và `Args` tách riêng.
- [x] Cache 10 phút đúng theo `Clock`; chỉ lưu báo cáo đạt; `-race` sạch.
- [x] `execPrompt` kiểm đăng nhập chỉ chạy khi các kiểm khác đạt.
- [x] `PreflightReport` không chứa bí mật (test với `Stderr` chứa `ghp_...`).

## Rủi ro và lưu ý

- Kiểm đăng nhập bằng `execPrompt` tốn một lượt gọi model mỗi 10 phút mỗi dev server; chấp nhận cho tới khi có lệnh không tốn token (Q3). Cờ `REQUEST_ENGINE_SKIP_CLAUDE_AUTH_PROBE` (mặc định tắt) cho môi trường đã chắc chắn, ghi trong README của service.
- `openspec --version` có thể in ra cả banner; regexp lấy dãy số đầu tiên, nên chuỗi như `node v20.1.0` nếu in chung sẽ sai. Test bằng mẫu thật khi cài được OpenSpec (chưa kiểm chứng).
- `agent.exec` trần 5 phút; mọi lệnh ở đây đều ngắn, nhưng `openspec archive` (task 026-07) có thể dài.
- Cache theo `repoPath` không tính biến môi trường của dev server; đổi `PATH` giữa hai lần kiểm có thể làm cache lệch trong 10 phút.
