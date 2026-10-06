# AG-CV-TASK-070-04: Test vàng parser `cypher` của GitNexus

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/gitnexus-cypher-output.golden.test.ts` (mới), `agent/src/relay/codeintel/__fixtures__/gitnexus/1.6.9/cypher-*.json` + `*.expected.json` (mới)
**Depends on:** 070-02, 070-03; AG-CV-SOL-002 (parser)
**Status:** [ ] TODO

## Context

Hợp đồng: agent-rpc §§2.4, §3.2, §10. Chưa chạy; mọi hình dạng đầu ra công cụ lấy từ CR-CV-070 (chạy thử 2026-10-05), chưa chạy lại.
Quan sát CR-070 §1: có hàng → `{markdown,row_count}`; không hàng → `[]` trần; lỗi → `{error}` **exit 0**; ô không thoát `|`, gộp xuống dòng; mảng hiện là `[]`.
Tên hàm tham khảo `parseCypherOutput`; tên thật do 002.

## Việc cần làm

1. Chụp/đặt tệp: `cypher-rows.json`, `cypher-empty.json`, `cypher-empty-markdown.json`, `cypher-error.json`, `cypher-write-blocked.json`, `cypher-pipe-newline-cells.json`, `cypher-communities-array.json`.
2. Viết test: cả hai hình dạng rỗng → 0 hàng; ô chứa `|` và `\n`; chuỗi `'comm_x'` bỏ nháy; số cột khớp tiêu đề, **dòng lệch bị bỏ + `warnings`**; CRLF (sinh trong test từ LF); `{error}` → `TOOL_FAILED`, với `Write operations` → `reason=write_blocked`, `not found|does not exist` → `SYMBOL_NOT_FOUND`/`INVALID_PARAMS`; còn lại `unknown_shape`.
3. `TestParserGolden`: khớp từng byte với `*.expected.json` (khoá sắp xếp).
4. Thử ngược: bản sao trong bộ nhớ đổi tên cột/bớt khoá → `format_drift`, thông điệp nêu tệp, không chứa đầu ra thô.

## Kiểm thử

- Các test ở mục 2; thêm một bảng ca (`it.each`).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Phủ đủ danh sách; golden khớp; không tin mã thoát.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Hình dạng thật phải được xác nhận bằng script chụp (task 03) trước khi tin tệp dựng tay.
