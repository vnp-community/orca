# BE-CV-TASK-011-06: Cổng repository (`ports.go`), `SnapshotLimits`, `RetentionTask`

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/ports.go` (mở rộng), `internal/usecase/retention_task.go` (mới), `internal/usecase/ports_test.go` (mới)
**Depends on:** BE-CV-TASK-011-04, BE-CV-TASK-010-06
**Status:** [x] DONE

---

## Context

Cổng đặt ở `usecase` (arch/03), adapter hai dialect hiện thực. Chữ ký ở SOL-011-repositories mục 2.A. `ports.go` đã có `OutboxStore`, `Clock` từ 010-06; task này **thêm** (không đổi tên) các cổng mới.

## Việc cần làm

1. Thêm cổng: `TenantSettingsRepository`, `RepoBindingRepository`, `SnapshotRepository`, `ReviewStateRepository`, `FindingDismissalRepository`, `C4OverrideRepository`, `ReindexJobRepository` (kể cả `CountActiveByDevServer`, `CountActiveByTenant`, `LastSucceededFinishedAt`), `ProcessedEventRepository`, `MaintenanceRepository`, `TenantMaintenanceRepository` (chữ ký nguyên văn SOL mục 2.A, 2.C).
2. `SnapshotLimits{MaxPayloadBytes int64; KeepCommits int; BindingQuotaBytes int64}` (+ constructor lấy từ `config.Config`).
3. `retention_task.go`: `RetentionTask` và `RetentionRegistry` (`Register(task)`, `All()`, chống trùng tên → panic lúc khởi động hoặc lỗi trả về, chọn lỗi trả về).
4. Mỗi phương thức có doc comment: lỗi trả về (`CODEINTEL_*`), có ghi outbox hay không, giới hạn danh sách.
5. Test: biên dịch kiểm `var _ Port = (*fake)(nil)` cho một fake trong `usecase` (dùng lại ở SOL-012/013); test `RetentionRegistry` (trùng tên, thứ tự đăng ký giữ nguyên).

## Kiểm thử

- `go build ./services/code-intel-service/... && go test ./services/code-intel-service/internal/usecase/...`

## Tiêu chí hoàn thành

- [x] Đủ 9 cổng + khung `RetentionTask`; không import adapter.
- [x] Mọi phương thức danh sách có tham số `limit`.
- [x] Test registry xanh.

## Rủi ro và lưu ý

- Đổi chữ ký sau khi SOL-012/013 dùng là phá biên dịch; chốt kỹ ở review.
- `percent *int` và `requested_by` rỗng = NULL (L3, L4).
