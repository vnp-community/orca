# AG-CV-TASK-082-05: Parser oxlint (json + github), tsc text, vitest json

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.4
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-parser-oxlint.ts`, `quality-parser-tsc.ts`, `quality-parser-vitest.ts` + test
**Depends on:** AG-CV-TASK-082-01, 082-02, 082-04
**Status:** [ ] TODO

## Context

CR-082 2.7. **Chưa chạy**: dùng fixture task 01; nếu hình dạng thật khác CR thì sửa theo fixture và ghi vào PR.

## Việc cần làm

1. `oxlint@json`: `diagnostics[]` (`message`, `code` dạng `plugin(rule)`, `severity`, `filename`, `labels[0].span`, `help`) → `RawQualityFinding`; `oxlint@github` dự phòng (dòng `::error title=…,file=…,line=…,col=…::msg`).
2. `tsc@text`: `path(line,col): error TSnnnn: message`, dòng tiếp nối thụt đầu dòng (tối đa 10 dòng, 2 KiB); lỗi toàn cục không tệp (`file:""`).
3. `vitest@json`: mỗi `assertionResults[].status==="failed"` → phát hiện (`testAnchor = ancestorTitles.join(" › ") + " › " + title`); `testResults[].status==="failed"` không có assertion lỗi → `vitest/suite-failed`; bỏ qua skipped/todo (chỉ đếm); đọc `extraPath` (vitest.json).
4. Shape guard từng parser: thiếu khoá bắt buộc, JSON cụt → `failure: format_drift`.

## Kiểm thử

Mỗi file test: hợp lệ (fixture), rỗng, nhiều phát hiện, Unicode, CRLF, đường dẫn có khoảng trắng, JSON cụt, khoá lạ, thông điệp rất dài. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-parser-oxlint.test.ts src/relay/quality-parser-tsc.test.ts src/relay/quality-parser-vitest.test.ts`.

## Tiêu chí hoàn thành

- [ ] Khớp `*.expected.json`; drift được phát hiện; không ném.

## Rủi ro

tsc 7.0.2 hình dạng dòng chưa biết; `vitest` vị trí dòng có thể chỉ có trong stack.
