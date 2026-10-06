# AG-CV-TASK-002-02: Parser đầu ra markdown của `gitnexus cypher`

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.3
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/gitnexus-cypher-markdown-parser.ts`, `gitnexus-cypher-markdown-parser.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-05](./AG-CV-TASK-001-05-run-codeintel-tool-with-tempfile-and-output-classification.md) (phân loại `{"error"}`) 
**Status:** [ ] TODO

## Context
CR-002 1.4–1.5: ba dạng đầu ra (`{markdown,row_count}`, `[]`, `{error}`); markdown không thoát ` | `, xuống dòng bị gộp, ô rỗng gồm cả `null`, mảng là chuỗi JSON có nháy đơn thừa (`["'comm_7339'"]`). Chưa chạy lại; dùng chuỗi thật từ CR làm fixture.

## Việc cần làm
1. `parseCypherOutput(stdoutText, template)` -> `{rows, rowCount, skippedRows, warnings}` theo solution 2.3: tiêu đề khớp đúng thứ tự/số cột (`unexpected_columns`), tách ` | `, gộp ô thừa vào `freeTextColumn` (một, ở cuối), hàng lệch bị bỏ + cảnh báo, kiểu `string|nullableString|int|float|jsonArray`, đối chiếu `row_count` (`row_count_mismatch` chỉ cảnh báo).
2. Từ chối mẫu chọn cột `content|description|docstring` (hàm `assertNoFreeTextColumns(template)` dùng ở task 03).

## Kiểm thử
Tiêu đề+phân cách; ô `["'comm_7339'","'comm_7395'"]`; id `Section:CLAUDE.md:L23:GitNexus — Code Intelligence`; ô rỗng; ` | ` ở cột cuối (gộp) và giữa (bỏ hàng + cảnh báo); `row_count` lệch; `[]`; `{"error"}`; JSON cụt.
Lệnh: `pnpm exec vitest run src/relay/gitnexus-cypher-markdown-parser.test.ts` (trong `/opt/repos/orca/agent`)

## Tiêu chí hoàn thành
- [ ] Mọi ca trên xanh; không bao giờ đoán cột.

## Rủi ro
- Định dạng nội bộ GitNexus 1.6.9, có thể đổi: fixture vàng (AG-CV-SOL-070).
