# BE-CV-TASK-080-06: Consumer `statusChanged`, cấu hình `CODEINTEL_AUTOREFRESH_*`, wiring `main.go`

**From Solution:** BE-CV-SOL-080
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/eventbus/agent_status_consumer.go`, `internal/config/auto_refresh.go`, `cmd/server/main.go` (sửa khi SOL-010 đã có)
**Depends on:** BE-CV-TASK-080-02, 080-05, BE-CV-SOL-010
**Status:** [x] DONE

## Context
`common/eventbus.Consumer.Subscribe(ctx, "INFRA", "code-intel-service-agent-status-changed", "orca.infra.agent.statusChanged", fn)` là consumer bền cạnh tranh (đã đọc `eventbus.go`). Idempotent bằng `processed_events (tenant_id, event_id)`.

## Việc cần làm
1. Giải mã payload (khoá lạ bỏ qua; thiếu `worktree_id` ⇒ bỏ, metric).
2. Trong một giao dịch: chèn `processed_events`; trùng ⇒ bỏ.
3. Đọc biến môi trường (bảng SOL-080 2.F), giá trị sai bị bỏ + log cảnh báo.
4. Wiring consumer + ticker; tắt êm theo `ctx`.

## Kiểm thử
- Unit giải mã (payload cũ/mới/hỏng); giao lặp cùng `event_id`; cấu hình sai.

## Tiêu chí hoàn thành
- [x] Consumer không panic trên payload lạ; không cần NATS để unit test.

## Rủi ro
Stream `INFRA` phải tồn tại (do infra-fleet tạo); consumer chờ stream (`awaitStream`).
