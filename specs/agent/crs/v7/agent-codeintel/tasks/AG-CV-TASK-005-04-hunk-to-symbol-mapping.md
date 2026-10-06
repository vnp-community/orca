# AG-CV-TASK-005-04: Ánh xạ hunk -> symbol và độ tin cậy theo tệp

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-hunk-symbol-mapping.ts`, `codeintel-hunk-symbol-mapping.test.ts` (mới)
**Depends on:** [003](./AG-CV-TASK-005-03-diff-collection-untracked-and-fallback.md), [AG-CV-TASK-002-04](./AG-CV-TASK-002-04-verify-cypher-templates-against-real-gitnexus.md), [AG-CV-TASK-002-05](./AG-CV-TASK-002-05-codeintel-symbol-ref.md)
**Status:** [ ] TODO

## Context
`FILE_SYMBOLS_BATCH` (CR-002) nhóm 100 đường dẫn, song song ≤ 3 theo cổng; dòng +1; `File` không có dòng; số dòng chỉ mục là của lúc `analyze`.

## Việc cần làm
1. `mapHunksToSymbols(files, indexInfo, deps)` theo quy tắc solution 2.2 (giao khoảng, innermost + `containers`, `value`/`Section`, `confidence`, `mappingConfidence`, `driftedFileCount`).
2. `D`/`A`/không có nút -> `unmapped.filesNotIndexed|filesWithoutSymbols`; cắt 2 000 symbol ưu tiên `additions+deletions`; `filesBeyondCap`.
3. `git diff --name-only -z <indexedCommit> <headOid>` chỉ khi `cat-file -e <indexedCommit>^{commit}`; không thì `index_commit_unreachable`.

## Kiểm thử
Method trong class, hằng cấp tệp, `Section`, xoá thuần, tệp bẩn `approximate`, chỉ mục khớp `exact`, cắt 2 000, cơ số dòng.
Lệnh: `pnpm exec vitest run src/relay/codeintel-hunk-symbol-mapping.test.ts`

## Tiêu chí hoàn thành
- [ ] Không bịa symbol cho mã mới chưa index.

## Rủi ro
- Giả định tệp bẩn luôn `approximate` (chưa biết `analyze` đọc working tree hay commit).
