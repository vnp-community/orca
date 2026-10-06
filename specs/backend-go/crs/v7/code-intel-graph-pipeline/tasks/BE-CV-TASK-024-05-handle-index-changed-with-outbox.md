# BE-CV-TASK-024-05: Xử lý `index_changed`: `processed_events` + `InvalidateBinding` + outbox trong một giao dịch

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P0
**Service:** `code-intel-service`
**File:** `.../internal/usecase/handle_agent_code_intel_event.go` (mới), `.../adapter/{postgres,mysql}` (cài `MarkProcessed` nếu SOL-011 chưa có) và test
**Depends on:** TASK-024-03, TASK-024-04, BE-CV-SOL-022
**Status:** [ ] TODO

---

## Context

SOL-024 2.C; payload `orca.codeintel.index.changed` theo hợp đồng §5.

## Việc cần làm

1. Tra binding `(tenant,dev_server,path_hash)`; không có → bỏ + `events_unmatched_total`.
2. Cục bộ: `InvalidateProbe`.
3. `TxRunner.InTx`: `MarkProcessed` (đã xử lý → thoát) → `InvalidateBinding` → `InsertOutboxEvent` (payload §5; ánh xạ `reason`; `tool:git` chỉ cập nhật `stale`).
4. Cờ tenant tắt → bỏ nhưng vẫn huỷ cache (PQ-24).

## Kiểm thử

- Unit với store giả: giao lặp → một outbox; crash giữa chừng không dòng mồ côi.
- `go test -tags=integration ./... -run IndexChanged -race` PG + MySQL; test tenant: sự kiện tenant A không đụng binding tenant B.

## Tiêu chí hoàn thành

- [ ] Một dòng outbox/`event_id`. - [ ] Hai dialect xanh. - [ ] Cô lập tenant.

## Rủi ro và lưu ý

- MySQL: không `RETURNING`; dùng `INSERT IGNORE`/`ON DUPLICATE KEY` và đếm hàng đúng driver.
