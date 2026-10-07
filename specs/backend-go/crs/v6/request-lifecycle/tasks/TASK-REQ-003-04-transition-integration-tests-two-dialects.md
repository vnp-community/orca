# TASK-REQ-003-04: Test tích hợp `TransitionRequest` trên Postgres và MySQL

**From Solution:** BE-REQ-SOL-003
**Priority:** P0
**Service:** `request-service`
**File:** `internal/adapter/contracttest/transition_request_contract.go` (mới), `internal/adapter/postgres/transition_request_integration_test.go`, `internal/adapter/mysql/transition_request_integration_test.go` (mới)
**Depends on:** TASK-REQ-003-03, TASK-REQ-002-06
**Status:** [x] DONE

---

## Context

Bộ `contracttest` (TASK-REQ-002-06) cho kịch bản dùng chung hai dialect. Hành vi cần chứng minh trên DB thật: CAS đồng thời, giao dịch gộp `status` và `outbox`, CHECK `requests_backlog_stage` không bị vi phạm. Quy ước build tag `integration`, testcontainers; Postgres dùng role `NOSUPERUSER NOBYPASSRLS`.

## Việc cần làm

1. `contracttest.RunTransitionContract(t, newEnv)` với `Env` mở rộng (thêm `Transition *usecase.TransitionRequest`, `Outbox` đọc dòng `outbox_events` để kiểm: hàm đọc theo tenant, không phải `FetchUnpublished`).
2. Kịch bản: `TenTransitionsConcurrent` (10 goroutine cùng Request ở `awaiting_type_confirmation` với `type_confirmed` và `cancel`: chỉ một thắng mỗi trạng thái, số dòng outbox bằng số chuyển áp dụng); `IdempotentRedeliveryNoExtraOutbox`; `OutboxFailureRollsBackStatus` (outbox writer bọc, ép lỗi ở lần ghi: `status` không đổi, kiểm từ DB); `BacklogCheckHeld` (`return_to_backlog` rồi `reopen`: không vi phạm CHECK, `returned_from_stage` NULL sau mở lại); `NestedJoinsOuterTransaction` (ngoài `InTx` gọi `TransitionRequest` rồi trả lỗi: `status` không đổi); `FullHappyPathPerType` (11 loại, chạy trigger chuẩn tới `completed` trên DB thật, kiểm trạng thái cuối và số sự kiện `status_changed`).
3. Mỗi file `_test.go` chỉ khởi môi trường, chạy hàm contract.
4. Postgres: một test `TransitionUnderRLS` xác nhận `Update` và outbox hoạt động với role không superuser (bắt trường hợp thiếu `set_config`).

## Kiểm thử

- `go test -tags=integration ./services/request-service/internal/adapter/postgres/... -run Transition -v`
- `go test -tags=integration ./services/request-service/internal/adapter/mysql/... -run Transition -v`
- Chạy lặp `-count=5` cho `TenTransitionsConcurrent` để bắt chập chờn.

## Tiêu chí hoàn thành

- [x] 10 lệnh đồng thời: đúng một thắng cho mỗi đích, số outbox đúng.
- [x] Giao lặp không thêm dòng outbox.
- [x] Lỗi outbox giữ nguyên `status`.
- [x] 11 loại đi hết `HappyPath` trên cả hai DB.
- [x] Không vi phạm CHECK `requests_backlog_stage` ở mọi chuyển.

## Rủi ro và lưu ý

- Deadlock MySQL (mã 1213) khi nhiều giao dịch cập nhật cùng hàng: kỳ vọng thua CAS hoặc lỗi 1213; test chấp nhận cả hai là "thua", nhưng không chấp nhận hai bên cùng thắng.
- Test chạy nặng; không đưa vào `go test ./...` thường (build tag).
