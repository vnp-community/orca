# FE-CV-TASK-057-06: `ErdTableDetail`, i18n 5 locale và e2e web lens ERD

**From Solution:** [FE-CV-SOL-057](../solutions/FE-CV-SOL-057-erd-lens.md) mục 2.6, 2.8, 6
**Priority:** P1
**Area:** frontend / renderer components + i18n + tests
**File:** `frontend/src/renderer/src/components/review-map/erd/ErdTableDetail.tsx` (mới) + test; `i18n/locales/{en,es,ja,ko,zh}.json`; `i18n/code-intel-locale-coverage.test.ts` (sửa: thêm `KEYS`); `tests/e2e/code-intel-web/lenses.web.e2e.ts` (phần ERD; file do FE-CV-TASK-073-05 tạo)
**Depends on:** FE-CV-TASK-057-05; FE-CV-SOL-053-impact-lens-and-symbol-detail (`SymbolDetailPanel`, mở symbol/diff); FE-CV-TASK-073-02/073-03 (fake backend + Playwright) cho e2e
**Status:** [x] DONE (verified 2026-10-08)

## Context

- `accessedBy` lấy từ quét tên bảng (confidence) nên mọi nhãn "gợi ý, có thể sai"; không viết "an toàn" (quy tắc hiển thị §4.7 hợp đồng và STYLEGUIDE "không overclaim").
- Nhảy dòng trong diff phụ thuộc `pendingDiffReveal` của SOL-053; chưa có thì mở đúng tệp và ghi rõ giới hạn.

## Việc cần làm

1. `ErdTableDetail` cắm vào `SymbolDetailPanel`: bảng cột (`ui/table`), index (`emulatesPartialUnique`), `checks`, RLS (`rlsState`, chỉ tên + biểu thức đã mask), quan hệ ra/vào, "Mã đọc/ghi bảng này" (lọc Tất cả|Ghi|Đọc, chấm "đã đổi", "Mở"/"Xem diff"/"Xem migration diff"), cảnh báo liên đới.
2. Dịch toàn bộ khoá `auto.components.reviewMap.Erd*` sang es/ja/ko/zh (không trùng văn bản `en`); thêm vào mảng `KEYS` của test phủ khoá.
3. e2e (Chromium, WS giả từ fake backend G4): mở Review → lens ERD → thấy bảng từ tệp vàng, chọn bảng, chuyển service qua nút ma, cột có `+`/`~`/`−`; nhãn chứa HTML hiển thị như văn bản; `backend.streamCount()` không đổi khi cờ tắt.
4. Test rò rỉ: khoá `erd*` bị dọn khi xoá worktree (nếu chưa phủ ở 057-04).

## Kiểm thử

- Testing Library: `ErdTableDetail` (nhóm read/write; nút "Xem diff" bị khoá khi symbol không đổi; consumers rỗng không viết "không ai dùng"); `maskSensitiveText` áp lên `defaultExpr`.
- `pnpm --filter orca-frontend test -- src/renderer/src/i18n/code-intel-locale-coverage src/renderer/src/components/review-map/erd`; e2e: `pnpm run test:e2e:code-intel-web -- lenses.web.e2e.ts` (script do 073-03 thêm; chưa chạy).

## Tiêu chí hoàn thành

- [ ] 5 locale đủ khoá; `no-top-level-translate.test.ts` xanh.
- [ ] Chi tiết bảng không hiển thị giá trị chưa mask.
- [ ] e2e ERD xanh trên fake backend (khi 073-03 sẵn sàng).

## Rủi ro

- Phụ thuộc SOL-053 cho mở symbol/diff; nếu chưa có, nút "Mở" tạm ẩn (không lỗi).

## Ghi chú triển khai (2026-10-07)

- Đã làm: `ErdTableDetail` (cột/index/check/RLS/quan hệ/accessor, lọc Tất cả|Ghi|Đọc, "Mở"/"Xem diff"/"Xem migration diff", cảnh báo liên đới nhãn gợi ý), khoá `auto.components.reviewMap.Erd*` đủ 5 locale.
- Sai lệch: kiểm phủ khoá nằm ở test mới `i18n/review-erd-storage-locale-coverage.test.ts` (không sửa `code-intel-locale-coverage.test.ts` dùng chung). "Mở" gọi `onSelectSymbol` của khung; "Xem diff" dùng `onOpenDiff(path, line)` của khung (chưa có `pendingDiffReveal` của 053 nên nhảy dòng tuỳ khung). Test rò rỉ `erd*`: đã phủ bởi test rò rỉ của `review-ui`.
- Còn thiếu: e2e web (cần 073-02/073-03).
