# TASK-REQ-005-03: `processed_events` repository và consumer `status_changed` (durable)

**From Solution:** BE-REQ-SOL-005
**Priority:** P0
**Service:** `request-service`
**File:** `internal/usecase/ports.go` (sửa: `ProcessedEventRepository`), `internal/adapter/postgres/processed_events.go`, `internal/adapter/mysql/processed_events.go`, `internal/adapter/eventbus/consumer.go`, `internal/adapter/eventbus/consumer_test.go` (mới); `cmd/server/main.go` (sửa)
**Depends on:** TASK-REQ-001-04, TASK-REQ-001-05
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -race ./internal/adapter/eventbus/... và go test -tags integration ./internal/adapter/eventbus (NATS thật) và go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql -run "Intake|Classification|Migration|Schema"`)

---

## Context

Bảng `processed_events(tenant_id, event_id, subject, processed_at)` có từ `0001` (TASK-REQ-001-03). `common/eventbus.Consumer.Subscribe(ctx, streamName, consumerName, subject, fn Handler)` chỉ ack khi `fn` trả nil; `Handler func(ctx, Event) error`; `Event{ID, TenantID, OccurredAt, Version, Payload}`. Mẫu consumer: `notification-service/internal/adapter/eventbus/consumer.go` (danh sách binding, goroutine mỗi subject, `WaitGroup`) và `task-service/internal/adapter/eventbus/consumer.go`. Stream `REQUEST` được tạo bởi TASK-REQ-001-05. Consumer này chỉ kích hoạt phân loại: logic nghiệp vụ nằm ở `ProposeRequestClassification` (TASK-REQ-005-04).

## Việc cần làm

1. `ports.go`: `ProcessedEventRepository{ MarkProcessed(ctx context.Context, eventID, subject string) (alreadyProcessed bool, err error); Prune(ctx context.Context, olderThan time.Time) (int64, error) }`. `MarkProcessed` dùng executor của ctx (tham gia giao dịch) và `tenant` trong ctx.
2. Postgres: `INSERT INTO request.processed_events (tenant_id, event_id, subject) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, `RowsAffected()==0` thì `alreadyProcessed=true`. MySQL: `INSERT IGNORE` tương ứng. `Prune`: `DELETE ... WHERE processed_at < $1`; khuyến nghị cửa sổ 7 ngày (arch/08).
3. `adapter/eventbus/consumer.go`: `type Consumer struct{ bus *eventbus.Consumer; classify ClassificationTrigger; logger }`; `Run(ctx)` đăng ký một binding `{Stream: "REQUEST", Durable: "request-classifier", Subject: "orca.request.request.status_changed"}`; handler: giải payload (`to`, `trigger`, `request_id`), bỏ qua nếu `to != "classifying"`; dựng ctx `tenant.WithTenantID(ev.TenantID)`; gọi `classify.Run(ctx, requestID, ev.ID, trigger)`; lỗi tạm thời (DB, mạng tới AI) trả lỗi để JetStream giao lại; lỗi vĩnh viễn (Request không tồn tại) log và trả nil.
4. Chỉ đăng ký khi có `*eventbus.Consumer` (NATS sẵn); `Run` trong goroutine có `WaitGroup` ở `main.go`; tên `durable` cố định.
5. Job dọn: goroutine hằng ngày gọi `Prune(now - 7 ngày)` (tách hàm `RunPruneLoop` để test).
6. `ClassificationTrigger` là interface một phương thức (`Run(ctx, requestID, eventID, trigger string) error`) để consumer test được mà không cần use case thật.

## Kiểm thử

- `TestConsumer_IgnoresOtherTargets` (sự kiện `to=analyzing` không gọi trigger).
- `TestConsumer_TriggersOnClassifying`, `TestConsumer_PermanentErrorAcked`, `TestConsumer_TransientErrorNotAcked`.
- Integration hai dialect: `TestMarkProcessed_FirstTrueSecondFalse`, `TestMarkProcessed_TenantScoped` (cùng `event_id` hai tenant đều là lần đầu), `TestMarkProcessed_JoinsTransaction` (rollback giao dịch thì `event_id` không còn), `TestPrune`.
- Integration với NATS thật (container): publish `status_changed`, consumer gọi trigger đúng một lần, giao lặp cùng `id` gọi hai lần nhưng dedup ở use case (TASK-REQ-005-04).
- Lệnh: `go test ./services/request-service/internal/adapter/eventbus/...` và `-tags=integration` cho repo.

## Tiêu chí hoàn thành

- [x] Consumer nhận `status_changed` `to=classifying` và gọi trigger với `eventID` của envelope.
- [x] `MarkProcessed` tham gia giao dịch của ctx và cách ly theo tenant.
- [x] Không có goroutine rò rỉ khi tắt service (`WaitGroup`).
- [x] Prune xoá bản ghi quá hạn.

## Rủi ro và lưu ý

- `MarkProcessed` phải gọi trong giao dịch ghi kết quả (TASK-REQ-005-04), không ở consumer; consumer chỉ chuyển `eventID`.
- Stream chưa tồn tại lúc consumer đăng ký: `awaitStream` của `eventbus` chờ; kiểm lỗi đăng ký chỉ log (như `notification-service`).

## Ghi chú triển khai

- Subject theo hằng `domain.SubjectRequestStatusChanged`; đã sửa giá trị hằng từ `...statuschanged` thành `...status_changed` (cùng `type_confirmed`, `type_changed`) cho khớp CR và `issue-status-sync`. Không còn chỗ nào dùng giá trị cũ.
- Consumer chỉ đưa việc vào `ClassificationRunner.Run` (tạo run bền) rồi ack; trigger là `ClassificationRunner`, không phải use case chạy AI nội tuyến (xem IMPLEMENTATION-NOTES, quyết định D3).
- `Prune` chạy xuyên tenant qua GUC `app.relay` (policy `relay_prune_scan`/`relay_prune` trong migration 0027).
