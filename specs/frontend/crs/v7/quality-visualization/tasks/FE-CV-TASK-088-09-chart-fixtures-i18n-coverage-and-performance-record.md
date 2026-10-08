# FE-CV-TASK-088-09: Fixture cỡ trần, phủ khoá i18n, đo hiệu năng, ghi quyết định thư viện

**From Solution:** [FE-CV-SOL-088](../solutions/FE-CV-SOL-088-graphics-foundation-and-chart-primitives.md) mục 2.1, 2.8, 4, 5
**Priority:** P1
**Area:** frontend / test-support + i18n
**File:** `frontend/src/renderer/src/test-support/quality-chart-fixtures.ts` (mới), `frontend/src/renderer/src/i18n/code-intel-quality-locale-coverage.test.ts` (mới), `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa; khoá `auto.components.qualityCharts.*`), `frontend/src/renderer/src/components/quality-charts/__tests__/quality-chart-performance.test.ts` (mới)
**Depends on:** FE-CV-TASK-088-03 đến 088-08
**Status:** [x] DONE (verified 2026-10-07: code-intel-quality-locale-coverage 11/11, quality-chart-performance 5/5; full quality-charts suite 25 files / 165 tests PASS)

## Context

- `i18n/task-jira-link-locale-coverage.test.ts` là mẫu: khoá "read-by-name" (không băm) không được sinh catalog tự động, nên test này giữ cho 4 locale không âm thầm hiển thị tiếng Anh.
- `i18n/no-top-level-translate.test.ts` cấm `translate()` ở cấp module.
- Test node + happy-dom ở `config/vitest.config.ts` (`include: src/**/*.test.ts(x)`).
- Ngân sách SOL-088 2.8 chưa có số đo; số đo đầu ghi vào PR. CR-071 chưa có chỗ chứa ngân sách frontend (câu hỏi mở 5).
- File `code-intel-quality-locale-coverage.test.ts` **dùng chung với 087**: 088 tạo file và danh sách khoá của `qualityCharts`; 087-08 thêm khoá `reviewQuality`/`codeIntelQuality`.

## Việc cần làm

1. `quality-chart-fixtures.ts`: hàm tất định có hạt giống: `buildTreemapItems(n=400)`, `buildHeatmapRows(rows=40, cols=6, nullRate)`, `buildDependencyGraph(nodes=60|150, density, cycles)`, `buildTrendSeries(points=50, series=4, nullRate)`, `buildSeverityCounts()`; không phụ thuộc kiểu hợp đồng (tách khỏi 087).
2. Khoá i18n: gom mọi khoá `auto.components.qualityCharts.*` đã dùng ở 088-03..08 vào danh sách trong test; thêm vào đủ 5 locale (en gốc; es, ja, ko, zh có bản dịch thật, không để tiếng Anh).
3. `code-intel-quality-locale-coverage.test.ts`: với mỗi khoá trong danh sách, 5 locale đều có chuỗi không rỗng và (trừ en) khác bản tiếng Anh, trừ ngoại lệ có chú thích (từ viết tắt như "HEAD").
4. `quality-chart-performance.test.ts`: đo `squarify(400)` ≤ 16×5 ms, `orderByStrongComponents(150)` ≤ 50×5 ms, render lần đầu `MetricTreemap`/`HotspotHeatmap`/`DependencyMatrix` với fixture trần ≤ 250×5 ms trong happy-dom (nhiễu cao; mục đích bắt hồi quy lớn); in số đo thật bằng `console.info` để ghi vào PR.
5. Quyết định ghi vào **mô tả PR** (không tạo file `.md`): bảng A/B/C của SOL-088 2.1, kèm "chưa kiểm chứng" còn lại và điều kiện xem lại A2.
6. (Tuỳ chọn, P2) spec Playwright web `tests/e2e/…` nạp fixture trần qua trang thử và dùng `performance.mark/measure`; chỉ khi hạ tầng web e2e chạy được (`tests/playwright.web.config.ts` tồn tại; chưa xác nhận chạy trong CI).

## Kiểm thử

- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/i18n/code-intel-quality-locale-coverage src/renderer/src/components/quality-charts/__tests__/quality-chart-performance src/renderer/src/i18n/no-top-level-translate`.
- Fixture tất định: gọi hai lần cho kết quả bằng nhau.

## Tiêu chí hoàn thành

- [ ] 5 locale phủ đủ khoá `qualityCharts`; test xanh.
- [ ] `no-top-level-translate` xanh.
- [ ] Số đo đầu tiên ghi vào PR; ghi chú quyết định A/B/C trong PR.
- [ ] Không dependency mới trong diff.

## Rủi ro

- Test thời gian trong happy-dom dễ nhiễu CI: ngưỡng ×5 và có thể đánh dấu để bỏ qua khi `process.env.CI` đặt tải thấp (không `skip` im lặng; ghi lý do).
- Bản dịch máy cho 4 locale cần người đọc duyệt.

## Ghi chú triển khai (2026-10-07)

Ghi nhận hiệu năng (happy-dom, ngưỡng x5): squarify(400) 0,99 ms; DSM(150) 1,01 ms; render đầu treemap(400) 225,5 ms (sát ngân sách 250 ms nhưng gồm khởi động lạnh), heatmap 60,9 ms, matrix(60 nút/218 cạnh) 99,6 ms. i18n: 78 khoá `auto.components.qualityCharts.*` ở 5 locale (bản dịch 4 locale cần người đọc duyệt; ngoại lệ trùng tiếng Anh: es severity.error, es/ko treemap.tileLabel). Quyết định A1 (tự viết SVG, không dependency) ghi ở SOL-088 mục 2.1; chưa có PR nên chưa ghi vào mô tả PR. Playwright (P2) không làm. Không dependency mới.
