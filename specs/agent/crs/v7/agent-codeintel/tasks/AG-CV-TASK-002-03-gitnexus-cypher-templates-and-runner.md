# AG-CV-TASK-002-03: Bảng mẫu Cypher và `runCypherTemplate`

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/gitnexus-cypher-templates.ts`, `gitnexus-cypher-runner.ts` và `gitnexus-cypher-runner.test.ts` (mới)
**Depends on:** [001](./AG-CV-TASK-002-01-gitnexus-cypher-literal-and-guard.md), [002](./AG-CV-TASK-002-02-gitnexus-cypher-markdown-parser.md), [AG-CV-TASK-001-05](./AG-CV-TASK-001-05-run-codeintel-tool-with-tempfile-and-output-classification.md)
**Status:** [x] DONE

## Context
Mẫu là chuỗi hằng với khe `{{ten}}`; bảng đủ ở solution 2.2 (OV_*, PR_*, NODES_BY_ID, STEP_EDGES, MEMBER_CLUSTER, SG_*, FILE_SYMBOLS_BATCH, SYMBOL_FLOWS, RT_*). Dùng `cypher` `ORDER BY ... LIMIT` v.v. đúng văn bản CR-002 2.4; `OV_EDGES` dùng `ca.id <> cb.id`.

## Việc cần làm
1. Khai `CypherTemplate` (id, text, slots, columns, freeTextColumn) cho toàn bộ mẫu, kèm `verifiedByCr: boolean` (false cho STEP_EDGES, MEMBER_CLUSTER, SG_EDGES_AROUND, FILE_SYMBOLS_BATCH, RT_LIST, OV_COUNT).
2. `renderCypherTemplate` (thay khe bằng bộ mã hoá theo `slots`; thiếu/thừa khe -> lỗi), rồi `assertReadOnlyCypher`.
3. `runCypherTemplate(binding, id, slots, ctx)`: render → `runCodeIntelTool({verb:'cypher',query})` → phân loại → `parseCypherOutput`; ghi `perf`.

## Kiểm thử
Mỗi mẫu: không còn `{{`, qua guard, `columns` không có cột cấm; runner với binary giả phát lại fixture; slot độc hại bị từ chối trước spawn (spawn=0).
Lệnh: `pnpm exec vitest run src/relay/gitnexus-cypher-runner.test.ts`

## Tiêu chí hoàn thành
- [x] Mọi mẫu qua guard; mẫu `verifiedByCr:false` được liệt kê cho task 04.

## Rủi ro
- Vị trí đối số `query` so với `-r` chưa kiểm chứng (task 04).
