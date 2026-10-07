# AG-CV-TASK-002-01: Bộ mã hoá giá trị Cypher và guard chỉ-đọc

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/gitnexus-cypher-literal.ts`, `gitnexus-cypher-guard.ts` (mới) và `gitnexus-cypher-literal.test.ts`, `gitnexus-cypher-guard.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md)
**Status:** [x] DONE

## Context
CLI `gitnexus cypher` không có tham số ràng buộc (CR-002 1.1; `OR 1=1` chạy khi ghép chuỗi thô). Contract §2.4: chỉ bốn bộ mã hoá, guard một câu bắt đầu `MATCH `, cấm từ khoá ghi/`CALL`/`LOAD`/... GitNexus tự chặn ghi nhưng không chặn `CALL|LOAD|INSTALL|ATTACH|EXPORT`.

## Việc cần làm
1. `cypherInt(n,min,max)`, `cypherString(s)` (1..512, cấm NUL/điều khiển kể cả `\n\r\t`, thoát `\`→`\\` rồi `'`→`\'`, bọc nháy đơn, giữ Unicode), `cypherStringList(a)` (1..300), `cypherKindList(a, allowed)` + hằng `GITNEXUS_EDGE_KINDS`; lỗi -> `CODEINTEL_INVALID_PARAMS`.
2. `assertReadOnlyCypher(text)`: ≤ 16 KiB, một câu (không `;` ngoài chuỗi), bắt đầu `MATCH `, bỏ chuỗi nháy đơn (xử lý `\'`, `\\`) rồi từ chối từ nguyên vẹn không phân biệt hoa thường thuộc `CREATE|MERGE|DELETE|SET|REMOVE|DROP|ALTER|COPY|DETACH|CALL|LOAD|INSTALL|ATTACH|EXPORT|IMPORT|FOREACH|UNWIND`.

## Kiểm thử
Bảng ca độc hại: `x' OR 1=1 --`, `\\'`, xuống dòng, NUL, `'; MATCH (a) CALL x`, `it's`; tên hợp lệ `deleteFile`, `Section` tiếng Việt, dấu gạch dài. Phản chứng: mọi mẫu của task 03 qua guard (thêm ở task 03).
Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/gitnexus-cypher-literal.test.ts src/relay/gitnexus-cypher-guard.test.ts`

## Tiêu chí hoàn thành
- [x] Không ca độc hại nào qua guard; `deleteFile` trong chuỗi qua.
- [x] Không có đường ghép chuỗi người dùng ngoài bốn bộ mã hoá.

## Rủi ro
- Quy tắc thoát `\'` được CR ghi "đã parse đúng" nhưng chưa chạy lại; task 04 xác nhận bằng ca có dấu nháy.
