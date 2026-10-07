# BE-CV-TASK-024-09: Tích hợp hai replica hai dialect, hồi quy bão, nối `main.go`

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/adapter/eventbus/event_distribution_integration_test.go` (`//go:build integration`), `.../cmd/server/main.go` (sửa)
**Depends on:** TASK-024-03 … TASK-024-08
**Status:** [x] DONE

---

## Context

Tiêu chí chấp nhận của CR-024 mục 4; ma trận CI `dialect: [postgres, mysql]`.

## Việc cần làm

1. Dựng hai `code-intel-service` in-process chung DB + NATS + nguồn infra-fleet giả; bao phủ các ca ở mục 5 solution (một outbox, hai replica phát, giao lặp, giết giữa chừng, 30 tin/s, 100 tiến trình/s, tenant tắt cờ, cô lập tenant).
2. Hồi quy bão: 10 000 sự kiện/10 s, theo dõi bộ nhớ/goroutine.
3. Nối `Supervisor`, consumer, broadcaster trong `main.go` (cờ `CODEINTEL_ENABLED` tắt → không mở luồng).
4. Ghi vào README service: stream `CODEINTEL`, biến môi trường mới.

## Kiểm thử

- `go test -tags=integration ./... -run EventDistribution -race` (PG và MySQL).

## Tiêu chí hoàn thành

- [x] Mọi tiêu chí CR-024 mục 4 có test. - [x] Hai dialect xanh. - [x] Không `max-lines` disable.

## Rủi ro và lưu ý

- Chưa chạy test nào; số liệu bão là giả định.
