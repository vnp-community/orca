# TASK-REQ-005-08: Test tích hợp phân loại, xác nhận, đổi loại trên hai dialect

**From Solution:** BE-REQ-SOL-005
**Priority:** P0
**Service:** `request-service`
**File:** `internal/adapter/contracttest/classification_contract.go` (mới), `internal/adapter/postgres/classification_integration_test.go`, `internal/adapter/mysql/classification_integration_test.go` (mới)
**Depends on:** TASK-REQ-005-07, TASK-REQ-002-06
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql -run "Intake|Classification|Migration|Schema" và go test -tags integration ./internal/adapter/eventbus`)

---

## Context

Các tiêu chí mục 4 của CR-REQ-005 cần DB thật cho: Confirm đồng thời (CAS), consumer giao lặp (`processed_events` cùng giao dịch), lịch sử đúng thứ tự, giới hạn 5 lần AI kể cả thất bại. Bộ phân loại là bản giả điều khiển được (trả JSON hợp lệ, JSON sai, lỗi, độ trễ). NATS thật chỉ cần cho một kịch bản giao lặp (container NATS có JetStream); phần còn lại gọi use case trực tiếp.

## Việc cần làm

1. `RunClassificationContract(t, newEnv)` với `Env` thêm `Propose`, `Confirm`, `Change`, `History`, `Classifier` (giả).
2. Kịch bản: `AIProposalWritesAllFields`; `AIFailureStillAwaitingConfirmation`; `RedeliverySameEventOnce` (gọi `Propose` hai lần cùng `EventID`: một dòng lịch sử, `attempts=1`); `ConsumerRedeliveryWithNATS` (publish hai lần cùng envelope `id`; một đề xuất; chạy một lần với container NATS); `ConfirmConcurrent` (hai `Confirm` khác loại: một thắng); `ConfirmIdempotent`; `HistoryOrderAIUserChange`; `ChangeKeepsSolutions` (chèn một `solutions` hàng, đổi loại, hàng còn nguyên); `AttemptsLimitIncludesFailures` (5 lần thất bại, lần 6 `ClassifyRequest` bị chặn); `ApprovalNoopInTx`; `TenantIsolation` (tenant B không xác nhận được Request tenant A).
3. Hai file `_test.go` chỉ khởi môi trường và gọi hàm.
4. Chạy với `-race`.

## Kiểm thử

- `go test -tags=integration -race ./services/request-service/internal/adapter/postgres/... -run Classification -v`
- `go test -tags=integration -race ./services/request-service/internal/adapter/mysql/... -run Classification -v`
- Kịch bản NATS đánh dấu `t.Skip` nếu không có Docker, và ghi vào PR đã chạy hay chưa.

## Tiêu chí hoàn thành

- [x] Mọi kịch bản xanh với cả hai dialect.
- [x] Giao lặp cùng `EventID`: một đề xuất.
- [x] Giới hạn 5 lần AI tính cả lần thất bại.
- [x] Đổi loại giữ nguyên `solutions`.

## Rủi ro và lưu ý

- Chạy với agent dev server thật và đánh giá chất lượng phân loại chưa nằm trong phạm vi (chưa kiểm chứng).
- Kịch bản NATS có thể chậm; không bắt buộc trong CI nhanh (gắn nhãn `integration`).

## Ghi chú triển khai

- Kịch bản ở `contracttest/classification_contract.go` (`RunClassificationContract`), thêm `RunnerEndToEnd`, `RunLifecycle`, `RunRecoveryAndClaimLimit`, `ProcessedEvents`.
- Kịch bản NATS thật nằm ở `adapter/eventbus/classification_consumer_nats_integration_test.go` (container `nats:2.10-alpine`), đã chạy PASS.
