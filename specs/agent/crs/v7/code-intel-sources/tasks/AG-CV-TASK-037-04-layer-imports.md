# AG-CV-TASK-037-04: `layerImports`: 4 cặp, loại trừ, khử trùng theo thư mục đích

**From Solution:** [AG-CV-SOL-037-structural-facts](../solutions/AG-CV-SOL-037-structural-facts.md) mục 5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel-structural-layer-imports.ts`, `codeintel-structural-facts-queries.ts` (mới) + test
**Depends on:** AG-CV-TASK-037-01, 037-02, AG-CV-SOL-002 (Cypher runner)
**Status:** [ ] TODO

## Context

Hợp đồng §4.9: cặp đoạn đường dẫn cố định, loại `_test.go`; CR-037 2.3 loại thêm `usecasetest/`, `testutil`, `cmd/`, `proto/gen`, `*.pb.go`.

## Việc cần làm

1. Hằng `LAYER_PAIRS`: `usecase->adapter` (`/internal/usecase/` → `/internal/adapter/`), `domain->usecase`, `domain->adapter`, `adapter->adapter`.
2. Mẫu (theo fixture đã xác nhận ở task 01): `MATCH (a:File)-[r:CodeRelation {type:'IMPORTS'}]->(b:File) WHERE a.filePath CONTAINS {{fromSeg}} AND b.filePath CONTAINS {{toSeg}} AND NOT a.filePath ENDS WITH '_test.go' AND NOT a.filePath CONTAINS '/usecasetest/' AND NOT a.filePath CONTAINS '/testutil/' AND NOT a.filePath CONTAINS '/cmd/' AND NOT a.filePath ENDS WITH '.pb.go' AND ({{prefixClause}}) RETURN a.filePath, b.filePath`; `{{prefixClause}}` ghép từ tối đa 20 `a.filePath STARTS WITH <cypherString>`; mọi slot qua bộ mã hoá, `assertReadOnlyCypher` chạy trên chuỗi cuối.
3. `adapter->adapter`: giữ hàng khi hai phân đoạn `adapter/<x>` khác nhau.
4. Khử trùng: khoá `(pair, fromFile, dirname(toFile))`, giữ `toFile` nhỏ nhất từ điển; sắp `(pair, fromFile, toFile)`.
5. `pair` không đặt → chạy cả 4 (tối đa 2 song song theo `gitnexus ≤ 2`/dev server).

## Kiểm thử

Fixture task 01 → 2 hàng `usecase->adapter` cho package có 3 tệp (khử trùng), `usecasetest` bị loại, `domain->*` rỗng; `pair` đơn; `pathPrefixes` loại service khác; `assertReadOnlyCypher` chạy trên mọi mẫu (test duyệt hằng); slot có dấu `'` được mã hoá. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-structural-layer-imports.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mỗi import package ra một hàng; kết quả xác định.

## Rủi ro

Cú pháp `STARTS WITH` chuỗi OR chưa chạy (task 01); 38 337 cạnh `IMPORTS` backend-go: truy vấn luôn có điều kiện đoạn đường dẫn.
