# FE-CV-TASK-087-04: Mô hình thuần: copy kết luận, độ cũ, phạm vi chạy, trạng thái hiển thị, chọn profile

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 2.5, 2.6, 2.7
**Priority:** P0
**Area:** frontend / pure functions
**File:** `frontend/src/renderer/src/components/review-map/quality/quality-gate-copy.ts`, `quality-stale-model.ts`, `quality-run-scope-model.ts`, `quality-view-state.ts`, `quality-profile-selection.ts` (mới) và `*.test.ts`
**Depends on:** 087-01 (kiểu); không cần backend
**Status:** [x] DONE

## Context

- STYLEGUIDE: "UI copy must not overclaim"; hợp đồng §4.7: `unknown` = "Chưa đủ dữ liệu để kết luận", cấm "an toàn", "đã đáp ứng", "AI đã review", không điểm đơn; `mode:'block'` vẫn chỉ cảnh báo (O9).
- `QualityGate.profile` = `"<name>@<scope>/v<version>"`; `RunnableProfile.scopes` giới hạn phạm vi; `ReviewScope` (branch|range|hostedReview) thuộc CR-051 (chưa có code).
- HEAD hiện tại: `gitBranchCompareSummaryByWorktree[worktreeId].headOid` (có trong store, đã grep).

## Việc cần làm

1. `quality-gate-copy.ts`: `verdictHeadline(verdict, profile)`, `modeNotice(mode)`, `unknownReason(gate)`; chuỗi dựng lúc gọi qua `translate()`.
2. `quality-stale-model.ts`: `isGateStale({gate, runs, currentHead})` = `basedOn.stale` hoặc `headCommit` của run ≠ HEAD; `indexDiffersFromHead` chỉ thông tin.
3. `quality-run-scope-model.ts`: `ReviewScope` → `{scope, base?}`; lọc theo `scopes` của profile; mặc định `changed`.
4. `quality-view-state.ts`: bảng ưu tiên 2.7 → `QualityViewState` có `kind` + khoá copy.
5. `quality-profile-selection.ts`: `splitProfileRef`, chọn mặc định (cổng → cấu hình → mục `ready` đầu).

## Kiểm thử

Bốn verdict × mode; quét chuỗi mặc định không chứa `an toàn|sạch|đã đáp ứng`; stale ba nhánh; scope mọi `ReviewScope`; ưu tiên view-state; `splitProfileRef` với chuỗi lạ. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/quality-`.

## Tiêu chí hoàn thành

- [ ] Test quét copy xanh; không phụ thuộc DOM.
- [ ] `unknown` không dùng từ của `pass`.

## Rủi ro

- Tên `ReviewScope` có thể khác ở CR-051; điều chỉnh khi 051 chốt.
