# AG-CV-TASK-002-08: Handler `overview`, `processes`, `process`, `routes`

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.6
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-gitnexus-overview.ts`, `-processes.ts`, `-process.ts`, `-routes.ts` (mới), `codeintel-method-table.ts` (sửa), `codeintel-gitnexus-methods.test.ts` (mới)
**Depends on:** [003](./AG-CV-TASK-002-03-gitnexus-cypher-templates-and-runner.md), [004](./AG-CV-TASK-002-04-verify-cypher-templates-against-real-gitnexus.md), [005](./AG-CV-TASK-002-05-codeintel-symbol-ref.md), [006](./AG-CV-TASK-002-06-codeintel-short-lived-cache.md), [007](./AG-CV-TASK-002-07-gitnexus-index-probe.md)
**Status:** [x] DONE

## Context
Contract §4.2, 4.3, 4.7. `overview`: tham số `topN` 1..500 (200), `maxEdges` ≤ 5000, `edgeKinds`, `withTopFiles`; `totalCount` = số `Community` thật (9 039) + `communities_count_mismatch`. `process` không có -> `SYMBOL_NOT_FOUND`. Lưu ý Process của GitNexus không đi vào DB backend (README v7 mục 8 điểm 9).

## Việc cần làm
1. Mỗi handler: `validate` (khoá lạ bị từ chối), repo, kiểm công cụ/chỉ mục, cache, dựng phong bì (`lineBase 1`, `perf`).
2. `overview`: `OV_CLUSTERS`∥`OV_COUNT` rồi `OV_EDGES`∥`OV_TOPFILES`; top-3 tệp mỗi cụm gom ở agent; `area`, `dominantLanguage`; lọc cạnh về cụm trong `topN`.
3. `processes`: `PR_LIST`∥`PR_COUNT`, `NODES_BY_ID`. `process`: `PR_ONE`, `PR_STEPS`(200), `STEP_EDGES(['CALLS'])`, `MEMBER_CLUSTER`.
4. `routes`: `RT_LIST`∥`RT_COUNTS`, `side`, `handler` kind `file`, `routes_coverage_js_only`.
5. Đăng ký 4 dòng ở bảng method (timeout 25 s).

## Kiểm thử
Binary giả phát lại fixture task 04: hình dạng đúng contract; `topN` cắt + `truncated`; cạnh có hai đầu trong tập nút; `proc_0_checkspanel` 9 bước; phân trang 300 luồng không trùng; 92+21 route. Phản chiếu schema không có khoá cấm.
Lệnh: `pnpm exec vitest run src/relay/codeintel-gitnexus-methods.test.ts`

## Tiêu chí hoàn thành
- [x] Các tiêu chí `overview/processes/process/routes` ở solution mục 5.

## Rủi ro
- `overview` ~6 s, 3-4 tiến trình/lần; cổng 3 có thể xếp hàng.
