# AG-CV-TASK-003-02: Probe chỉ mục CodeGraph (`indexes.codegraph`)

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 2.3
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codegraph-index-probe.ts`, `codegraph-index-probe.test.ts` (mới)
**Depends on:** [001](./AG-CV-TASK-003-01-codegraph-cli-output-parsing.md), [AG-CV-TASK-001-09](./AG-CV-TASK-001-09-codeintel-status-method-table-and-dispatcher.md)
**Status:** [x] DONE

## Context
Contract §4.1: `state, indexedAt, stats, pendingChanges, backend, journalMode, dbSizeBytes, extractionVersion, reindexRecommended, rootMismatch`. `status` nhận đối số vị trí; không commit (`sources[].commit=null`); `lastIndexed` không đổi khi daemon tự đồng bộ (không là tín hiệu).

## Việc cần làm
1. `codegraph status <projectPath> -j` (`CodeGraphCommand status`), ánh xạ trường; `rootMismatch` từ `worktreeMismatch` của CLI; `pendingChanges` `null` khi gốc chỉ mục ≠ `workspaceRoot`.
2. `state` theo solution 2.3; cảnh báo `codegraph_has_no_commit` khi stale suy ra từ pending.
3. `registerIndexProbe('codegraph', …)`; không gọi lệnh có thể ghi, không đụng daemon.

## Kiểm thử
Fixture status thật; `initialized:false`; `worktreeMismatch`; pending>0; building.
Lệnh: `pnpm exec vitest run src/relay/codegraph-index-probe.test.ts`

## Tiêu chí hoàn thành
- [x] Worktree liên kết: `state:'stale'`, `rootMismatch` khác null, không kết luận `fresh`.

## Rủi ro
- `sync` ở worktree liên kết báo pending của checkout chính.
