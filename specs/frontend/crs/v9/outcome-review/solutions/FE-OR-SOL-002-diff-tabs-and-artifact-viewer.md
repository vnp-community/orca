---
cr_id: CR-OR-021
solution_id: FE-OR-SOL-002
product: orca
layer: frontend
status: draft
priority: P0
effort: 8–10 ngày
depends_on: FE-OR-SOL-001, BE-OR-SOL-003 (GetReviewArtifact)
created: 2026-10-20
---

# FE-OR-SOL-002 — Tab diff mức hệ thống và trình xem artifact an toàn

## 1. Phân tích
- Mockup 2026-10-20: bảy tab con; Architecture/Flow đầu; công tắc `Design / After code / Side by side`; hàng diff `+ ~ − ∅`, chấm D/B/H, chip bằng chứng, cờ.
- Hình là HTML/SVG do LLM sinh: **không tin cậy**. Iframe `sandbox` không `allow-scripts`/`allow-same-origin` ⇒ trang chủ không đọc được chiều cao ⇒ dùng `view_box` do máy chủ trích; **huy hiệu "n components changed" không bấm được trong iframe ⇒ drill-down phải nằm ngoài khung** (danh sách "Inside containers").
- Frontend có mẫu lọc SVG ở nơi khác (`MermaidBlock` + DOMPurify) và bộ lọc nguồn Mermaid ở `review-report-diagram-guard.ts`; đều cho Mermaid, không cho HTML tuỳ ý.
- Màu: skill dùng vai trò màu; trình xem điền token của Orca (kết quả spike BE-OR-S2). Chỉ dùng token trong `main.css`.

## 2. Thiết kế

### 2.1 Tab con
Architecture (mặc định), Flow, Behavior, Data, Contracts, Requirements, Config. Số `unplanned` hiển thị trên Architecture/Flow. Pha `design`: chỉ phần Design; Contracts/Config "appear after execution completes"; Requirements = yêu cầu áp dụng ("Planned"). Pha `delivered`: mặc định `After code`.

### 2.2 Công tắc chế độ
`Design` | `After code` | `Side by side` (hai cột ở ≥ 900px, xếp dọc dưới đó). Trước khi Execution xong: `After code`/`Side by side` mờ kèm "Available when execution completes". Đã xong nhưng chưa có `delivered`: "Generate the after-code review first". Thiếu KG: `Design` mờ kèm "Design not available".

### 2.3 Hàng diff
Ký hiệu `+`/`~`/`−`/`∅` (không chỉ màu), tiêu đề, chi tiết, chip bằng chứng, cờ ("Deviates from design", "Missing from code", "Not in design" chỉ cho `source=code`, "In SAD text only" cho `source=sad-text`, "Advisory" khi dựa trên phần tử chưa duyệt), ba chấm D/B/H có `aria-label`. Chip `kg` hiển thị id + snapshot + nút sao chép (không liên kết); chip `source` hiển thị tài liệu/mục; chip `file` mở diff tệp (theo tệp, không theo dòng). Ô "đã xem" lưu phía client theo `reviewId` (try/catch; Q1).

### 2.4 `ReviewArtifactFrame`
`GetReviewArtifact(section, variant)` khi tab con mở; `if_digest` → `not_modified`. Dựng `srcdoc` = HTML đã lưu + khối `<style>:root{…}</style>` (token đang áp dụng) + `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:">` chèn đầu `<head>`. `<iframe sandbox="" referrerpolicy="no-referrer" loading="lazy" title="…">`; `aspect-ratio` từ `view_box`, cao tối đa 70vh; "Open larger" bằng hộp thoại shadcn. Đổi sáng/tối dựng lại `srcdoc` không gọi mạng. Dải thông tin: loại, `skill_version`, revision, thời điểm, `origin`; "Regenerate…", "Revisions", "Compare" (hai hình + danh sách nút/cạnh thêm/bớt từ `model`). Hình `failed`/thiếu: bảng + "Drawing failed" + thử lại (khi đủ quyền). Dưới hình luôn có danh sách hàng diff.

### 2.5 Panel Regenerate
Phạm vi (một hình / đọc lại thiết kế (và code)), lý do bắt buộc, Estimate (số lượt, số hình, `lastActualCost` hoặc "chưa có số liệu"), xác nhận chi phí khi cần; lỗi `BUDGET_EXCEEDED`, `RUN_IN_PROGRESS`, `STALE_REVISION` qua i18n.

### 2.6 Cấu trúc file
`outcome-review/diff/OutcomeSystemDiff.tsx`, `OutcomeDiffRow.tsx`, `OutcomeEvidenceChip.tsx`, `OutcomeModeSwitch.tsx`, `OutcomeRequirementsTable.tsx`, `OutcomeContainerDrilldown.tsx`, `outcome-diff-view-model.ts`; `artifact/ReviewArtifactFrame.tsx`, `ReviewArtifactInfoBar.tsx`, `ReviewArtifactCompareDialog.tsx`, `RegeneratePanel.tsx`, `review-artifact-srcdoc.ts`, `review-artifact-theme-tokens.ts`; `hooks/useReviewArtifact.ts`.

## 3. Kiểm thử
`review-artifact-srcdoc` (CSP, token, thoát), view model hàng diff, so sánh mô hình; RTL (các trạng thái hình, công tắc theme, chế độ theo pha, bàn phím, thuộc tính `sandbox` chính xác); bộ HTML độc hại trong trình duyệt thật (Playwright): script, `onload`, `<meta refresh>`, `<base>`, form tự gửi, `@import` ⇒ không yêu cầu mạng, không đổi DOM trang chủ; đọc màn hình cho danh sách hàng thay hình.

## 4. Rủi ro
Phụ thuộc kết quả S2 (token lúc hiển thị); `sandbox`/CSP khác nhau giữa Electron và web; `view_box` giả định skill luôn có `viewBox`; `DiffViewer` chưa nhận dòng (chip `file` mở theo tệp).
