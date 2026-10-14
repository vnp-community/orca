# TASK-TDD-002-T2-03: ProposalWorkspace và DevServerExecutor thật (thay stub)

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 2)
**Checklist:** [T2.3](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** request-service (Go, adapter grpcclient)
**Files:** `internal/usecase/{ports_workspace,ports_engine}.go`, `internal/adapter/grpcclient/{proposal_workspace,dev_server_executor}.go`, test đi kèm, git-gateway proto `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (đọc)
**Depends on:** T2.2
**Status:** `[ ]` TODO

---

## Context

Hai adapter hiện là **stub**:
- `ProposalWorkspace` (`proposal_workspace.go`): mọi hàm trả `nil`/rỗng; chữ ký còn lệch với CR-026 mục F.
- `DevServerExecutor` (`dev_server_executor.go`): `Exec` trả cứng `"openspec version 1.2.3"` với exit 0, `ExecPrompt` trả `"OK"`. Hệ quả: cổng readiness `EngineReadinessGate.Check` hiện **luôn đạt**, không phản ánh dev server thật.
Phải thay bằng mã thật trước khi dùng engine. Git-gateway có sẵn: `CreateWorktree` (cần `project_id`, `repo_id`, `branch`, `base_ref`, tuỳ chọn `idempotency_key`, `name`, `path`), `RemoveWorktree`, `ReadFile`/`WriteFile`/`ReadDir`/`StatFile` (`worktree_id` + `path`), `GetStatus`, `Commit`, `Discard`/`BulkDiscard`. `agent.exec` nhận `{binary, args, cwd, stdin, env, timeoutMs}` (trần 5 phút).

## Việc cần làm

1. Chốt chữ ký port `usecase.ProposalWorkspace` theo CR-026 mục F: `Ensure(ctx, EnsureWorkspaceInput) (Workspace, error)`, `ReadFile(ctx, ws, relPath, maxBytes)`, `WriteFile`, `ChangedPaths(ctx, ws)`, `DiscardAll`, `Commit(ctx, ws, message, paths) (sha, error)`, `Remove`; `Workspace` mang `WorktreeID`, `Path` (đường dẫn trên dev server), `Branch`.
2. `Ensure`: gọi `CreateWorktree` với `branch = request/<số>-proposal`, `base_ref` = nhánh mặc định của repo, `idempotency_key = <run id>` (lặp lại không tạo thêm); **không** dùng `CreateWorktreeFromIssue`.
3. `ReadFile` có `maxBytes` (≤ 256 KiB); `ChangedPaths` qua `GetStatus` gồm cả tệp mới (untracked), đổi tên, xoá; `DiscardAll` dùng `GetStatus` rồi `BulkDiscard`, xử lý riêng tệp untracked; `Commit` chỉ các `paths` được truyền; `Remove` gỡ worktree nhưng giữ nhánh.
4. `DevServerExecutor.Exec`: relay `agent.exec` qua infra-fleet (`Relay` hoặc `RelayByDevServer` như `AgentPromptRelay`), không ghép chuỗi shell, trần thời gian; trả `Stdout`, `Stderr`, `ExitCode`, `TimedOut`. Bỏ `ExecPrompt` giả (hoặc nối vào `AgentPromptRunner`).
5. Ánh xạ lỗi sang `REQUEST_ENGINE_*` (ví dụ không phân giải được repo: `REQUEST_ENGINE_REPO_NOT_RESOLVED`).
6. Sửa `EngineReadinessGate` để dùng `Exec` thật và đọc phiên bản bằng `domain.ParseSemver` trên đầu ra thực của `openspec --version`.

## Tiêu chí hoàn thành

- [ ] Không còn hàm trả `nil`/giá trị cứng trong hai adapter.
- [ ] Readiness báo lỗi thật khi `openspec` không có hoặc quá cũ (kiểm bằng dev server giả trả lỗi).
- [ ] `Ensure` idempotent; `ChangedPaths` thấy tệp mới do agent tạo; `Remove` không xoá nhánh.

## Kiểm thử

- Test đơn vị với git-gateway giả (bufconn) và infra-fleet giả: từng hàm, từng mã lỗi.
- Test tích hợp trên repo tạm với git-gateway thật nếu có thể chạy cục bộ (tag `integration`).

## Rủi ro và ghi chú

- Nhánh `request/<số>-proposal` cục bộ, không bao giờ push (chặn push ở T4.1).
- Đọc `proposal_scope_check` (đã có) khi dùng `ChangedPaths`.
