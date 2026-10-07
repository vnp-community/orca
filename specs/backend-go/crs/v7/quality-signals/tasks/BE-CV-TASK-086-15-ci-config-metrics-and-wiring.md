# BE-CV-TASK-086-15: Cấu hình, metric và wiring cho nhập CI

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/config/ci_refresh.go`, `cmd/server/main.go` (sửa), `deploy/dev/docker-compose.yml` (biến môi trường, khi SOL-010 đã thêm service)
**Depends on:** BE-CV-TASK-086-13, BE-CV-SOL-071-metrics-tracing-and-budgets
**Status:** [x] DONE

## Context
Biến: `SCM_INTEGRATION_SERVICE_ADDR`, `CODEINTEL_CI_REFRESH_PER_MINUTE`=30, `CODEINTEL_CI_RATE_BUDGET_PERCENT`=10, `CODEINTEL_CI_PROVIDER_HOSTS`. Giá trị là ước lượng CR.

## Việc cần làm
1. Đọc cấu hình; sai ⇒ bỏ + cảnh báo.
2. Metric: `orca_codeintel_ci_refresh_total{outcome=cache_hit|fetched|rate_limited|unsupported|error}`, không nhãn repo/đường dẫn.
3. Wiring client, repository, use case, handler; scm không cấu hình ⇒ `RefreshCiRun` trả `CODEINTEL_UNAVAILABLE`.

## Kiểm thử
- Unit cấu hình; test khởi động không có địa chỉ scm.

## Tiêu chí hoàn thành
- [x] Service khởi động được khi scm vắng.

## Rủi ro
Biến chưa có trong C-DM §6.2.
