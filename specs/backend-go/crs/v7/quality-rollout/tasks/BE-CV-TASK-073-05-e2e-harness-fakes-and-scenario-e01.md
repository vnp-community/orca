# BE-CV-TASK-073-05: Khung e2e T1, agent giả phát lại tệp vàng, kịch bản E01

**From Solution:** BE-CV-SOL-073
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/e2e/harness_test.go` (mới, tag `e2e`), `.../e2e/fakes/replay_agent.go` (mới), `.../e2e/fakes/in_memory_infra_fleet.go` (mới), `.../e2e/index_status_test.go` (mới)
**Depends on:** BE-CV-TASK-070-01, 070-02, BE-CV-TASK-073-03, BE-CV-SOL-012-index-status-aggregation, BE-CV-SOL-021, BE-CV-SOL-023, BE-CV-SOL-011-*
**Status:** `[ ] TODO`

---

## Context

- T1: tiến trình đơn, testcontainers (Postgres và MySQL), outbox thật, DB thật; agent **giả** phát lại `testdata/agent-results/` (SOL-070); infra-fleet giả trong bộ nhớ.
- E01: `GetIndexStatus` công cụ có sẵn, `indexedAt`, `stale` đúng khi HEAD vượt `indexedCommit`; `IndexStatus` theo PQ-08 (`overall`, `tools[]`).
- Thư mục fake đặt tên theo nội dung (không `helpers`).

## Việc cần làm

1. Harness: khởi DB theo biến `E2E_DIALECT`, chạy migration, dựng service trong tiến trình với cổng thật trừ `InfraFleet` (fake), bật cờ cho tenant thử qua `SetSettings`.
2. `replay_agent`: phục vụ `Exec(method, params)` bằng tệp vàng theo bảng ánh xạ method → tệp; chế độ lỗi (`failNext(code)`), độ trễ, ngắt kết nối; ghi lại mọi lời gọi (để test khẳng định đường dẫn/số lần gọi).
3. `in_memory_infra_fleet`: `RelayByDevServer`, `IsDevServerConnected`, `GetAgentCapabilities`, kênh sự kiện `StreamCodeIntelEvents`.
4. E01 theo mô tả; kiểm `stale` khi `headCommit` ≠ `indexedCommit`.

## Kiểm thử

- `go test -tags=e2e ./e2e/... -run IndexStatus` với `E2E_DIALECT=postgres` và `mysql` (cần Docker).
- Chưa chạy.

## Tiêu chí hoàn thành

- [ ] E01 xanh hai dialect.
- [ ] Fake dùng tệp vàng, không dữ liệu thứ hai.

## Rủi ro và lưu ý

- T1 dùng infra-fleet giả nên có thể che lỗi đường nối thật; T3 bù (không chặn).
