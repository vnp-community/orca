# Phase 1 — Khung tab, kiểu, parser, hook, trạng thái (FE-OR-SOL-001)

**Phụ thuộc:** BE Phase 1 (RPC `Estimate/Generate/Get/List`) hoặc fake gateway · **Mở khoá:** Phase 2, 3
Trước khi sửa symbol đã có: `gitnexus_impact`; trước commit: `detect_changes()` (hoặc ghi rõ nếu công cụ không có).

## Tách file trước khi thêm

- [x] **FE-OR-T101** Tách danh sách tab ra `RequestDetailTabs.tsx`.
  Files: `components/request/RequestDetailPane.tsx` (379 dòng), `RequestDetailTabs.tsx`. Verify: test hiện có của `RequestDetailPane` xanh; file ≤ ngưỡng dòng.
  Kết quả (2026-10-21): `RequestDetailTabs.tsx` (172 dòng) tách khỏi `RequestDetailPane.tsx` (379 → 338 dòng); test `RequestDetailPane` hiện có và test mới `RequestDetailPane.outcome-review.test.tsx` xanh.
- [x] **FE-OR-T102** Tách `request-stage-timeline-model.ts` (299 dòng) thành mô hình luồng và hàng mở rộng.
  Verify: test timeline hiện có xanh.
  Kết quả (2026-10-21): Bước và chú thích tách sang `request-stage-timeline-steps.ts`, hàng review sang `request-stage-timeline-review.ts`; `request-stage-timeline-model.ts` còn 183 dòng; test timeline hiện có và `request-stage-timeline-review.test.ts` xanh.
- [x] **FE-OR-T103** ĐỌC TRƯỚC `next-action-rules`, `request-action-rules.ts`, `useRequestFlow`, cách cờ project đến frontend; ghi kết luận.
  Kết quả (2026-10-21): Đã đọc trước khi sửa. Kết luận: tab id `'outcome-review'` thêm vào `next-action-rules.ts` (`primaryTab`) chứ không rẽ nhánh theo loại Request; cờ project đến frontend qua `outcomeReviewEnabled` của `request.engineSettings` (`request-run-all.ts`, mặc định `false` khi gateway cũ không gửi) và `useOutcomeReviewEnabled` (cache 60 giây). Không có công cụ GitNexus nên không chạy `impact`.

## Kiểu và dữ liệu

- [x] **FE-OR-T110** `shared/outcome-review-types.ts`: `OutcomeReview`, `ReviewItem` (`origin`, `source: kg|sad-text|code`, `reviewState`, `advisory`, `containers`, `protocol`, `expected`), `DiagramRef` (`variant: design|delta`), `Verdict`.
  Verify: tsc không lỗi.
  Kết quả (2026-10-21): `shared/outcome-review-types.ts` (+ `outcome-review-enums.ts`); `tsc` không thêm lỗi mới (đường cơ sở 115 lỗi có từ trước, vẫn 115).
- [x] **FE-OR-T111** `shared/outcome-review-wire-parsers.ts`: không gấp loại, enum lạ giữ thô.
  Verify: test enum lạ → "Unknown"; không đụng `request-wire-parsers.ts`.
  Kết quả (2026-10-21): Parser tách thành `outcome-review-wire-readers.ts`, `-wire-parsers.ts`, `-response-parsers.ts`; enum lạ giữ thô và hiện "Unknown"; không đụng `request-wire-parsers.ts`. Có test `outcome-review-wire-parsers.test.ts`.
- [x] **FE-OR-T112** `hooks/useOutcomeReview.ts`: `Get`, `Estimate`, `generate`, `refresh`, push `request.review.changed|progress`; mở lại tab chỉ `Get`.
  Verify: test hook (mở lại không gọi `generate`).
  Kết quả (2026-10-21): `useOutcomeReview.ts`: `Get` → `Estimate` khi chưa có review, `generate`, `refresh`; nghe `review.changed|progress`, thăm dò 4 giây chỉ khi có lượt chạy và trang đang hiện. Test xác nhận mở lại tab chỉ gọi `Get`/`Runs`, không gọi `generate`.

## Giao diện

