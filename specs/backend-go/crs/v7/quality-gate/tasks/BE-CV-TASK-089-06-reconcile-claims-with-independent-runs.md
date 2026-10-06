# BE-CV-TASK-089-06: Đối chiếu lời tự báo với run độc lập (domain + consumer)

**From Solution:** BE-CV-SOL-089-agent-turn-provenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/domain/agent_claim_reconciler.go`, `internal/usecase/reconcile_agent_turn.go`, `internal/adapter/eventbus/agent_turn_reconcile_consumer.go` (mới)
**Depends on:** BE-CV-TASK-089-05, BE-CV-TASK-085-05 (profile hiệu lực), BE-CV-SOL-082
**Status:** [ ] TODO

## Việc cần làm
1. Reconciler thuần theo bảng SOL-089 §2.4 (`consistent|contradicted|unverified|not_claimed`; `build_ok`/`all_done` ⇒ `unverified/no_equivalent_check`; cây bẩn ⇒ `unverified/tree_may_differ`).
2. Chọn run cùng category qua `checks[].category ↔ profile`; run `env_not_ready`/`error_code≠''` ⇒ `unverified`.
3. Consumer durable `orca.codeintel.quality.run_finished` (lọc subject; `processed_events`); cũng gọi lại khi `RecordAgentTurn` đến sau run.
4. Ghi `verification` (≤ 8 KiB); **không** đổi `verdict` của cổng.

## Kiểm thử
- Bảng 4 trạng thái × cây sạch/bẩn × run `succeeded|failed|env`; thứ tự sự kiện ngược; giao lặp.

## Tiêu chí hoàn thành
- [ ] `contradicted` chỉ khi hai phía sạch; [ ] `GetQualityGate` không đổi khi có verification.

## Rủi ro
- Không có dấu vân tay cây ở lượt ⇒ lượt kết thúc khi cây bẩn không bao giờ `contradicted`.
