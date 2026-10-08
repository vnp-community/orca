# FE-CV-TASK-051-04: `ReviewSummaryBar`, `ReviewRiskChip`, `review-chip-filter`

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 1 (dòng 2, 4), 4.2
**Priority:** P0
**Area:** frontend / review-map
**File:** `components/review-map/ReviewSummaryBar.tsx`, `ReviewRiskChip.tsx`, `review-chip-filter.ts`, tests
**Depends on:** FE-CV-TASK-051-01, FE-CV-TASK-050-13
**Status:** [x] DONE (verified 2026-10-07: review-chip-filter.test, ReviewSummaryBar.test incl. ReviewRiskChip)

## Context

- §4.3 `ChangeOverlay`: `changedFiles`, `changedSymbols`, `affectedFlows`, `touchedTables`, `touchedContracts`, `uncoveredSymbols`, `violations: ViolationRef[]`, `risk`, `limits`.
- Số đếm khi mảng bị cắt: `limits.totalCounts` (`Record<string,number>`); `truncated` cờ.

## Việc cần làm

1. Bảy chip luôn hiển thị (kể cả 0, mờ/vô hiệu), `aria-pressed`; số = mảng hoặc `limits.totalCounts[...]` nếu `truncated`.
2. `reviewChipPredicate(overlay, chip)` thuần: `files`/`symbols`/`untested`/`violations` ⇒ tập khoá; `flows/tables/contracts` ⇒ chuyển lens, không lọc.
3. `ReviewRiskChip`: icon + chữ cho `LOW|MEDIUM|HIGH|CRITICAL`; `incomplete` ⇒ "Chưa đủ dữ liệu"; popover liệt kê `risk.reasons[]` (`messageKey`, `params` qua `translate()`; mã lạ hiển thị nguyên văn); không điểm đơn, không "an toàn".

## Kiểm thử

- Mỗi chip; truncated dùng totalCounts; `incomplete`; reason lạ; `aria-pressed`.

## Tiêu chí hoàn thành

- [ ] Một bộ lọc một lúc; copy không overclaim.

## Rủi ro

- Khoá `limits.totalCounts` chưa liệt kê: dùng khoá theo tên mảng, bỏ qua nếu vắng.
