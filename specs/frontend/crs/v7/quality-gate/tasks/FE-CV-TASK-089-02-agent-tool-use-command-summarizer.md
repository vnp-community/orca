# FE-CV-TASK-089-02: Bộ chuẩn hoá lệnh và thu thập công cụ theo pane

**From Solution:** [FE-CV-SOL-089-agent-turn-recorder](../solutions/FE-CV-SOL-089-agent-turn-recorder.md) mục 2.3 (2)
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/turns/agent-tool-use-command-summarizer.ts` (mới) + test
**Depends on:** FE-CV-TASK-089-01 (kiểu `AgentTurnCommandsSummary`)
**Status:** [ ] TODO

## Context

- Chỉ giữ tên chương trình + tối đa một tiểu lệnh; bỏ đối số, đường dẫn, env, URL, chuyển hướng.
- Chỉ `toolName`/`toolInput` hiện tại có ở renderer; phải lấy mẫu mỗi lần `setAgentStatus`.

## Việc cần làm

1. `normalizeToolInput` ánh xạ `category` (test|lint|typecheck|build|install|git|other).
2. `createAgentTurnToolCollector.observe/take/reset`: đếm khi `state==="working"`, `updatedAt` tăng, cặp công cụ đổi.
3. Trần 20 `commands`, 32 khoá `toolCounts`, `truncated`.

## Kiểm thử

- Bảng ca: `pnpm test --filter x`, `cd /tmp && rm -rf`, URL, `curl -H "Authorization"`, 200 ký tự cắt giữa UTF-8; fuzz không ném; collector hai ping cùng công cụ.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Đầu ra không chứa đối số/đường dẫn/secret.
- [ ] Không ném với chuỗi bất kỳ.

## Rủi ro

- Đếm có thể lệch (chưa kiểm chứng ping hook).
