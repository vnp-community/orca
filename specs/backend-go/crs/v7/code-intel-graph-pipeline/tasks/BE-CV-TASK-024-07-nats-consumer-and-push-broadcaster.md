# BE-CV-TASK-024-07: Consumer NATS tạm, broadcaster cục bộ, LRU `event_id`

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/adapter/eventbus/code_intel_consumer.go`, `.../adapter/broadcaster/code_intel_push_broadcaster.go` (mới), `.../cmd/server/main.go` (sửa) và test
**Depends on:** TASK-024-05
**Status:** [x] DONE

---

## Context

Mỗi replica nhận bản sao riêng (`SubscribeEphemeral`, `eventbus.go:196`); stream `CODEINTEL` tạo ở `main.go` như `INFRAFLEET`.

## Việc cần làm

1. `pub.EnsureStream(ctx, "CODEINTEL", []string{"orca.codeintel.>"})`; chạy `outbox.Relay`.
2. Consumer `SubscribeEphemeral`: lọc subject (chỉ `index.changed`, `reindex.finished`, `quality.gate_changed`), bỏ thiếu `TenantID`, LRU 1 024 `event_id`.
3. Broadcaster theo `(tenant,user)`, bộ đệm 64, đầy → thay bằng `changed{reason:overflow}`, không chặn `Publish`.
4. Đóng gọn khi tắt.

## Kiểm thử

- `go test ./internal/adapter/... -run 'CodeIntelConsumer|Broadcaster' -race`; integration NATS: hai replica → mỗi replica phát đúng một push; giao lặp bị LRU chặn; người đăng ký chậm → đúng một overflow.

## Tiêu chí hoàn thành

- [x] Các ca xanh. - [x] Không rò goroutine.

## Rủi ro và lưu ý

- NATS không sẵn: outbox vẫn bền; replica khác dựa vào thăm dò.
