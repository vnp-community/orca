# BE-CV-TASK-092-05: Cổng `RequirementProvider`, adapter project/task-service, khung request-service, nguồn trailer

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/usecase/requirement_trace_ports.go`, `internal/adapter/grpcclient/task_requirement_provider.go`, `request_requirement_provider.go` (khung + fake) (mới)
**Depends on:** BE-CV-TASK-092-03, BE-CV-SOL-012-target-resolution-and-bindings
**Status:** [ ] TODO

## Context
Dữ liệu thật/chưa có: SOL-092 §1. `GetTask` **không** kiểm grant (`get_task.go`); `GetWorktree` **không** lọc tenant (L1).

## Việc cần làm
1. `WorktreeTaskResolver`: link tay (T15) → `ListWorktrees(project_id)` lọc `id==binding.worktree_id` → `task_id`; `linked_issue_*` không `task_id` ⇒ `no_reverse_lookup`.
2. `TaskReader`: `ResolvePermission(task_id, user_id, "read")` **trước** `GetTask`; mọi lỗi ⇒ `task_not_readable` (fail closed); `task_id` do client phải cùng project.
3. `FindTaskByNumber`/`GetTaskSource` cho nhánh `inferred`.
4. `CommitTrailerSource` (port) + chỉ adapter `git.history` khi Q1 SOL-092 được duyệt; thiếu ⇒ `commit_trailers_unavailable`.
5. `request_requirement_provider`: khung sau cờ `CODEINTEL_REQUIREMENT_SOURCE_REQUEST_ENABLED` (mặc định `false`) + fake `GetRequestCoverage`; **không** import mã v6; rơi về nhánh Task kèm `request_service_unavailable`.
6. Client gRPC đặt deadline và `withTenantMetadata` tương tự mẫu `git-gateway/adapter/grpcclient`.

## Kiểm thử
- Fake hai người dùng (có/không grant); `ResolvePermission` lỗi; worktree không `worktree_id`; `task_id` khác project.

## Tiêu chí hoàn thành
- [ ] không rò title khi không có grant; [ ] không gọi `GetWorktree`; [ ] nhánh Request tắt không phát RPC nào.

## Rủi ro
- `request-service` chưa tồn tại; mã lỗi `errNoGrant` cụ thể chưa đọc hết.
