# Phase 1 — Khung tab, kiểu, parser, hook, trạng thái (FE-OR-SOL-001)

**Phụ thuộc:** BE Phase 1 (RPC `Estimate/Generate/Get/List`) hoặc fake gateway · **Mở khoá:** Phase 2, 3
Trước khi sửa symbol đã có: `gitnexus_impact`; trước commit: `detect_changes()` (hoặc ghi rõ nếu công cụ không có).

## Tách file trước khi thêm

- [ ] **FE-OR-T101** Tách danh sách tab ra `RequestDetailTabs.tsx`.
  Files: `components/request/RequestDetailPane.tsx` (379 dòng), `RequestDetailTabs.tsx`. Verify: test hiện có của `RequestDetailPane` xanh; file ≤ ngưỡng dòng.
- [ ] **FE-OR-T102** Tách `request-stage-timeline-model.ts` (299 dòng) thành mô hình luồng và hàng mở rộng.
  Verify: test timeline hiện có xanh.
- [ ] **FE-OR-T103** ĐỌC TRƯỚC `next-action-rules`, `request-action-rules.ts`, `useRequestFlow`, cách cờ project đến frontend; ghi kết luận.

## Kiểu và dữ liệu

- [ ] **FE-OR-T110** `shared/outcome-review-types.ts`: `OutcomeReview`, `ReviewItem` (`origin`, `source: kg|sad-text|code`, `reviewState`, `advisory`, `containers`, `protocol`, `expected`), `DiagramRef` (`variant: design|delta`), `Verdict`.
  Verify: tsc không lỗi.
- [ ] **FE-OR-T111** `shared/outcome-review-wire-parsers.ts`: không gấp loại, enum lạ giữ thô.
  Verify: test enum lạ → "Unknown"; không đụng `request-wire-parsers.ts`.
- [ ] **FE-OR-T112** `hooks/useOutcomeReview.ts`: `Get`, `Estimate`, `generate`, `refresh`, push `request.review.changed|progress`; mở lại tab chỉ `Get`.
  Verify: test hook (mở lại không gọi `generate`).

## Giao diện

- [ ] **FE-OR-T120** Tab `outcome-review` ngay sau Execution (thứ tự cố định), điều kiện hiện (cờ + có Plan).
  Verify: RTL; cờ tắt → không render, không có lưu lượng `request.review.*`.
- [ ] **FE-OR-T121** Hàng "Outcome review" ở dòng thời gian; gợi ý `NextActionBar` ("Review the design", "Generate after-code review", `needs_you`).
  Verify: test mô hình timeline/next action.
- [ ] **FE-OR-T122** `OutcomeReviewHeader`, `OutcomeDesignControl` (pha `design` hiện số thiết kế; `delivered` hiện số kiểm soát; Unknown khi `designChecked=false`), thẻ kết luận, `OutcomeReleaseReadiness`, `OutcomeAgentRunFacts`.
  Verify: số liệu khớp mô hình; thẻ nhảy đúng tab con.
- [ ] **FE-OR-T123** Trạng thái: chưa sinh (Estimate), pha `design`, Execution xong chưa có `delivered`, đang sinh (`aria-live=polite`), cũ, KG vắng, lỗi từng phần, lỗi toàn bộ, cờ tắt/thiếu quyền; dải "Design not reviewed" (AM-3).
  Files: `OutcomeReviewStates.tsx`. Verify: RTL từng trạng thái.
- [ ] **FE-OR-T124** `outcome-review-copy.ts` + test từ cấm; i18n 5 locale + `outcome-review-locale-coverage.test.ts`.
  Verify: test locale đạt; không có "safe to release".

## Hoàn tất

- [ ] **FE-OR-T199** vitest toàn thư mục `outcome-review` + typecheck/lint; hai render target; điều khiển bàn phím; ghi kết quả.
