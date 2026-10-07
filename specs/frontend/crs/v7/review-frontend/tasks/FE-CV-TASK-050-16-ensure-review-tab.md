# FE-CV-TASK-050-16: `ensureReviewTab`

**From Solution:** [FE-CV-SOL-050-review-tab-wiring](../solutions/FE-CV-SOL-050-review-tab-wiring.md) mục 4.3
**Priority:** P0
**Area:** frontend / renderer lib
**File:** `lib/ensure-review-tab.ts` (mới), tests `ensure-review-tab.test.ts`, `ensure-review-tab-behavior.test.ts`
**Depends on:** FE-CV-TASK-050-15, FE-CV-TASK-050-10, FE-CV-TASK-050-09
**Status:** [x] DONE

## Context

- Mẫu `lib/ensure-simulator-tab.ts` (một tab/worktree; `activateTab`, `focusGroup`, `setActiveTabType`; `rightSplit` qua `findReusableRightSplitGroupId`); tests mẫu `ensure-simulator-tab*.test.ts`.

## Việc cần làm

1. `getReviewTabForWorktree`, `ensureReviewTab(worktreeId, {targetGroupId?, placement?, surfacePane?})` trả `tabId | null`.
2. `null` khi: không có nhóm đích; support `disabled`/`unsupported`; selector `unsupported`. `unknown` thì cho mở.
3. Tạo tab `contentType:'review'`, `entityId = worktreeId`; không đăng ký phím toàn cục.

## Kiểm thử

- Tạo; tái dùng; `rightSplit`; support các trạng thái; selector không hợp lệ.

## Tiêu chí hoàn thành

- [ ] Không tạo trùng; hành vi khớp mẫu simulator.

## Rủi ro

- Điểm vào (SOL-061, nhóm B) gọi hàm này: giữ chữ ký ổn định.
