# BE-CV-TASK-071-02: Registry metrics `orca_codeintel_*` và `/metrics` của `code-intel-service`

**From Solution:** BE-CV-SOL-071
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/metrics/metrics.go` (mới), `.../internal/adapter/metrics/label_sets.go` (mới), `.../internal/adapter/metrics/metrics_test.go` (mới), `.../internal/adapter/grpc/metrics_interceptor.go` (mới), `.../cmd/server/main.go`
**Depends on:** BE-CV-SOL-010 (khung, cổng health 8080), BE-CV-SOL-020/021 (proto sinh ra `ServiceDesc`), BE-CV-SOL-022 (cache), BE-CV-SOL-013-agent-call-gate-and-quotas (hạn mức)
**Status:** `[x] DONE`

---

## Context

- Mẫu: `api-gateway/internal/adapter/mcpmetrics/metrics.go` (registry riêng, `pick(v, fallback, allowed…)`, quy tắc cardinality ở đầu file); `mcp-service`, `notification-service` mount `healthAndMetricsMux(health, metrics)` ở `cmd/server/main.go`. arch/09 T8: `/metrics` trên cổng health, `promhttp.HandlerFor` registry riêng.
- Tập đóng và nguồn của chúng: SOL-071 mục 5.2 (L1 `rpc` 49 giá trị từ `ServiceDesc`; L3 `result` từ bảng mã lỗi).
- Thông số cấm làm nhãn: tenant, user, worktree, project, đường dẫn, symbol, `jobId`, `commit`, `run_id`.

## Việc cần làm

1. `metrics.go`: struct `Metrics` với `prometheus.NewRegistry()` + `GoCollector`/`ProcessCollector`; khai báo các metric ở SOL-071 mục 5.2 (trừ nhóm CR-095, task 09); `Handler()`, `Registry()`.
2. `label_sets.go`: `rpcLabels` dựng lúc khởi tạo từ `CodeIntelService_ServiceDesc` và `QualityGateService_ServiceDesc` (Methods + Streams); `resultLabels` dựng từ bảng mã `CODEINTEL_*` (nguồn duy nhất trong `domain`, dùng chung với bộ dựng message lỗi); mọi giá trị ngoài tập → `other`.
3. `metrics_interceptor.go`: unary + stream interceptor gRPC đo `requests_total`, `request_duration_seconds{rpc,cache}` (nhãn `cache` do use case đặt vào ctx; mặc định `miss`), đặt **sau** interceptor cờ/quyền để đếm cả từ chối (kết quả `disabled`, `not_authorized`).
4. `main.go`: mount `/metrics` trên `HTTP_PORT` (mux health + metrics); không mount trên cổng gRPC.
5. Hàm quan sát cho collector/cache/reindex/outbox (`ObserveAgentCall`, `ObserveCacheEvent`, …) là cổng (`interface`) ở `usecase`; adapter metrics hiện thực (arch/03: domain không import Prometheus).

## Kiểm thử

- `TestMetricsLabelsAreClosedSets`: mã lỗi mới / `rpc` giả rơi vào `other`, số chuỗi không tăng.
- `TestRPCLabelSetMatchesServiceDesc`: số `rpc` bằng số RPC trong `ServiceDesc`; thêm RPC vào proto mà tập không đổi thì đỏ.
- `TestMetricsScrapeHasNoIdentifiers`: sau tải giả, quét `/metrics`: không khớp UUID, đường dẫn tuyệt đối, chuỗi 40 hex (commit).
- `go test ./internal/adapter/metrics/... ./internal/adapter/grpc/...`; không cần DB.

## Tiêu chí hoàn thành

- [x] `/metrics` trả đủ metric nhóm CR-071 §2.3 (đã sửa theo SOL-071).
- [x] Ba test trên xanh.
- [x] Không nhãn tenant/user/worktree.

## Rủi ro và lưu ý

- Tên `cache` do use case đặt: nếu CR-022 đổi vòng đời cache, sửa nhãn cùng PR.
- Sửa `cmd/server/main.go` của service mới: không có blast radius ngoài service; nếu mượn chung `common/health` thì chạy `gitnexus_impact` trước (nhiều service dùng).
