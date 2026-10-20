# Phase 2 — Tab diff và trình xem artifact (FE-OR-SOL-002)

**Phụ thuộc:** Phase 1; BE Phase 3 cho hình (bảng làm trước, hình sau) và spike BE-OR-S2

## Tab diff và chế độ

- [x] **FE-OR-T201** `OutcomeSystemDiff` + thanh tab con (Architecture mặc định, Flow, Behavior, Data, Contracts, Requirements, Config), số `unplanned` trên Architecture/Flow.
  Files: `outcome-review/diff/OutcomeSystemDiff.tsx`. Verify: RTL.
  Kết quả (2026-10-21): `OutcomeSystemDiff` + thanh tab con (Architecture mặc định), số `unplanned` trên Architecture/Flow, số mục chưa xem (`outcome-review-seen.ts`). Test `OutcomeSystemDiff.test.tsx`.
- [x] **FE-OR-T202** `OutcomeModeSwitch`: `Design | After code | Side by side` theo pha và KG (mờ kèm lý do đúng từng trường hợp); `Side by side` hai cột ≥ 900px, xếp dọc dưới đó.
  Verify: RTL cho pha `design`, Execution xong chưa có `delivered`, `delivered`, KG vắng.
  Kết quả (2026-10-21): `OutcomeModeSwitch`: mờ kèm đúng lý do (thiết kế vắng / Execution chưa xong / cần sinh trước); Side by side hai cột ≥ 900px, dưới đó xếp dọc.
- [x] **FE-OR-T203** `OutcomeDiffRow` (ký hiệu `+ ~ − ∅`, cờ, chấm D/B/H có `aria-label`), `OutcomeEvidenceChip` (`kg` có sao chép không liên kết; `source` hiển thị tài liệu/mục; `file` mở diff tệp).
  Verify: chip `kg` không có liên kết; nhãn "In SAD text only" / "Not in design" / "Advisory" đúng nguồn (AM-2, AM-3).
  Kết quả (2026-10-21): `OutcomeDiffRow` (`+ ~ − ∅`, cờ, chấm D/B/H có `aria-label`), `OutcomeEvidenceChip` (`kg` chỉ sao chép, không liên kết). **Giới hạn:** chip `file` mở thấu kính impact của review code, chưa mở diff riêng của tệp đó.
- [x] **FE-OR-T204** `outcome-diff-view-model.ts` (hàm thuần): lọc `same`, nhóm theo tab, đếm.
  Verify: bảng ca; ẩn hàng `unchanged`.
  Kết quả (2026-10-21): `diff/outcome-diff-view-model.ts` hàm thuần, có bảng ca; ẩn hàng `unchanged` mặc định.
- [x] **FE-OR-T205** Tab Requirements: pha `design` ("Planned"), pha `delivered` (trạng thái trước → sau; "Regression" gắn nhãn agent).
  Files: `diff/OutcomeRequirementsTable.tsx`. Verify: RTL.
  Kết quả (2026-10-21): `OutcomeRequirementsTable`: pha `design` = "Planned"; pha `delivered` = trước → sau, "Regression" gắn nhãn agent.
- [x] **FE-OR-T206** Contracts/Config: bảng sau code, "appear after execution completes" ở pha `design`; Config chỉ hiện tên biến.
  Kết quả (2026-10-21): Contracts/Config hiện sau code; ở pha `design` ghi "appear after execution completes"; Config chỉ hiện tên biến.

## Trình xem artifact

- [x] **FE-OR-T210** `review-artifact-theme-tokens.ts` (đọc token đang áp dụng) và `review-artifact-srcdoc.ts` (CSP meta + khối `:root`, hàm thuần).
  Verify: unit test chèn/thoát; không hex cứng trong mã mới.
  Kết quả (2026-10-21): `review-artifact-theme-tokens.ts` đọc token Orca đang áp (`--status-success`, `--risk-medium`, `--destructive`, `--primary` → `--rv-*`) và `review-artifact-srcdoc.ts` (làm sạch trong `<template>` trơ + CSP meta); không hex cứng.
