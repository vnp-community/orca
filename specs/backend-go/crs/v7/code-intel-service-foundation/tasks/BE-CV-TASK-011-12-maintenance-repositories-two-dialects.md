# BE-CV-TASK-011-12: Repository bảo trì: `withMaintenanceTx`, xoá hết hạn, mồ côi, job mồ côi, binding rảnh (hai dialect)

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/postgres/{maintenance_tx.go,maintenance_repository.go}`, `internal/adapter/mysql/maintenance_repository.go` (+ `_integration_test.go`) (mới)
**Depends on:** BE-CV-TASK-011-02, 011-03, 011-07, 011-08, 011-09, 011-10
**Status:** [ ] TODO

---

## Context

Hợp đồng §4.3; cờ `app.maintenance` là mới (chưa có ở mã hiện có; SOL-011-repositories C1). Postgres: `withMaintenanceTx` đặt `set_config('app.maintenance', 'on', true)`; MySQL không RLS. Việc xuyên tenant chỉ gồm: xoá snapshot hết hạn, xoá outbox đã publish, xoá `processed_events` cũ, liệt kê tenant. Việc còn lại chạy trong `withTenantTx` của từng tenant (SOL-011-data D3).

## Việc cần làm

1. Postgres `maintenance_tx.go`: `withMaintenanceTx`; test AST (010-06) bổ sung nhận `withMaintenanceTx`.
2. `MaintenanceRepository` Postgres: `ListTenantsWithData` (`SELECT DISTINCT tenant_id` từ `graph_snapshots`, `repo_bindings`, `reindex_jobs` — chính sách `maint_read`), `DeleteExpiredSnapshots(batch)` (`DELETE … WHERE id IN (SELECT id … WHERE expires_at < now() ORDER BY expires_at LIMIT $1)`), `DeletePublishedOutbox(olderThan, batch)`, `DeleteOldProcessedEvents(olderThan, batch)`.
3. MySQL tương đương: `DELETE … WHERE expires_at < CURRENT_TIMESTAMP(6) ORDER BY expires_at LIMIT ?` (viết trực tiếp, không subquery có `LIMIT`); `ListTenantsWithData` bằng `UNION` ba bảng.
4. `TenantMaintenanceRepository` (cả hai dialect, ctx đã có tenant): `DeleteWrongSchemaSnapshots`, `EvictOldestSnapshotsOverQuota` (tổng `payload_bytes` > quota → xoá cũ nhất, không xoá mới nhất mỗi view), `DeleteOrphans` (`NOT EXISTS` theo `repo_binding_id`, `created_at < olderThan`, bảng `graph_snapshots`, `review_states`, `reindex_jobs`), `FailOrphanedReindexJobs` (`status IN ('queued','running') AND updated_at < staleBefore` → `failed`, `error_code='CODEINTEL_REINDEX_ORPHANED'`, `active_key=NULL`, `finished_at=now`; trả danh sách job để use case phát sự kiện), `DeleteIdleBindings` (`COALESCE(last_status_at, updated_at) < idleBefore`).
5. Mọi hàm nhận `batch` (500) và trả số dòng đã xoá; vòng lặp lô do use case (011-13).

## Kiểm thử

- Integration hai dialect: dữ liệu có hạn rút ngắn; từng hàm xoá đúng tập mục tiêu, không đụng dòng còn hạn; `FailOrphanedReindexJobs` giải phóng `active_key` (tạo job mới được ngay); `DeleteOrphans` giữ `c4_overrides`/`finding_dismissals`; hạn mức xoá cũ nhất.
- Postgres, role không superuser: không đặt `app.maintenance` → `DELETE` bị chính sách chặn (0 dòng); đặt cờ → chỉ xoá dòng khớp điều kiện hàng; `INSERT` bị từ chối.
- Lệnh: `go test -tags=integration ./services/code-intel-service/internal/adapter/... -run Maintenance`.

## Tiêu chí hoàn thành

- [ ] Các việc bảo trì ở SOL mục 2.C chạy đúng ở hai dialect.
- [ ] Chính sách Postgres chặn xoá ngoài điều kiện; không cho `INSERT`.
- [ ] Không phương thức nào xoá bản snapshot mới nhất mỗi view.

## Rủi ro và lưu ý

- `NOT EXISTS` trên bảng lớn chưa đo; luôn có `LIMIT`.
- `UNION` liệt kê tenant có thể chậm khi nhiều tenant; chấp nhận ở MVP.
