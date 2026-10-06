# BE-CV-TASK-089-07: `ListAgentTurns`, `GetAgentTurn`, bảo trì, fixture và cách ly tenant

**From Solution:** BE-CV-SOL-089-agent-turn-provenance
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/usecase/agent_turn_queries.go`, `agent_turn_maintenance.go`; handler trong `agent_turn_server.go`; `testdata/agent-turn/*.json` (mới)
**Depends on:** BE-CV-TASK-089-05, 089-06
**Status:** [ ] TODO

## Việc cần làm
1. `List`: `limit ≤ 50` (ngoài khoảng bị từ chối), `ended_at DESC, id DESC`, `before` (L5), gắn `gate?` bằng **một** truy vấn.
2. `Get`: id lạ/khác tenant/khác binding ⇒ `CODEINTEL_NOT_FOUND`.
3. Bảo trì: xoá > `CODEINTEL_AGENT_TURN_RETENTION_DAYS`(90) và > 200/binding; xoá văn bản > 30 ngày; mồ côi > 7 ngày; idempotent.
4. Golden `AgentTurn` (ui-api §4.7) dùng chung FE; test cô lập tenant hai dialect.

## Kiểm thử / Tiêu chí hoàn thành
- [ ] không N+1 (đếm truy vấn); [ ] bảo trì chạy lặp ổn định; [ ] tenant B không thấy lượt tenant A; [ ] `agentType/model` không vào nhãn metric.

## Rủi ro
- Kiểu `before` chưa chốt (L5).
