# BE-CV-TASK-011-13: Công việc bảo trì định kỳ, đăng ký `RetentionTask`, nối vào `main.go`

**From Solution:** BE-CV-SOL-011-repositories-and-maintenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/snapshot_maintenance.go`, `internal/usecase/snapshot_maintenance_test.go` (mới); `backend-go/services/code-intel-service/cmd/server/main.go` (sửa)
**Depends on:** BE-CV-TASK-011-06, 011-12, 010-07
**Status:** [x] DONE

---

## Context

Hợp đồng §4.3: mỗi `CODEINTEL_MAINTENANCE_INTERVAL` (10 phút), lô 500. Giới hạn/retention từ `config.Config` (SOL-010). CR chủ sở hữu bảng khác (`quality_*`, `agent_turns`, `coverage_reports`) đăng ký `RetentionTask` riêng; task này chỉ dựng khung và việc của `0002`.

## Việc cần làm

1. `SnapshotMaintenance{Maint MaintenanceRepository, PerTenant func(ctx, tenantID string) (TenantMaintenanceRepository, ctx), Registry *RetentionRegistry, Clock, Cfg, Events OutboxPublisher, SchemaVersions func(view string) int}`; phương thức `RunOnce(ctx) (Report, error)` và `Run(ctx)` (ticker, dừng theo `ctx.Done()`).
2. `RunOnce` theo thứ tự SOL mục 2.C: (a) `DeleteExpiredSnapshots` lặp lô đến hết; (b) `ListTenantsWithData`; (c) mỗi tenant: sai `schema_version` → vượt hạn mức → mồ côi (`now - OrphanRetention`) → job mồ côi (`now - ReindexStaleAfter`; mỗi job trả về sinh `orca.codeintel.reindex.finished` payload `{job_id, repo_binding_id, project_id, status:'failed', outcome:'', error_code:'CODEINTEL_REINDEX_ORPHANED', finished_at}`, ghi qua outbox trong transaction của `FailOrphanedReindexJobs` — chốt: `FailOrphanedReindexJobs` nhận `events` builder để cùng tx) → binding rảnh (`now - BindingIdleRetention`); (d) outbox đã publish và `processed_events` quá 7 ngày; (e) `Registry.All()` từng tác vụ.
3. Mỗi bước chạy trong lỗi-cô-lập: lỗi một tenant/tác vụ ghi log (không nội dung) và **tiếp tục**; `Report` đếm dòng xoá theo bước; trả lỗi gộp ở cuối nhưng không dừng vòng sau.
4. `main.go`: khởi goroutine `Run(ctx)` (tắt theo ctx, chờ `WaitGroup`); đăng ký ghi `Report` ở DEBUG.
5. Chưa có metric (SOL-071); để chỗ gọi `observer` interface rỗng.

## Kiểm thử

- Unit với repository giả: thứ tự bước; lỗi một tenant không chặn tenant khác; lô lặp đến khi trả < batch; `ctx` huỷ giữa vòng dừng sạch; `RetentionTask` lỗi không chặn tác vụ khác; sự kiện `reindex.finished` đúng payload.
- Integration (hai dialect) dùng 011-12: một vòng đầy đủ với dữ liệu hạn rút ngắn; hai bản sao chạy đồng thời cho kết quả giống một bản (idempotent).
- Lệnh: `go test ./services/code-intel-service/internal/usecase/... -run Maintenance`; `go test -tags=integration ./services/code-intel-service/... -run MaintenanceJob`.

## Tiêu chí hoàn thành

- [x] Job `running` quá hạn → `failed` + `CODEINTEL_REINDEX_ORPHANED`, `active_key = NULL`, có sự kiện.
- [x] Snapshot hết hạn/hạn mức/sai schema bị dọn; binding mồ côi dọn dữ liệu phụ thuộc sau hạn.
- [x] Lỗi cục bộ không dừng vòng; shutdown sạch.

## Rủi ro và lưu ý

- Không có khoá tư vấn: nhiều bản sao làm trùng việc (vô hại).
- Đồng hồ: mọi so sánh hạn dùng đồng hồ DB; `Clock` của ứng dụng chỉ dùng để tính `olderThan` truyền vào; sai lệch giữa hai đồng hồ chấp nhận được ở thang phút.
