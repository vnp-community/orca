# Phase 2 — Tab diff và trình xem artifact (FE-OR-SOL-002)

**Phụ thuộc:** Phase 1; BE Phase 3 cho hình (bảng làm trước, hình sau) và spike BE-OR-S2

## Tab diff và chế độ

- [ ] **FE-OR-T201** `OutcomeSystemDiff` + thanh tab con (Architecture mặc định, Flow, Behavior, Data, Contracts, Requirements, Config), số `unplanned` trên Architecture/Flow.
  Files: `outcome-review/diff/OutcomeSystemDiff.tsx`. Verify: RTL.
- [ ] **FE-OR-T202** `OutcomeModeSwitch`: `Design | After code | Side by side` theo pha và KG (mờ kèm lý do đúng từng trường hợp); `Side by side` hai cột ≥ 900px, xếp dọc dưới đó.
  Verify: RTL cho pha `design`, Execution xong chưa có `delivered`, `delivered`, KG vắng.
- [ ] **FE-OR-T203** `OutcomeDiffRow` (ký hiệu `+ ~ − ∅`, cờ, chấm D/B/H có `aria-label`), `OutcomeEvidenceChip` (`kg` có sao chép không liên kết; `source` hiển thị tài liệu/mục; `file` mở diff tệp).
  Verify: chip `kg` không có liên kết; nhãn "In SAD text only" / "Not in design" / "Advisory" đúng nguồn (AM-2, AM-3).
- [ ] **FE-OR-T204** `outcome-diff-view-model.ts` (hàm thuần): lọc `same`, nhóm theo tab, đếm.
  Verify: bảng ca; ẩn hàng `unchanged`.
- [ ] **FE-OR-T205** Tab Requirements: pha `design` ("Planned"), pha `delivered` (trạng thái trước → sau; "Regression" gắn nhãn agent).
  Files: `diff/OutcomeRequirementsTable.tsx`. Verify: RTL.
- [ ] **FE-OR-T206** Contracts/Config: bảng sau code, "appear after execution completes" ở pha `design`; Config chỉ hiện tên biến.

## Trình xem artifact

- [ ] **FE-OR-T210** `review-artifact-theme-tokens.ts` (đọc token đang áp dụng) và `review-artifact-srcdoc.ts` (CSP meta + khối `:root`, hàm thuần).
  Verify: unit test chèn/thoát; không hex cứng trong mã mới.
- [ ] **FE-OR-T211** `ReviewArtifactFrame`: iframe `sandbox=""` (không `allow-scripts`, không `allow-same-origin`), `aspect-ratio` theo `view_box`, cao ≤ 70vh, tên truy cập từ `title/desc`, `if_digest`.
  Verify: kiểm thuộc tính `sandbox` chính xác; đổi sáng/tối không gọi mạng.
- [ ] **FE-OR-T212** `ReviewArtifactInfoBar` (loại, `skill_version`, revision, thời điểm, `origin`), "Open larger" (Dialog), "Revisions", "Compare" (`ReviewArtifactCompareDialog`: hai hình + khác biệt mô hình tính xác định).
  Verify: RTL, bàn phím, Esc đóng.
- [ ] **FE-OR-T213** `OutcomeContainerDrilldown`: danh sách "Inside containers" ngoài iframe với nút "Open in code review" (nối ở Phase 3).
  Verify: huy hiệu không nằm trong iframe.
- [ ] **FE-OR-T214** Hình `failed`/thiếu: bảng + "Drawing failed"/"No diagram for this change" + thử lại (đủ quyền).
  Verify: RTL.
- [ ] **FE-OR-T215** `RegeneratePanel`: phạm vi, lý do bắt buộc, Estimate ("chưa có số liệu" khi chưa đo), xác nhận chi phí; lỗi `BUDGET_EXCEEDED|RUN_IN_PROGRESS|STALE_REVISION`; `useReviewArtifact`.
  Verify: RTL; Estimate không gọi `regenerate`.

## Bảo mật (trình duyệt thật)

- [ ] **FE-OR-T220** Bộ HTML độc hại (script, `onload`, `<svg><script>`, `<foreignObject>`, `<meta refresh>`, `<base>`, form tự gửi, `<style>@import url()`, `javascript:`) nạp thẳng vào `ReviewArtifactFrame` (giả lập bộ kiểm backend bị bỏ qua).
  Verify: Playwright: không yêu cầu mạng, không đổi DOM trang chủ; chạy cả Electron lẫn web nếu có thể.

## Hoàn tất

- [ ] **FE-OR-T299** vitest + typecheck/lint; kiểm đọc màn hình cho danh sách hàng thay hình; ghi kết quả.
