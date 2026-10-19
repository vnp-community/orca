# TASK-E2E-031: Giới hạn ghi cấu hình được, có `retryAfterMs` và metric

**From Solution:** SOL-E2E-006 · **Resolves:** OBS-C
**Priority:** P2 · **Service/Area:** request-service, api-gateway
**Files:**
- backend-go/services/request-service/internal/domain/rate_limit_policy.go
- backend-go/services/request-service/internal/config/config.go
- backend-go/services/request-service/internal/adapter/grpc (thông báo lỗi)
- metric `orca_request_rate_limited_total{class}`
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

5 ghi/giây, burst 10 là đề xuất "chưa đo". Một lượt e2e bị chặn 10 lần dù đã giãn nhịp.

## Việc cần làm

- [x] Đọc `REQUEST_RATE_<CLASS>_PER_SECOND` và `…_BURST` từ env, mặc định giữ nguyên.
- [x] Thông báo `REQUEST_RATE_LIMITED` kèm `retryAfterMs`.
- [~] Metric đếm theo lớp; hướng dẫn đo và cập nhật CR-REQ-035. — _có metric và ghi trong CONTRACT; chưa cập nhật CR-REQ-035_

## Tiêu chí hoàn thành

- [x] Đặt giới hạn cao bằng env thì `RQL-20` đổi kỳ vọng (đã có tham số trong test).
- [x] Metric có số liệu.

## Kiểm thử

Unit test cấu hình; `RQL-20`.

## Kết quả thực thi

Xong. `REQUEST_RATE_<READ|WRITE|AI|WEBHOOK>_PER_SECOND/_BURST` (giá trị sai giữ mặc định + cảnh báo), `retryAfterMs` ở thông báo gateway, metric `orca_request_rate_limited_total{class}`; test ở domain, interceptor, metrics, gateway. e2e client dùng `retryAfterMs`.