- [x] **FE-OR-T120** Tab `outcome-review` ngay sau Execution (thứ tự cố định), điều kiện hiện (cờ + có Plan).
  Verify: RTL; cờ tắt → không render, không có lưu lượng `request.review.*`.
  Kết quả (2026-10-21): Tab `outcome-review` ngay sau Execution (`RequestDetailTabs.tsx`); cờ tắt hoặc chưa có Plan → không render và không gọi `request.review.*` (`RequestDetailPane.outcome-review.test.tsx`).
- [x] **FE-OR-T121** Hàng "Outcome review" ở dòng thời gian; gợi ý `NextActionBar` ("Review the design", "Generate after-code review", `needs_you`).
  Verify: test mô hình timeline/next action.
  Kết quả (2026-10-21): Hàng "Outcome review" ở timeline và gợi ý `NextActionBar` (`next-action-review.ts`: review the design / generate after-code / decide); gợi ý mềm, không chặn việc khác, review đã duyệt không gợi ý gì. Test `next-action-review.test.ts`, `request-stage-timeline-review.test.ts`.
- [x] **FE-OR-T122** `OutcomeReviewHeader`, `OutcomeDesignControl` (pha `design` hiện số thiết kế; `delivered` hiện số kiểm soát; Unknown khi `designChecked=false`), thẻ kết luận, `OutcomeReleaseReadiness`, `OutcomeAgentRunFacts`.
  Verify: số liệu khớp mô hình; thẻ nhảy đúng tab con.
  Kết quả (2026-10-21): `OutcomeReviewHeader`, `OutcomeDesignControl`, `OutcomeVerdictCards`, `OutcomeReleaseReadiness`, `OutcomeAgentRunFacts`; số liệu lấy từ `outcome-review-view-model.ts` (hàm thuần, có test); `designChecked=false` → ô "Unknown", không hiện 0. **Lệch so với mô tả:** thẻ Requirements ghi "N of M" các tác vụ gắn với tiêu chí đã xong, không dùng chữ "đạt/sẵn sàng" vì Orca không biết kiểm tra nào chứng minh điều đó.
- [x] **FE-OR-T123** Trạng thái: chưa sinh (Estimate), pha `design`, Execution xong chưa có `delivered`, đang sinh (`aria-live=polite`), cũ, KG vắng, lỗi từng phần, lỗi toàn bộ, cờ tắt/thiếu quyền; dải "Design not reviewed" (AM-3).
  Files: `OutcomeReviewStates.tsx`. Verify: RTL từng trạng thái.
  Kết quả (2026-10-21): `OutcomeReviewStates.tsx` + `outcome-review-state-copy.ts`: chưa sinh (Estimate), pha `design`, Execution xong chưa có `delivered`, đang sinh (`aria-live=polite`), cũ, KG vắng, lượt chạy lỗi (vẫn hiện sự kiện đã đo), lỗi toàn bộ, cờ tắt/thiếu quyền, dải "Design not reviewed" (AM-3). Test RTL từng trạng thái trong `OutcomeReviewTab.test.tsx`.
- [x] **FE-OR-T124** `outcome-review-copy.ts` + test từ cấm; i18n 5 locale + `outcome-review-locale-coverage.test.ts`.
  Verify: test locale đạt; không có "safe to release".
  Kết quả (2026-10-21): `outcome-review-copy.ts` + test từ cấm (không có "safe to release"); 288 khoá i18n × 5 locale; es/ja/ko/zh dịch tay (không phải bản nháp tiếng Anh), giữ nguyên placeholder; `outcome-review-locale-coverage.test.ts` 8/8 xanh. **Chưa có:** người bản ngữ rà soát es/ja/ko/zh.

## Hoàn tất

- [~] **FE-OR-T199** vitest toàn thư mục `outcome-review` + typecheck/lint; hai render target; điều khiển bàn phím; ghi kết quả.
  Kết quả (2026-10-21): `vitest` thư mục `outcome-review`: 15 tệp, 170 test PASS; `tsc` 115 lỗi (bằng đường cơ sở, 0 lỗi mới từ phần này); `oxlint` sạch trên 92 tệp đã đổi/mới. **Không có:** kiểm tra bằng trình đọc màn hình thật (chỉ kiểm vai trò/nhãn ARIA bằng RTL); hai render target mới thử ở web (Playwright), chưa thử Electron.
