# BE-CV-TASK-092-07: Use case `GetRequirementTrace`

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/usecase/get_requirement_trace.go` (mới)
**Depends on:** BE-CV-TASK-092-04, 092-05, 092-06, BE-CV-SOL-036, BE-CV-SOL-082
**Status:** [x] DONE

## Việc cần làm
1. Ghép: resolver → provider (Task; Request nếu bật) → overlay + runs + trailer → matcher → `TraceLink` ghép lúc đọc.
2. **Không** ghi `graph_snapshots(requirementTrace)` (L3); không cache chữ tiêu chí.
3. `include_inferred=false` loại bằng chứng `inferred`; `task_id` tay đi qua `ResolvePermission`.
4. `warnings`: `no_structured_criteria`, `index_stale`, `request_service_unavailable`, `commit_trailers_unavailable`; mọi chuỗi qua `TextRedactor`.

## Kiểm thử
- Use case với port giả cho mọi tiêu chí chấp nhận SOL-092 §4 (worktree không liên kết; không quyền; `inferred`-only; index cũ; có change không test).

## Tiêu chí hoàn thành
- [x] `ai_context` không xuất hiện đầu ra nào; [ ] `unknown` ≠ `no_evidence`; [ ] ≤ 1 `ListWorktrees`, 1 `ResolvePermission`, 1 `GetTask` mỗi lần gọi.

## Rủi ro
- Thiếu overlay (CR-036 chưa có) ⇒ phần lớn `unknown` tới khi merge.
