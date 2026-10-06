# BE-CV-TASK-080-04: Repository slot debounce trên `reindex_jobs` (hai dialect)

**From Solution:** BE-CV-SOL-080
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/auto_refresh_ports.go`, `internal/adapter/postgres/reindex_job_auto_refresh.go`, `internal/adapter/mysql/reindex_job_auto_refresh.go` (mới); thêm `GetByWorktreeID` cho `repo_bindings` nếu SOL-011 chưa có
**Depends on:** BE-CV-SOL-011 (bảng `reindex_jobs`, `repo_bindings` migration `0002`), BE-CV-TASK-080-03
**Status:** [ ] TODO

## Context
C-DM T7: `active_key` UNIQUE, `trigger`, `trigger_event_id`, `requested_by NULL`. Slot = dòng `queued`, `trigger='agent_done'`. Không file của SOL-011 bị sửa; thêm file riêng. Điểm hợp đồng thiếu: chỉ mục `(tenant_id, worktree_id)` và `(tenant_id, created_at)` (SOL-080 mục 7).

## Việc cần làm
1. Cổng `AutoRefreshSlotRepository`: `UpsertSlot`, `CancelQueuedSlot`, `ClaimDueSlots(limit, quiet, maxWait)`, `Release(job, deferred)`, `CountAutoRefreshSince(tenant, since)` (`trigger='agent_done' AND outcome=''`).
2. `UpsertSlot`: INSERT; xung đột `active_key` → nếu dòng là `queued ∧ agent_done` thì chạm `updated_at=now()`, `trigger_event_id`, `version+1`; ngược lại trả `Busy`.
3. `ClaimDueSlots`: chạy trong `withMaintenanceTx`, đồng hồ DB (PG `now()`, MySQL `CURRENT_TIMESTAMP(6)`), CAS `WHERE id=? AND tenant_id=? AND version=? AND status='queued'`.
4. Mọi truy vấn tenant-scoped có `tenant_id`; Postgres `set_config('app.tenant_id', $1, true)`.

## Kiểm thử
- Integration `-tags=integration`, `dialect: [postgres, mysql]`: upsert đồng thời, hai replica claim (một thắng), cô lập tenant (role `NOSUPERUSER NOBYPASSRLS`), dùng đồng hồ DB.

## Tiêu chí hoàn thành
- [ ] Bộ kịch bản chung chạy xanh cả hai dialect.

## Rủi ro
Quét `ClaimDueSlots` nhiều tenant chưa đo; thiếu chỉ mục hợp đồng thì chậm.
