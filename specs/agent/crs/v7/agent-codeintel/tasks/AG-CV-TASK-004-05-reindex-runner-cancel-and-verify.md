# AG-CV-TASK-004-05: Runner reindex: spawn nhóm tiến trình, stage, verify, huỷ

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.3
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-reindex-runner.ts`, `codeintel-reindex-runner.test.ts` (mới)
**Depends on:** [004](./AG-CV-TASK-004-04-reindex-job-core.md), [AG-CV-TASK-001-04](./AG-CV-TASK-001-04-run-tool-command-options-and-legacy-tool-guard.md) (`detached`, `signal`, `killGraceMs`)
**Status:** [ ] TODO

## Context
Thành công = exit 0 và `verify`; huỷ SIGTERM nhóm rồi SIGKILL sau 10 s rồi verify; timeout 45 phút/công cụ; không xoá tệp chỉ mục.

## Việc cần làm
1. `runReindexJob(job)`: stage `preflight→codegraph.*→gitnexus.analyze→verify→done`, phát `reindexProgress` qua sink; giữ 64 KiB đuôi stdout/stderr.
2. `verify` gọi lại probe GitNexus/CodeGraph; kết quả `index_not_updated`/`already_up_to_date`; sau huỷ `indexHealth:'unknown'` khi probe lỗi.
3. `cancelReindex(jobId)` idempotent; sau thành công: `reason:'reindex'` indexChanged + huỷ cache.

## Kiểm thử
Binary giả: tiến độ, 0/1, treo (huỷ, timeout giả), bỏ qua SIGTERM (SIGKILL), sinh con (kill nhóm, `describe.skipIf(win32)`); verify đổi/không đổi `meta.json`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-reindex-runner.test.ts`

## Tiêu chí hoàn thành
- [ ] Cả cây tiến trình biến mất ≤ 15 s sau huỷ.

## Rủi ro
- Huỷ giữa analyze có thể để DB dở dang: chưa kiểm chứng (task 09).
