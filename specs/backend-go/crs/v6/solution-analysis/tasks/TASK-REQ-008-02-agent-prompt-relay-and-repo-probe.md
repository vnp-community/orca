# TASK-REQ-008-02: Adapter `agent.execPrompt` (chỉ đọc) và `RepoStateProbe`

**From Solution:** [BE-REQ-SOL-008](../solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) mục C
**Priority:** P1
**Service/Area:** `request-service` / adapter grpcclient, ports
**File:** `internal/adapter/grpcclient/agent_prompt_relay.go` (mới), `repo_state_probe.go` (mới), `internal/usecase/ports.go` (sửa), và `_test.go`
**Depends on:** TASK-REQ-007-04 (`connection_resolver.go`, `withTenantMetadata`)
**Status:** [x] DONE

## Context

- `agent.execPrompt`: params `stepId, prompt, worktreePath, trustPreset, model?, accountId?, env?, timeoutMs?`; kết quả `{stdout, stderr, exitCode, timedOut, stepId}` (doc comment `task-service/internal/adapter/grpcclient/simple_executor.go`). `SimpleExecutor` đặt `trustPreset:"full"`; adapter này **không bao giờ** đặt `full`.
- `git-gateway` `GetStatus` nhận `worktree_id` (không phải đường dẫn) và chỉ trả `files[]` và `branch` (`gitgateway.proto` dòng 182-194): không có `HEAD`.
- `ResolveConnectionResponse` có `repo_path` và `worktree_id`; `worktree_id` có thể rỗng.

## Việc cần làm

1. Port `AgentPromptRunner` và `RepoStateProbe` như solution mục C; `RepoSnapshot.Equal` so `Branch` và danh sách `(Path, State)` đã sắp xếp.
2. `agent_prompt_relay.go`: hằng `trustPresetReadonly = "default"`; dựng `params_json` với `stepId=runID`, `worktreePath=repoPath`, `env={ORCA_REQUEST_ID, ORCA_PROJECT_ID}`, `timeoutMs`; không nhận `trustPreset` từ bên gọi; không nhận `env` tuỳ ý (chỉ hai khoá cố định). Phân tích kết quả thành `AgentPromptResult`; kết quả không phải JSON thì lỗi bọc.
3. Cờ `REQUEST_AGENT_READONLY_USE_AGENT_FLAG` (mặc định tắt): khi bật thêm khoá `readOnly:true` vào `params_json`. Tên khoá do CR-REQ-033 chốt; để hằng `agentReadonlyParam` kèm TODO tham chiếu CR-REQ-033, không giả định.
4. `repo_state_probe.go`: gọi `GetStatus{worktree_id}` qua client git-gateway; `worktree_id==""` thì trả `ErrProbeUnavailable` (usecase đánh dấu `repo_check=skipped`).
5. Giới hạn kích thước `stdout` đọc vào bộ nhớ (256 KB, cắt thêm).

## Kiểm thử

- Fake client: `params_json` đúng từng khoá; `trustPreset` luôn `"default"` (test cố ép giá trị khác phải không thể, qua kiểu); `env` chỉ hai khoá; cờ agent bật/tắt.
- Probe: `Equal` với thứ tự khác nhau; `worktree_id` rỗng trả `ErrProbeUnavailable`.
- Lệnh: `go test ./internal/adapter/grpcclient/... -run "AgentPrompt|RepoState"`.

## Tiêu chí hoàn thành

- [x] Không có đường mã nào đặt `trustPreset="full"` (test và `grep` trong review).
- [x] `env` không chứa token hay biến nhạy cảm.
- [x] Probe nêu rõ không kiểm được khi thiếu `worktree_id`.
- [x] Không thêm lệnh Git mới (`guides/reference/git-compatibility.md`).

## Rủi ro và lưu ý

- HEAD không được so sánh ở v1 (câu hỏi mở 1): ghi vào PR.
- `repo_path` có thể là checkout đang được dùng; so sánh trước và sau, không so với "sạch".
