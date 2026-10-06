# AG-CV-TASK-037-05: `importInDegree` và `fileSizes` (gộp `Function` và `Method`)

**From Solution:** [AG-CV-SOL-037-structural-facts](../solutions/AG-CV-SOL-037-structural-facts.md) mục 5.3
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel-structural-file-metrics.ts` (mới), mẫu bổ sung trong `codeintel-structural-facts-queries.ts`, `.test.ts`
**Depends on:** AG-CV-TASK-037-01, 037-02, 037-04 (file queries)
**Status:** [ ] TODO

## Context

CR-037 2.2: `importInDegree` = `count(DISTINCT a)` theo `b.filePath`; `fileSizes` = hai truy vấn theo nhãn (không hỗ trợ `(s:A OR s:B)`) gộp ở agent.

## Việc cần làm

1. `importInDegree`: `MATCH (a:File)-[r:CodeRelation {type:IMPORTS}]->(b:File) WHERE <prefix on b.filePath> RETURN b.filePath, count(DISTINCT a) ORDER BY … LIMIT {{n}}`; `n = offset + limit` (≤ 10 000); sắp `inDegree` giảm, `file` tăng.
2. `fileSizes`: cho mỗi nhãn `Function`, `Method`: `MATCH (f:File)-[:CodeRelation {type:DEFINES}]->(s:<Label>) … RETURN f.filePath, count(s), sum(s.endLine - s.startLine + 1), max(s.endLine - s.startLine + 1)`; gộp theo tệp (cộng `functions`, `totalLines`; `max` lấy lớn hơn).
3. `longest.name`: theo kết quả task 01 (nếu có truy vấn lấy được tên); nếu không → `name:""` kèm `warnings:["longest_symbol_name_unavailable"]` (mã tạm; xem solution, câu hỏi mở 1).
4. Lọc `pathPrefixes` trên `f.filePath`; loại `_test.go`.
5. Phân trang theo hàng sau khi sắp (`file` tăng) cho `fileSizes`.

## Kiểm thử

Fixture: tệp có cả hàm và method (gộp đúng); một nhãn rỗng; sắp xếp xác định; `longest` fallback; cú pháp `(s:A OR s:B)` không có trong mẫu (test quét). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-structural-file-metrics.test.ts`.

## Tiêu chí hoàn thành

- [ ] Tổng hàm/dòng đúng khi gộp hai nhãn; không `OR` nhãn.

## Rủi ro

Hàm lồng/closure đếm riêng; tính theo `endLine - startLine + 1` chỉ là độ dài, không phải độ phức tạp (nhãn đúng ở backend).
