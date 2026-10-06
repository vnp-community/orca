# BE-CV-TASK-080-05: Use case `AutoRefreshIndex` và bộ phát slot đến hạn

**From Solution:** BE-CV-SOL-080
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/auto_refresh_index.go`, `auto_refresh_index_test.go`, `internal/adapter/eventbus/auto_refresh_ticker.go` (mới)
**Depends on:** BE-CV-TASK-080-03, 080-04, BE-CV-SOL-012-target-resolution-and-bindings, BE-CV-SOL-013-authorization-flags-and-audit, BE-CV-SOL-021-agent-collector
**Status:** [ ] TODO

## Context
Luồng [1]-[7] của SOL-080 2.C. Cổng tiêu thụ: `FlagReader`, `AgentStatusReader` (`codeintel.status`), `AgentReindexer` (`codeintel.reindex` với `trigger:"agent_done"`, `ifStale:true`, `expectHead`; không có `tiers`, PQ-16), `AutoRefreshBudget`, `AuditWriter`.

## Việc cần làm
1. `OnAgentStatus(ctx, ev)`: cờ (lỗi=tắt) → binding theo `worktree_id` (+ khớp `dev_server_id`) → `status=running` ⇒ `CancelQueuedSlot`; `status ∈ idle|completed|waiting|error|stopped` ⇒ `UpsertSlot`.
2. `OnSlotDue(job)`: `AgentStatusReader` → `PlanRefresh` → ngân sách → `Reindex` hoặc Finish (`already_up_to_date`, `skipped_scope_repo_root`, `cancelled(deferred)`); lưu `agent_job_id`.
3. Hoãn một lần rồi `cancelled(message=deferred)`; vượt ngân sách: `failed`, `CODEINTEL_RATE_LIMITED`.
4. Audit `actor_type=system` (đưa vào `details` tới khi SOL-013 sửa `auditclient`).
5. Ticker mọi replica, `CODEINTEL_AUTOREFRESH_TICK`.

## Kiểm thử
- Unit đồng hồ giả + cổng giả: 10 sự kiện/20 s → 1 slot; worktree liên kết → 0 lời gọi reindex; đang `running` không kill; deferral; hạn mức.

## Tiêu chí hoàn thành
- [ ] Các tiêu chí mục 4 của SOL-080 liên quan use case xanh.

## Rủi ro
Thứ tự sự kiện giữa replica không bảo đảm; thiết kế dựa trạng thái nên chịu được.
