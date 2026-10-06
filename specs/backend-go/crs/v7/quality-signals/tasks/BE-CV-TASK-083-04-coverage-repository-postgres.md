# BE-CV-TASK-083-04: Repository Postgres `coverage_reports`

**From Solution:** BE-CV-SOL-083
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/coverage_ports.go`, `internal/adapter/postgres/coverage_repository.go` (mới)
**Depends on:** BE-CV-TASK-083-02, 083-03
**Status:** [ ] TODO

## Context
Cổng `CoverageRepository`: `Upsert`, `GetByRun`, `LatestByHead(tenant, binding, head, dirty, treeHash)`, `Prune(before, perBindingCap, limit)`.

## Việc cần làm
1. `Upsert` `ON CONFLICT (unique) DO UPDATE` (cập nhật `quality_run_id`, `payload`, `expires_at`).
2. Mọi câu `WHERE tenant_id`; `set_config('app.tenant_id', $1, true)`.
3. `Prune`: hết hạn + giữ 20 báo cáo mới nhất mỗi binding, lô 500.

## Kiểm thử
- Bộ hợp đồng 083-06 trên Postgres.

## Tiêu chí hoàn thành
- [ ] Idempotent: chạy lại cùng khoá không nhân đôi.

## Rủi ro
`payload` JSONB ≤ 1 MiB: kiểm `payload_bytes` trước ghi.
