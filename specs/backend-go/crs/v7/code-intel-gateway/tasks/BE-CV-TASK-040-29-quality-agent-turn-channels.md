# BE-CV-TASK-040-29: Kênh `codeIntel.quality.turn.record`, `.turns`, `.turn`

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_turns.go` (mới), `channels_codeintel_quality_turns_test.go` (mới)
**Depends on:** TASK-040-23; stub `RecordAgentTurn`, `ListAgentTurns`, `GetAgentTurn` (BE-CV-SOL-089-agent-turn-provenance)
**Status:** [ ] TODO

---

## Context

UI-API 3.2: `turn.record` (sel, `clientTurnId`, `agentType`, `model?`, `endedAt`, `startedAt?`, `interrupted?`, `endHeadCommit`, `treeDirtyEnd`, `filesChangedCount`, `filesDigest`, `promptDigest`, `promptExcerpt?`, `commandsSummary?`, `claims?`) => `{turn: AgentTurn}` (8 s, `review_write`); `turns` (sel, `limit ≤ 50` (20), `before?`) => `{turns}`; `turn` (sel, `turnId`) => `{turn}`. PQ-35: nguồn A (renderer) là mặc định; `agent.hook` không dùng; không lưu prompt người dùng trừ `promptExcerpt` khi tenant bật `agentTurnStorePromptExcerpt`; `claimText` chỉ khi `agentClaimTextEnabled` (service quyết).

## Việc cần làm

1. `qualityTurnRecordArgs{sel, ClientTurnID, AgentType, Model, EndedAt, StartedAt, Interrupted *bool, EndHeadCommit, TreeDirtyEnd *bool, FilesChangedCount *int, FilesDigest, PromptDigest, PromptExcerpt, CommandsSummary, Claims json.RawMessage}`: bắt buộc `clientTurnId` (1..128, dạng `${paneKey}:${doneAt}`), `agentType` (≤ 64), `endedAt` RFC 3339, `endHeadCommit`, `treeDirtyEnd` (khoá có mặt), `filesChangedCount >= 0`, `filesDigest`/`promptDigest` (≤ 128); `commandsSummary`/`claims` là object JSON hợp lệ, mỗi cái ≤ 8 KiB (tổng đã ≤ 16 KiB); `promptExcerpt` ≤ 2 KiB (quyết định solution; Q4) và **không bao giờ** log/đưa vào lỗi.
2. `qualityTurnsArgs{sel, Limit, Before}`: `limit` 1..50; `before` RFC 3339 hoặc id opaque (≤ 128).
3. `qualityTurnArgs{sel, TurnID}`.
4. Dịch `AgentTurn`: `commandsSummary.commands[]`, `claims.items[]` mảng; `agreement` vắng khi `not_claimed`... theo proto; `gate?` vắng khi chưa đánh giá.
5. Đề xuất (Q5 foundation): thêm `codeIntel.quality.turn.record`/`promptExcerpt` vào `sensitiveArgChannels` nếu được duyệt; nếu chưa, test khẳng định lỗi không echo args.

## Kiểm thử

- Validate: thiếu `treeDirtyEnd`; `filesChangedCount` -1; `endedAt` sai; `claims` là mảng; `promptExcerpt` 2 KiB+1; `limit` 51.
- Fake: tenant tắt `promptExcerpt` => service bỏ (gateway không tự); lỗi `INVALID_PARAMS` không chứa `promptExcerpt`.
- Quét log (hook logger của test): không có `promptExcerpt` trong bất kỳ bản ghi.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelQualityTurns'`.

## Tiêu chí hoàn thành

- [x] Ba kênh đúng shape UI-API 4.7; prompt không rò log/lỗi.

## Rủi ro và lưu ý

- Cỡ `promptExcerpt` hợp đồng không nêu; 2 KiB là giả định của solution, cần CR-089 xác nhận.
- `RecordAgentTurn` không idempotent trừ khoá `clientTurnId` (service): không retry ở gateway.
