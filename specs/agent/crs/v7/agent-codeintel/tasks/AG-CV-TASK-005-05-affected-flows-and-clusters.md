# AG-CV-TASK-005-05: Luồng và cụm bị ảnh hưởng

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-detect-changes-flows.ts`, `codeintel-detect-changes-flows.test.ts` (mới)
**Depends on:** [AG-CV-TASK-002-03](./AG-CV-TASK-002-03-gitnexus-cypher-templates-and-runner.md), [AG-CV-TASK-002-04](./AG-CV-TASK-002-04-verify-cypher-templates-against-real-gitnexus.md)
**Status:** [x] DONE

## Context
`SYMBOL_FLOWS` (đã chạy), `MEMBER_CLUSTER` (chưa chạy). CLI lặp luồng và chỉ 10 dòng; ta gom mỗi luồng một lần.

## Việc cần làm
1. `findAffectedFlows(symbolIds)`: nhóm 300, gom theo `p.id`, `changedSymbolKeys`, `earliestChangedStep=min(step)`, cắt 200 theo số symbol chạm.
2. `findAffectedClusters(ids)` khi `withClusters` -> `[{id,label,changedSymbols}]` (nhãn nếu có, backend ghép từ overview).

## Kiểm thử
Nhiều symbol cùng luồng, nhiều nhóm, cắt 200, cụm; binary giả phát lại fixture.
Lệnh: `pnpm exec vitest run src/relay/codeintel-detect-changes-flows.test.ts`

## Tiêu chí hoàn thành
- [x] Mỗi luồng xuất hiện một lần.

## Rủi ro
- 7 nhóm song song 3 ≈ 5–6 s (ước tính).
