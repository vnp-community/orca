# BE-CV-TASK-022-08: Cấu hình, metrics, nối vào handler và test tích hợp hai dialect

**From Solution:** BE-CV-SOL-022-snapshot-cache
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/config/config.go` (sửa), `.../internal/adapter/grpc/server_graph_views.go` (sửa), test tích hợp
**Depends on:** TASK-022-05, 022-06, 022-07, BE-CV-TASK-021-09
**Status:** [x] DONE

---

## Context

Nối `CachedViewReader` thay `ViewReader` trực tiếp ở handler; chạy ma trận hai dialect.

## Việc cần làm

1. Thêm 9 biến cấu hình (2.E) với mặc định.
2. Metrics (2.E).
3. Handler dùng `CachedViewReader`.
4. Tích hợp `-tags=integration` PG và MySQL: miss → hit không gọi agent; invalidate; tenant khác không thấy.

## Kiểm thử

- `go test -tags=integration ./... -run CacheIntegration -race` (matrix dialect).

## Tiêu chí hoàn thành

- [x] Hai dialect xanh. - [x] Cô lập tenant có test.

## Rủi ro và lưu ý

- Chưa chạy test nào.
