# FE-CV-TASK-051-02: `review-view-state` (13 trạng thái) và `usePerceivedLoadingStage`

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 4.4
**Priority:** P0
**Area:** frontend / review-map + hooks
**File:** `components/review-map/review-view-state.ts`, `ReviewLoadingStage.tsx`, `hooks/usePerceivedLoadingStage.ts` (mới), tests
**Depends on:** FE-CV-TASK-051-01, FE-CV-TASK-050-14
**Status:** [ ] TODO

## Context

- STYLEGUIDE "Match in-flight feedback to perceived duration": không đổi < 100 ms; vô hiệu hoá 100 ms–1 s; spinner ≥ 1 s; nhãn giai đoạn ≥ 3 s; từ xa 200 ms.
- Kind lỗi UI-API §2.3: `no-binding`, `tool-unavailable`, `repo-not-registered`, `path-not-allowed`, `index-missing`, `offline`, `forbidden`, `rate-limited`, `timeout`, `too-large`, `tool-failed`, `unknown`.

## Việc cần làm

1. Hàm thuần `computeReviewViewState({support, scopeResult, indexStatus, overlayQuery, connections})` theo bảng 13 dòng (SOL 4.4); trả `{screen|banner[]}`.
2. `usePerceivedLoadingStage(isPending, {remote})` trả `idle|busy|dimmed|spinner|stages`; `remote` = `environmentId !== null`.
3. `ReviewLoadingStage`: giữ chỗ cố định, nhãn giai đoạn do caller truyền.

## Kiểm thử

- `review-view-state.test.ts`: mỗi dòng và ưu tiên (ví dụ `no-binding` thắng `index-missing`; offline có cache ≠ không cache; `emptyReason:'unborn-head'`).
- `usePerceivedLoadingStage.test.ts`: ngưỡng, từ xa 200 ms, huỷ trước ngưỡng.

## Tiêu chí hoàn thành

- [ ] Bảng 13 dòng được phủ; không toast.

## Rủi ro

- Ngưỡng thời gian là của STYLEGUIDE, chưa đo trên SSH thật.
