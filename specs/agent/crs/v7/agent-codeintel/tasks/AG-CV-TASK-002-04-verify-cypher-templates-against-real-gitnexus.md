# AG-CV-TASK-002-04: Re-verify mẫu Cypher trên GitNexus thật và lưu fixture

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.2, 7
**Priority:** P0
**Area:** `agent/` (kiểm chứng; chỉ lệnh đọc)
**File:** `agent/src/relay/codeintel/__fixtures__/gitnexus-1.6.9/*.json` (mới), cập nhật cột `verifiedByCr` ở `gitnexus-cypher-templates.ts`
**Depends on:** [003](./AG-CV-TASK-002-03-gitnexus-cypher-templates-and-runner.md)
**Status:** [ ] TODO

## Context
Điều kiện tiên quyết merge (CR-002 2.1; contract O-3). Chưa mẫu nào được chạy lại khi soạn solution. Quy trình chụp fixture chính thức thuộc AG-CV-SOL-070 (`agent/scripts/capture-codeintel-fixtures.mjs`); task này làm thủ công lần đầu.

## Việc cần làm
1. Với từng mẫu, render bằng giá trị thật rồi chạy `gitnexus cypher -r /opt/repos/orca "<query>"` (ghi stdout qua tệp, không pipe); ghi thời gian.
2. Ưu tiên 6 mẫu chưa chạy: `STEP_EDGES`, `MEMBER_CLUSTER`, `SG_EDGES_AROUND`, `FILE_SYMBOLS_BATCH` (`MATCH (n)` + `IN`), `RT_LIST` (`SKIP`), `OV_COUNT`. Mẫu sai: sửa văn bản hoặc đề xuất cách khác, ghi vào PR và solution mục 7.
3. Xác nhận thứ tự đối số `cypher <query> -r <path>`, ca dấu nháy `'it\'s'`, truy vấn 3 000 hàng không cụt khi ghi tệp, `row_count` khớp.
4. Lưu đầu ra (đã rút gọn, không dữ liệu nhạy cảm) làm fixture; đặt `verifiedByCr:false→true` cho mẫu chạy được.

## Kiểm thử
Test runner (task 03) chạy lại trên fixture. Không có lệnh ghi nào: chỉ `cypher`, `status`.
Lệnh: `pnpm exec vitest run src/relay/gitnexus-cypher-runner.test.ts`

## Tiêu chí hoàn thành
- [ ] Bảng "chạy" trong solution đã cập nhật; mọi mẫu có fixture; thời gian ghi trong PR.

## Rủi ro
- Nếu `IN` trên `MATCH (n)` lỗi, SOL-005 phải đổi `FILE_SYMBOLS_BATCH`; báo ngay.
