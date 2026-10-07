# AG-CV-TASK-002-07: Probe chỉ mục GitNexus cho `codeintel.status`

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.5
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/gitnexus-index-probe.ts`, `gitnexus-index-probe.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-06](./AG-CV-TASK-001-06-gitnexus-registry-and-repo-resolution.md), [AG-CV-TASK-001-09](./AG-CV-TASK-001-09-codeintel-status-method-table-and-dispatcher.md)
**Status:** [x] DONE

## Context
Contract §4.1 khối `indexes.gitnexus`: `state, indexedCommit, indexedAt, branch, stats, schemaVersion, storagePath, indicators`. Đã thấy `.gitnexus/meta.json` (cấp gốc có `lastCommit`, `indexedAt`; 2,8 MB do `fileHashes`) và bốn tệp `lbug.wal.missing-shadow.*`. Marker hỗ trợ `schemaVersion ∈ {5}`.

## Việc cần làm
1. `gitnexusIndexProbe(binding, ctx)`: lấy từ phần tử registry; `missing` nếu không có `.gitnexus/lbug`; `building` nếu có job reindex GitNexus (hàm tiêm từ SOL-004, mặc định false); `stale` nếu `indexedCommit !== headCommit` hoặc `worktreeMismatch`; `indicators` đếm `lbug.wal.missing-shadow.*`.
2. `schemaVersion`: đọc `meta.json` một lần theo `mtime`; quá 200 ms thì bỏ và nhớ quyết định; ngoài `{5}` -> `supported:false`.
3. `registerIndexProbe('gitnexus', …)`.

## Kiểm thử
Thư mục tạm với `.gitnexus/lbug`, `meta.json` nhỏ, 4 tệp shadow: `ready/stale/missing/building`; `schemaVersion 5` vs `6`; cache theo `mtime`; không spawn CLI nào.
Lệnh: `pnpm exec vitest run src/relay/gitnexus-index-probe.test.ts`

## Tiêu chí hoàn thành
- [x] Không gọi CLI/Cypher; `wal_missing_shadow_files:4` xuất hiện.

## Rủi ro
- Chi phí parse `meta.json` chưa đo.
