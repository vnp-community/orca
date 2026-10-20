---
cr_id: CR-OR-020
solution_id: FE-OR-SOL-001
product: orca
layer: frontend
status: draft
priority: P0
effort: 6–8 ngày
depends_on: BE-OR-SOL-001 (RPC, mô hình)
created: 2026-10-20
---

# FE-OR-SOL-001 — Tab "Outcome review": khung, thẻ kiểm soát, trạng thái

## 1. Phân tích

- `RequestDetailPane.tsx` (379 dòng) dùng `Tabs` với `overview, analysis, plan, execution, history, related`; một số tab hiện có điều kiện; điều hướng tab có thể đến từ `nextAction.tab`. `request-stage-timeline-model.ts` 299 dòng. Cả hai sát ngưỡng tách file.
- Hook dữ liệu ở `src/renderer/src/hooks/` (`useExecutionSummary.ts`, `useRequestFlow.ts`); parser dây ở `src/shared/request-wire-parsers.ts` (bẫy gấp loại).
- `NextActionBar` dùng token sẵn có (`text-destructive`, `text-primary`, `text-muted-foreground`, `--status-success`).
- ĐỌC TRƯỚC: `next-action-rules`, `request-action-rules.ts`, cách cờ project đến frontend, `useRequestFlow`.

## 2. Thiết kế

### 2.1 Điểm vào
- Tab `outcome-review`, nhãn "Outcome review", **ngay sau Execution**; hiện khi cờ `outcome_review ≠ off` ∧ Request có Plan (`awaiting_plan_approval|executing|completed`). Tách `RequestDetailTabs.tsx` trước khi thêm.
- Dòng thời gian: hàng "Outcome review" (`none|design|delivered…` + `stale|approved`); tách mô hình dòng thời gian thành hai file trước.
- `NextActionBar`: "Review the design" (pha `design`), "Generate after-code review" (Execution xong chưa có `delivered`), `needs_you` khi còn quyết định mở.

### 2.2 Bố cục (màn đầu gọn)
`OutcomeReviewHeader` (đối tượng, chip KG snapshot hoặc "KG unavailable", trạng thái, Regenerate…) → `OutcomeDesignControl` (hai thẻ Architecture/Flow: pha `design` hiện số thiết kế, pha `delivered` hiện số kiểm soát; nhãn "Agent reported"; `designChecked=false` → Unknown) → thẻ kết luận (chỉ `delivered`) → "Needs your decision" (chỉ `delivered`) → tab con (SOL-002) → Release readiness và Agent run (chỉ `delivered`) → thanh quyết định (SOL-003). Dải "Design not reviewed" khi `reviewedRatio` thấp (AM-3).

### 2.3 Trạng thái
Chưa sinh (Estimate: cache, context run, số hình, "chưa có số liệu" khi chưa đo) · Pha `design` (Execution chưa xong: dải thông tin, không có quyết định) · Execution xong chưa có `delivered` (Estimate + nút chính) · Đang sinh (khung xương, `aria-live=polite`) · Cũ (`stale`) · KG vắng · Lỗi từng phần ("Drawing failed, showing the table") · Lỗi toàn bộ (mã `REQUEST_REVIEW_*` → i18n) · Cờ tắt/thiếu quyền (ẩn tab).

### 2.4 Dữ liệu
`shared/outcome-review-types.ts` (kiểu `OutcomeReview`, `ReviewItem` có `source`, `reviewState`, `advisory`), `outcome-review-wire-parsers.ts` (không gấp loại, enum lạ giữ thô). `hooks/useOutcomeReview.ts`: `Get`, `Estimate` khi chưa có, theo dõi push `request.review.changed|progress`, `generate`, `refresh`. Mở lại tab chỉ `Get` (không sinh).

### 2.5 Chữ
Bảng chữ (xem CR-OR-020 2.4) trong `outcome-review-copy.ts`; test không chứa từ cấm ("safe to release", "verified", "requirements met").

### 2.6 Cấu trúc file
`outcome-review/OutcomeReviewTab.tsx`, `OutcomeReviewHeader.tsx`, `OutcomeDesignControl.tsx`, `OutcomeVerdictCards.tsx`, `OutcomeReleaseReadiness.tsx`, `OutcomeAgentRunFacts.tsx`, `OutcomeReviewStates.tsx`, `outcome-review-view-model.ts`, `outcome-review-copy.ts` (+ test).

## 3. Kiểm thử
View model (mọi tổ hợp trạng thái/pha/enum lạ), copy, parser, RTL cho từng trạng thái (focus, `aria-live`), locale, hai render target.

## 4. Rủi ro
`next-action-rules` có thể cần đổi kiểu `NextAction`; cờ project đến frontend chưa xác minh (màn hình cài đặt engine chưa có); bản dịch es/ja/ko/zh còn nợ từ đợt trước (192 khoá) — dùng rơi về tiếng Anh đang có.
