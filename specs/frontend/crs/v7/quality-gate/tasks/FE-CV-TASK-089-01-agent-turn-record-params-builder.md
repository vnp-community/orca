# FE-CV-TASK-089-01: Hàm dựng tham số `quality.turn.record` và digest

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 2.2, 2.3 (3, 4)
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/agent-turn-record-params.ts`, `frontend/src/renderer/src/lib/agent-turn-digest.ts` (đều mới) + test
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu `AgentTurn`)
**Status:** [x] DONE (verified 2026-10-07: 10 + 3 tests (agent-turn-record-params.test.ts, agent-turn-digest.test.ts))

## Context

- Hợp đồng 3.2 liệt kê tham số; không có trường prompt; args ≤ 16 KiB.
- `AgentStatusEntry.prompt` đã cắt 200 ký tự; `lib/sha256.ts` chạy cả ngữ cảnh không bảo mật.
- Không có `model` ở renderer.

## Việc cần làm

1. `sha256Hex` + `normalizePrompt` (trim, gộp khoảng trắng, NFC).
2. `buildAgentTurnRecordParams`: `clientTurnId=${paneKey}:${stateStartedAt}`, `endedAt` RFC 3339, `startedAt` từ mốc `working` gần nhất trong `stateHistory` nếu có, `filesDigest` = sha256 của danh sách định danh tệp đã sắp.
3. Trả `null` khi thiếu `headOid`/`projectId`/worktree nổi.
4. `promptExcerpt` chỉ khi cờ tenant bật VÀ có hàm che (`maskSensitiveText` chưa tồn tại: mặc định không gửi).

## Kiểm thử

- Vector SHA-256 đã biết; snapshot tập khoá đầu ra ⊂ hợp đồng; không có prompt thô; bảng ca null.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không prompt/lastAssistantMessage/toolInput thô trong đầu ra.
- [ ] Hàm thuần.

## Rủi ro

- Digest trên prompt đã cắt 200 ký tự.

## Ghi chú triển khai (2026-10-07)

Viết lại theo hợp đồng 3.2 (camelCase, endHeadCommit, treeDirtyEnd, promptDigest, filesChangedCount...); bản cũ dùng tên trường tự đặt và hash FNV giả. sha256 dùng lib/sha256.ts.
