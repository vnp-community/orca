# FE-CV-TASK-061-02: `openReviewFromEntryPoint` và `useReviewEntryAvailability`

**From Solution:** [FE-CV-SOL-061](../solutions/FE-CV-SOL-061-review-entry-points.md) mục 2.1
**Priority:** P0
**Area:** frontend / renderer lib + hooks
**File:** `frontend/src/renderer/src/components/review-map/entry/open-review-entry.ts`, `useReviewEntryAvailability.ts`, `ReviewEntryButton.tsx` (mới) + test
**Depends on:** FE-CV-SOL-050-review-tab-wiring (`ensureReviewTab`); FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelSupport`); FE-CV-SOL-051-review-workspace-shell (phạm vi mặc định O7, `setReviewLens`)
**Status:** [x] DONE

## Context

- Đã xác minh `lib/worktree-activation.ts` tồn tại (`activateAndRevealWorktree`).
- Cờ: `effective.codeIntelEnabled` từ `settings.get` (hợp đồng §6); `unknown` ⇒ chưa hiển thị.

## Việc cần làm

1. `useReviewEntryAvailability(worktreeId)` → `{visible, reason?}` (cờ + worktree thuộc repo git, không phải folder workspace).
2. `openReviewFromEntryPoint(worktreeId, source, options?)`: kiểm cờ → `activateAndRevealWorktree` → kích hoạt tab `review` có sẵn hoặc `ensureReviewTab` → đặt phạm vi/lens/lọc; trả `boolean`; không gọi gì khi cờ tắt.
3. `ReviewEntryButton` (`ScanSearch`, `Tooltip`, `aria-label`; `stopPropagation`).

## Kiểm thử

- Dedupe tab; thứ tự activate→ensure; cờ tắt bỏ qua; `unknown` không hiển thị; folder workspace không hiển thị.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/entry`.

## Tiêu chí hoàn thành

- [ ] Một hàm mở duy nhất cho 5 `source`.
- [ ] Không yêu cầu `codeIntel.*` khi cờ tắt.

## Rủi ro

- Phụ thuộc `ensureReviewTab` của SOL-050; thiếu thì không điểm vào nào chạy.
