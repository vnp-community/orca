# BE-CV-TASK-089-05: Use case `RecordAgentTurn`, handler, outbox `agent_turn.recorded`

**From Solution:** BE-CV-SOL-089-agent-turn-provenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/usecase/record_agent_turn.go`, `internal/adapter/grpc/agent_turn_server.go` (mới)
**Depends on:** BE-CV-TASK-089-04, BE-CV-TASK-085-07 (guard cờ/OPA), BE-CV-SOL-013 (`TextRedactor`, hạn mức)
**Status:** [x] DONE

## Việc cần làm
1. Chuỗi: cờ → OPA `review_write` → `selector → binding` → kiểm đầu vào (bảng SOL-089 §2.2) → hợp nhất → outbox → trả.
2. `promptExcerpt` chỉ khi cờ tenant; ≤ 160 + che; `model_source='unknown'`; `expires_at=ended_at+retention`.
3. Outbox `orca.codeintel.agent_turn.recorded` `{repo_binding_id, turn_id, end_head_commit}` cùng transaction; `event_id` UUID v5 `(tenant, turn_id, version)`.
4. Hạn mức ghi qua cổng của SOL-013.

## Kiểm thử
- Cờ tắt từng tầng; thiếu `review_write`; `endedAt` tương lai > 5 phút bị từ chối; rollback ⇒ không event; `TextRedactor` giả; tenant khác ⇒ `NOT_AUTHORIZED`.

## Tiêu chí hoàn thành
- [x] hai lần cùng `clientTurnId` ⇒ một hàng; [ ] `stated` không sinh khi cờ tắt; [ ] event chỉ chứa id.

## Rủi ro
- Chưa có bộ che bí mật dùng chung (O-16).
