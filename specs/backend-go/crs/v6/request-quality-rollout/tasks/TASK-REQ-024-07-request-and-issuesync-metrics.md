# TASK-REQ-024-07: Metric Prometheus và `/metrics` cho `request-service` và `issue-status-sync`

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `request-service`, `issue-status-sync`
**File:** `backend-go/services/request-service/internal/adapter/metrics/request_metrics.go` (mới), `.../cmd/server/main.go`; `backend-go/services/issue-status-sync/internal/adapter/metrics/issuesync_metrics.go` (mới), `.../cmd/server/main.go`, `.../internal/usecase/ports.go`; các `_test.go`
**Depends on:** TASK-REQ-024-04; CR-REQ-003, 007, 009, 013 (nơi gọi `Observe*`)
**Status:** `[ ] TODO`

---

## Context

- Mẫu: `notification-service/internal/adapter/metrics/push_metrics.go` (`Set` giữ `prometheus.Registry` riêng, `MustRegister` kèm Go và Process collector, `Handler()`), `notification-service/cmd/server/main.go:369` `healthAndMetricsMux(health, metrics)` mount `/healthz`, `/readyz`, `/metrics`.
- `issue-status-sync/cmd/server/main.go:180-182`: `httpServer := &http.Server{Handler: healthSrv.Handler()}`: không `/metrics`.
- Bảng metric: CR-REQ-024 mục 2.9 (15 series). Cấm nhãn `tenant_id`, `request_id`, tiêu đề.
- Gauge `approvals_pending`, `stuck` lấy mẫu từ DB; `outbox_pending` đếm `outbox_events` chưa gửi (tên bảng theo README v6 mục 8 dòng 3).

## Việc cần làm

1. `request_metrics.go`: `type Set` với 12 series `orca_request_*` (counter, histogram, gauge) và `Observe...` cho từng cái; `Handler()`.
2. Cổng quan sát nhỏ ở `usecase` của `request-service` (ví dụ `type RequestObserver interface{ Created(provider string); Transition(typ, from, to string); ... }`), bản `noopObserver`; adapter `metrics.Set` hiện thực. Use case gọi `Observe` sau giao dịch thành công.
3. Goroutine lấy mẫu mỗi 30 giây (`approvals_pending{subject_type}`, `stuck{status}` theo ngưỡng cấu hình `REQUEST_STUCK_THRESHOLD_<STATUS>`, `outbox_pending`); dừng khi `ctx` xong.
4. `issuesync_metrics.go`: `orca_issuesync_request_events_total{event,result}` với `result` ∈ `applied, skipped_not_jira, skipped_flag_off, skipped_stale, skipped_no_actor, skipped_category, skipped_transition_unavailable, failed`; `orca_issuesync_skipped_request_owned_total{source}`; `orca_issuesync_jira_transition_seconds`. Cổng `SyncObserver` ở `usecase`, `noop` mặc định.
5. `main.go` ở cả hai service: `healthAndMetricsMux`; thêm `/metrics`.
6. Giữ `orca_mcp_tool_calls_total` làm số đo tool MCP; không thêm metric riêng cho `request_*`.

## Kiểm thử

- `request_metrics_test.go`, `issuesync_metrics_test.go`: gọi `Observe` rồi `testutil.GatherAndCount`/đọc `/metrics`; khẳng định tên series, bộ nhãn, và không nhãn nào có chuỗi id (ví dụ test đưa UUID vào các tham số cho phép, khẳng định không xuất hiện trong output).
- `usecase`: bảng `result` của `HandleRequestStatus` khớp bộ giá trị trên.
- `go test ./internal/adapter/metrics/... ./internal/usecase/...` ở hai service.

## Tiêu chí hoàn thành

- [ ] `/metrics` của hai service trả các series 2.9.
- [ ] Không nhãn có id hay tiêu đề.
- [ ] Goroutine lấy mẫu dừng sạch khi tắt.

## Rủi ro và lưu ý

- Gauge lấy mẫu tăng tải DB nhẹ; dùng truy vấn đếm có chỉ mục theo `status`.
- Ngưỡng `stuck` là giả định, chưa đo.