- [x] **FE-OR-T211** `ReviewArtifactFrame`: iframe `sandbox=""` (không `allow-scripts`, không `allow-same-origin`), `aspect-ratio` theo `view_box`, cao ≤ 70vh, tên truy cập từ `title/desc`, `if_digest`.
  Verify: kiểm thuộc tính `sandbox` chính xác; đổi sáng/tối không gọi mạng.
  Kết quả (2026-10-21): `ReviewArtifactFrame`: `sandbox=""` (kiểm thuộc tính chính xác), `aspect-ratio` theo `view_box`, cao ≤ 70vh, tên truy cập từ `title/desc`, `ifDigest`; đổi sáng/tối không gọi mạng (`useDocumentThemeVersion`).
- [x] **FE-OR-T212** `ReviewArtifactInfoBar` (loại, `skill_version`, revision, thời điểm, `origin`), "Open larger" (Dialog), "Revisions", "Compare" (`ReviewArtifactCompareDialog`: hai hình + khác biệt mô hình tính xác định).
  Verify: RTL, bàn phím, Esc đóng.
  Kết quả (2026-10-21): `ReviewArtifactInfoBar`, "Open larger", "Revisions", "Compare" (`ReviewArtifactCompareDialog`: khác biệt tính xác định từ mô hình, không để agent mô tả).
- [x] **FE-OR-T213** `OutcomeContainerDrilldown`: danh sách "Inside containers" ngoài iframe với nút "Open in code review" (nối ở Phase 3).
  Verify: huy hiệu không nằm trong iframe.
  Kết quả (2026-10-21): `OutcomeContainerDrilldown` nằm ngoài iframe. **Giới hạn:** nút mở review code chưa thu hẹp được về đúng một container.
- [x] **FE-OR-T214** Hình `failed`/thiếu: bảng + "Drawing failed"/"No diagram for this change" + thử lại (đủ quyền).
  Verify: RTL.
  Kết quả (2026-10-21): Hình `failed`/thiếu → bảng + "Drawing failed"/"No diagram for this change" + thử lại khi đủ quyền.
- [x] **FE-OR-T215** `RegeneratePanel`: phạm vi, lý do bắt buộc, Estimate ("chưa có số liệu" khi chưa đo), xác nhận chi phí; lỗi `BUDGET_EXCEEDED|RUN_IN_PROGRESS|STALE_REVISION`; `useReviewArtifact`.
  Verify: RTL; Estimate không gọi `regenerate`.
  Kết quả (2026-10-21): `RegeneratePanel`: phạm vi, lý do bắt buộc, Estimate ("chưa có số liệu" khi chưa đo), xác nhận chi phí; lỗi `BUDGET_EXCEEDED|RUN_IN_PROGRESS|STALE_REVISION` hiện đúng; Estimate không gọi `regenerate`. Test `RegeneratePanel.test.tsx`.

## Bảo mật (trình duyệt thật)

- [x] **FE-OR-T220** Bộ HTML độc hại (script, `onload`, `<svg><script>`, `<foreignObject>`, `<meta refresh>`, `<base>`, form tự gửi, `<style>@import url()`, `javascript:`) nạp thẳng vào `ReviewArtifactFrame` (giả lập bộ kiểm backend bị bỏ qua).
  Verify: Playwright: không yêu cầu mạng, không đổi DOM trang chủ; chạy cả Electron lẫn web nếu có thể.
  Kết quả (2026-10-21): Playwright trên Chromium thật, `review-artifact-sandbox.web.e2e.ts`: **30/30 PASS** (31 giây) cho bộ HTML độc hại (script, `onload`, `<svg><script>`, `<foreignObject>`, `<meta refresh>`, `<base>`, form tự gửi, `@import url()`, `javascript:`) — mỗi ca chạy cả khi bỏ bước làm sạch: không yêu cầu mạng, không đổi DOM trang chủ; có ca đối chứng (iframe cho chạy script, không CSP) bị bắt đúng. **Chưa chạy** trong Electron.

## Hoàn tất

- [~] **FE-OR-T299** vitest + typecheck/lint; kiểm đọc màn hình cho danh sách hàng thay hình; ghi kết quả.
  Kết quả (2026-10-21): Cùng số liệu T199 (vitest xanh, tsc 115, oxlint sạch). **Không có:** kiểm tra bằng trình đọc màn hình thật cho danh sách hàng thay hình (chỉ kiểm bảng thay thế và nhãn ARIA bằng RTL).
