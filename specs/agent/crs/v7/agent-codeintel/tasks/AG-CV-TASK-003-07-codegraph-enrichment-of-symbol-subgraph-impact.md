# AG-CV-TASK-003-07: Làm giàu `symbol`, `subgraph`, `impact` bằng CodeGraph

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 2.2
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-codegraph-enrichment.ts` (mới), sửa nhẹ `codeintel-gitnexus-symbol.ts`, `-subgraph.ts`, `-impact.ts`; test `codeintel-codegraph-methods.test.ts` (sửa)
**Depends on:** [003](./AG-CV-TASK-003-03-codeintel-symbol-ref-codegraph.md), [004](./AG-CV-TASK-003-04-codegraph-sqlite-readonly-reader.md), [005](./AG-CV-TASK-003-05-codegraph-search-and-files-methods.md), [006](./AG-CV-TASK-003-06-codegraph-affected-tests.md), [AG-CV-TASK-002-09](./AG-CV-TASK-002-09-gitnexus-subgraph-impact-symbol.md)
**Status:** [ ] TODO

## Context
Contract §4.4–4.6: `symbol` thêm `codegraphId`, `signature`, `docstring`, `isExported`; `subgraph source:'codegraph'|'auto'` chỉ tầng `calls` khi SQLite khả dụng; `impact.testsCovering` từ `affected` khi `includeTests`; `symbol.includeTrail` thử nghiệm (`experimental:true`, text từ `codegraph node`, parse tối thiểu, lỗi bỏ im lặng + cảnh báo).

## Việc cần làm
1. Điểm cắm trong handler SOL-002 (hàm `enrich*` tuỳ chọn, no-op khi thiếu CodeGraph).
2. Khớp symbol theo `(file_path,name,kind)`; nhiều khớp -> `AMBIGUOUS_SYMBOL`; không SQLite -> CLI theo tên + `codegraph_name_based_resolution`.
3. Hợp nhất cạnh với `sources:["gitnexus","codegraph"]`; `sources_disagree` khi lệch.
4. `includeTrail` qua `codegraph node`; không công khai method `node`.

## Kiểm thử
`ORCA_CODEINTEL_SQLITE=off` vs `auto`; người gọi từ SQL không lẫn `spawn`/`kill` tệp khác; `testsCovering`; trail lỗi parse không làm vỡ.
Lệnh: `pnpm exec vitest run src/relay/codeintel-codegraph-methods.test.ts src/relay/codeintel-gitnexus-methods.test.ts`

## Tiêu chí hoàn thành
- [ ] GitNexus-only vẫn chạy khi CodeGraph vắng.

## Rủi ro
- Sửa handler SOL-002: chạy `gitnexus impact` cho các hàm handler trước khi sửa.
