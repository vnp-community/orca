# AG-CV-TASK-083-06: Handler `quality.coverage`

**From Solution:** [AG-CV-SOL-083-coverage-collection](../solutions/AG-CV-SOL-083-coverage-collection.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-coverage-handler.ts` (mới), dòng `quality.coverage` trong `quality-method-table.ts`, `.test.ts`
**Depends on:** AG-CV-TASK-083-05, AG-CV-TASK-081-08
**Status:** [ ] TODO

## Context

Hợp đồng §5.6; PQ-21; timeout agent 10 s.

## Việc cần làm

1. Validate `{workspaceRoot, runId}`; `runId` không thuộc worktree → `RUN_NOT_FOUND`.
2. Run `queued|running|cancelling` → `RUN_IN_PROGRESS data.runId`; run `cancelled` → `RUN_CANCELLED`; không có bước coverage → `{ runId, available:false, reason:"no_coverage_step" }`; TS thiếu provider → `ENV_NOT_READY reason="coverage_provider_missing"`; hết TTL/`interrupted` → `{available:false, reason:"expired"}` (đề xuất; hợp đồng chưa có lý do này — câu hỏi mở của solution).
3. Thành công → `{ runId, available:true, report }` đúng ví dụ §5.6 (`headCommit`, `baseCommit`, `dirty` từ run).

## Kiểm thử

Bảng trạng thái run → đáp ứng; JSON khớp ví dụ hợp đồng; `runId` lạ. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-coverage-handler.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mọi nhánh lỗi có `data.code` đúng §3.2.

## Rủi ro

`reason:"expired"` là đề xuất ngoài hợp đồng.
