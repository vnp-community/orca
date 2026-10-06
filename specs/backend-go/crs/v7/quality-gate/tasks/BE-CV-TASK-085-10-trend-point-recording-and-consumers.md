# BE-CV-TASK-085-10: Ghi điểm xu hướng: use case và ba consumer idempotent (+ `record=true`)

**From Solution:** BE-CV-SOL-085-waivers-and-trend
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/usecase/record_quality_trend_point.go`, `internal/adapter/{postgres,mysql}/quality_trend_repository.go`, `internal/adapter/eventbus/quality_trend_consumers.go` (mới)
**Depends on:** BE-CV-TASK-085-06, 085-09; BE-CV-SOL-082 (`run_finished`), BE-CV-SOL-010 (`processed_events`, stream `CODEINTEL`)
**Status:** [ ] TODO

## Context
`eventbus.Consumer.Subscribe` durable = competing consumer (đã đọc `eventbus.go:128`): đúng cho ghi DB một lần. Consumer phải lọc subject (stream `CODEINTEL` chứa cả `review.saved`). Khoá T12 có `source`.

## Việc cần làm
1. `RecordQualityTrendPoint`: tính cổng, upsert điểm (PG `ON CONFLICT`; MySQL `ON DUPLICATE KEY UPDATE`), so điểm gần nhất cùng `(binding, profile_ref, source)`.
2. Consumer durable: `orca.codeintel.quality.run_finished`; `orca.codeintel.index.changed` (chỉ khi `stale` đổi); `orca.codeintel.agent_turn.recorded` (`turn_key = agent_turns.client_turn_id`). Mỗi handler: một transaction gồm `INSERT processed_events (tenant_id, event_id)` + upsert + outbox; `event_id` đã có ⇒ bỏ qua.
3. `GetQualityGate(record=true)`: ghi **nếu chưa có** điểm cho `(head, turn_key, profile_ref, source)`.

## Kiểm thử
- Giao lặp cùng `event_id` ⇒ một điểm; thứ tự sự kiện ngược; `local`/`ci` hai điểm; stream chưa tồn tại (`awaitStream`); integration hai dialect.

## Tiêu chí hoàn thành
- [ ] idempotent; [ ] `turn_key` đúng; [ ] cờ tắt vẫn tiêu thụ để không tắc outbox.

## Rủi ro
- `agent_turn.recorded` đến trước khi `quality_runs` có ⇒ điểm `unknown`; chấp nhận, điểm thay thế khi `run_finished` đến.
