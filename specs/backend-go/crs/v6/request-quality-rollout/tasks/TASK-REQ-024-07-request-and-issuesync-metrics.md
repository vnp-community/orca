# TASK-REQ-024-07: Metric Prometheus và `/metrics` cho `request-service` và `issue-status-sync`

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `request-service`, `issue-status-sync`
**File:** `backend-go/services/request-service/internal/adapter/metrics/request_metrics.go` (mới), `.../cmd/server/main.go`; `backend-go/services/issue-status-sync/internal/adapter/metrics/issuesync_metrics.go` (mới), `.../cmd/server/main.go`, `.../internal/usecase/ports.go`; các `_test.go`
**Depends on:** TASK-REQ-024-04; CR-REQ-003, 007, 009, 013 (nơi gọi `Observe*`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./... -count=1` trong `services/request-service` (512 test PASS, 0 FAIL) và `services/issue-status-sync` (186 PASS); `go test -tags integration ./internal/adapter/postgres/... ./internal/adapter/grpc/... ./cmd/...` trên `postgres:16-alpine` thật (262 test PASS); `go test -tags integration ./internal/adapter/mysql/...` trên `mysql:8.0` thật (PASS, 238 giây); e2e `TestMetricsEndpointServesRequestSeries`)

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

- [x] `/metrics` của hai service trả các series 2.9.
  `issue-status-sync` đã phục vụ `/metrics` (test đọc output); `request-service` chưa.
- [x] Không nhãn có id hay tiêu đề.
  Phía `issue-status-sync`: nhãn chỉ lấy từ hằng cố định, test khẳng định không có id/`tenant_id`/`request_id` trong output. Phía `request-service` chưa làm.
- [x] Goroutine lấy mẫu dừng sạch khi tắt.
  Thuộc `request-service`, chưa làm.

## Rủi ro và lưu ý

- Gauge lấy mẫu tăng tải DB nhẹ; dùng truy vấn đếm có chỉ mục theo `status`.
- Ngưỡng `stuck` là giả định, chưa đo.

## Tiến độ (2026-10-07)

Đã làm ở `issue-status-sync`: `adapter/metrics/issuesync_metrics.go` (`orca_issuesync_request_events_total{event,result}`, `orca_issuesync_skipped_request_owned_total{source}`, `orca_issuesync_jira_transition_seconds`, registry riêng + Go/Process collector), cổng `SyncObserver` + `noopObserver` mặc định, `healthAndMetricsMux` trong `main.go`, test `issuesync_metrics_test.go` và test `result` của `HandleRequestStatus` trong `sync_request_status_test.go`. Giá trị `event` thực tế: `status_changed`, `completed`, `comment`, `worktree`, `pr` (hai giá trị cuối cho lỗi tra cứu, `comment` cho bình luận lỗi).

Còn thiếu: toàn bộ phần `request-service` (12 series `orca_request_*`, `RequestObserver`, goroutine lấy mẫu 30 giây, `/metrics`), đợt sau khi request-service rảnh.

## Kết quả triển khai (2026-10-08)

12 series `orca_request_*` ở `adapter/metrics/request_metrics.go`: đếm từ sự kiện outbox đã commit (không nhãn id), gauge `approvals_pending`/`stuck`/`outbox_pending` lấy mẫu qua `MetricsSampleSource` (migration `0092`, policy `relay_scan` chỉ đọc), goroutine dừng khi tắt (`lifecycleWG`), `/metrics` ghép trong `wire_rollout.go`. `ObserveAIGeneration` chỉ nối cho phân loại và `ObserveTaskOutcome` chưa ai gọi: chờ Solution/Plan (rf-sol, rf-exec) và `ReportTaskOutcome`. Lệch task: bộ quan sát nhỏ hơn đề xuất vì phần lớn series suy từ sự kiện. Ngưỡng stuck là giả định.
