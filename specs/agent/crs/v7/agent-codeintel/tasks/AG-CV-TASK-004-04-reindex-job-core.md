# AG-CV-TASK-004-04: Lõi job reindex (hàng đợi, khoá, outcome)

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.3
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-reindex-job.ts`, `codeintel-reindex-job.test.ts` (mới)
**Depends on:** [002](./AG-CV-TASK-004-02-reindex-commands-and-progress-parser.md), [003](./AG-CV-TASK-004-03-reindex-journal.md)
**Status:** [x] DONE

## Context
Contract §4.10: params `mode,tools,trigger,ifStale,expectHead`; trạng thái và `outcome ∈ already_up_to_date|superseded|skipped_scope_repo_root|""`; mỗi repo một job; `MAX_REINDEX` 1 (≤ 2); hàng đợi ≤ 4.

## Việc cần làm
1. `startReindex(binding, params, deps)`: validate (`tools`, `mode`, `trigger`, `ifStale`, `expectHead` qua `assertGitRef`-kiểu); `ORCA_CODEINTEL_REINDEX=off`; worktree liên kết theo `trigger`; `ifStale`/`expectHead` ngắn mạch; `tools` mặc định mọi công cụ khả dụng và được hỗ trợ.
2. Máy trạng thái + `getJob(jobId|latest)`, ULID `ri_…`, hàng đợi, khoá `realpath(repoRoot)`, `REINDEX_IN_PROGRESS`/`queue_full`.
3. Trả kết quả `{jobId,state,workspaceRoot,repoRoot,mode,tools,trigger,startedAt,estimate:null,outcome,skipped}` (không phong bì).

## Kiểm thử
Hai job cùng repo; repo khác vào hàng đợi; hàng đầy; `manual` vs `agent_done` worktree liên kết; `ifStale` fresh; `expectHead` lệch; `reindex=off`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-reindex-job.test.ts`

## Tiêu chí hoàn thành
- [x] Không spawn trong ca ngắn mạch.

## Rủi ro
- `freshness` chỉ đúng khi AG-CV-SOL-080 xong; trước đó `ifStale` dùng `indexedCommit===headCommit`.
