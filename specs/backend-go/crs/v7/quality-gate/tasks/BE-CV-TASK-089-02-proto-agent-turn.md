# BE-CV-TASK-089-02: Proto `codeintel_agent_turn.proto` và ba dòng `rpc`

**From Solution:** BE-CV-SOL-089-agent-turn-provenance
**Priority:** P1
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_agent_turn.proto` (mới); `codeintel_quality_gate.proto` (thêm `RecordAgentTurn`, `ListAgentTurns`, `GetAgentTurn`)
**Depends on:** BE-CV-TASK-085-02
**Status:** [x] DONE

## Việc cần làm
1. Message `AgentTurn`, `CommandSummary`, `AgentClaim`, `ClaimVerification` theo ui-api §4.7; `RecordAgentTurnRequest` theo §3.2 (`selector` field 1; **không** có `prompt`, `base_head_commit`).
2. `ListAgentTurnsRequest{selector, limit, before}`; `GetAgentTurnRequest{selector, turn_id}`.
3. `buf lint`, `buf breaking` trực tiếp; sinh stub.

## Kiểm thử / Tiêu chí hoàn thành
- [x] lint+breaking xanh; [ ] `before` là `string` (L5 chờ duyệt).

## Rủi ro
- Xung đột merge dòng `rpc` với solution khác.
