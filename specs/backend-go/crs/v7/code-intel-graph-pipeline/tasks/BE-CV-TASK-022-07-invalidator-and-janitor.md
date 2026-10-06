# BE-CV-TASK-022-07: `SnapshotInvalidator` và janitor

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/usecase/{snapshot_invalidator.go,snapshot_janitor.go}` (mới) và test
**Depends on:** TASK-022-03, TASK-022-04
**Status:** [ ] TODO

---

## Context

SOL-024 gọi `InvalidateBinding`/`InvalidateProbe`; bảo trì §4.3 (10 phút, lô 500).

## Việc cần làm

1. `InvalidateBinding(ctx, tenant, binding, reason)` xoá DB + probe + LRU, idempotent; `InvalidateProbe` chỉ bộ nhớ.
2. Janitor goroutine theo `CODEINTEL_MAINTENANCE_INTERVAL`, dừng theo ctx, `DeleteExpired` lô 500, `EvictOldest`.

## Kiểm thử

- `go test ./internal/usecase/ -run 'Invalidat|Janitor' -race`; gọi hai lần không lỗi; janitor dừng khi ctx huỷ.

## Tiêu chí hoàn thành

- [ ] Idempotent. - [ ] Không xoá snapshot mới nhất mỗi view.

## Rủi ro và lưu ý

- Dùng `withMaintenanceTx` (SOL-011).
