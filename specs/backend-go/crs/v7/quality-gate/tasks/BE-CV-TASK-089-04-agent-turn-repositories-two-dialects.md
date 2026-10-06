# BE-CV-TASK-089-04: Repository `agent_turns` hai dialect (upsert hợp nhất, chuỗi base, truy vấn, xoá)

**From Solution:** BE-CV-SOL-089-agent-turn-provenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/adapter/{postgres,mysql}/agent_turn_repository.go` (mới)
**Depends on:** BE-CV-TASK-089-01, 089-03
**Status:** [ ] TODO

## Việc cần làm
1. `UpsertMerged`: transaction `SELECT … FOR UPDATE` theo khoá duy nhất → `MergeAgentTurn` → `UPDATE … WHERE version=?` hoặc `INSERT`; 23505/1062 ⇒ thử lại một lần.
2. `PreviousEndHead(tenant, binding, endedAt)` (`ended_at < ?` DESC LIMIT 1) cho `base_head_commit`.
3. `List(before, limit)`, `Get(id)`, `GateByTurnKeys` (một truy vấn `quality_trend_points … turn_key IN`).
4. `DeleteExpired`, `DeleteBeyondCap(200)`, `ClearTextOlderThan`, `DeleteOrphans` (lô 500).
5. Mọi truy vấn có `tenant_id`; test AST.

## Kiểm thử
- Integration hai dialect: hai goroutine cùng `clientTurnId`; `base` chain; lô xoá; UTF-8; cách ly tenant.

## Tiêu chí hoàn thành
- [ ] một hàng/`clientTurnId`; [ ] `source=both` khi hợp; [ ] guard AST xanh.

## Rủi ro
- Lượt đến sai thứ tự không sửa `base` của lượt sau (hạn chế đã biết).
