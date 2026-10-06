# FE-CV-TASK-061-05: Ba hành động Cmd+K (Review, kiến trúc, ERD)

**From Solution:** [FE-CV-SOL-061](../solutions/FE-CV-SOL-061-review-entry-points.md) mục 2.5
**Priority:** P1
**Area:** frontend / renderer cmd-j
**File:** `frontend/src/renderer/src/components/cmd-j/quick-actions.ts`, `quick-action-context.ts`, `quick-action-context.test.ts`, `components/WorktreeJumpPalette.tsx` (sửa) + test
**Depends on:** FE-CV-TASK-061-02; lens đã phát hành (`reviewLensAvailable`)
**Status:** [ ] TODO

## Context

- Đã xác minh `CmdJUnavailableReason` 4 giá trị (`quick-action-context.ts:6-10`). Chạy `gitnexus_impact` trên `getCmdJQuickActions`, `buildCmdJQuickActionContext`, `getUnavailableQuickActionMessage` trước khi sửa.
- Chuỗi trong `createLocalizedCatalog`, không `translate()` cấp module.

## Việc cần làm

1. Thêm `review-changes`, `open-architecture-map`, `open-erd` với `verbKeywords` qua `translate`.
2. Context: `codeIntelEnabled`, `reviewLensAvailable`, `openReviewChanges`; `isAvailable` = availability workspace sẵn có ∧ cờ ∧ lens khả dụng.
3. `CmdJUnavailableReason` thêm `'code-intel-disabled'` + thông điệp.
4. `WorktreeJumpPalette.tsx`: `openReviewChangesAction` bằng `useCallback` → `openReviewFromEntryPoint(…,'cmd-k',{lens})` rồi đóng modal.

## Kiểm thử

- `isAvailable` theo từng lý do (loading, ssh-disconnected, cờ tắt, lens chưa có); thông điệp lý do; chạy được bằng bàn phím; `no-top-level-translate.test.ts` xanh.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/cmd-j src/renderer/src/i18n/no-top-level-translate`.

## Tiêu chí hoàn thành

- [ ] Tiêu chí Cmd+K của SOL-061 mục 5.
- [ ] Test hiện có của `cmd-j/` xanh.

## Rủi ro

- Danh mục Cmd+J vốn curated (câu hỏi mở 3).
