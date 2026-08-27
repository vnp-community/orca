# BUG-FE-BIGFILE-004 — `SourceControl.tsx` (8,370 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-004](./solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md)
**Module:** `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

8,370 dòng — chỉ có `/* eslint-disable max-lines */` không kèm giải thích (khác
với phần lớn file khác trong nhóm Critical, xem `BUG-FE-HLD-006` §"disable
không kèm giải thích").

File chứa **nhiều component top-level độc lập**, không chỉ 1 component chính:

```
6556  export function CommitArea({...})
7095  export function CompareSummary({...})
7186  export function CompareSummaryToolbarButton({...})
7534  export function ConflictSummaryCard({...})
7637  export function OperationBanner({...})
7684  export function TooManyChangesBanner({ limit }: ...)
8297  export function ActionButton({...})
```

Cộng thêm nhiều pure helper function xuất khẩu ở đầu file (dòng 354–774):
`resolveSourceControlBaseRef`, `resolveSourceControlCompareBaseRef`,
`shouldClearBranchCompareForMissingBase`, `resolveSourceControlPickerBaseRef`,
`normalizeSourceControlViewMode`, `readCommitDraftForWorktree`,
`writeCommitDraftForWorktree`, `shouldRenderCommitArea`,
`pickDefaultSourceControlAgent`, `refreshSourceControlAfterRemoteAction`,
`clearRemoteActionErrorsForCompletedConflictOperations` — các hàm này không
phụ thuộc component nào, dễ tách và có khả năng đã có test riêng (đáng kiểm
tra file test cùng thư mục trước khi di chuyển).

24 `useEffect` trong toàn file — mật độ effect cao, nên tách theo từng
component trước khi động vào state.

## Hậu quả

- 7 component export riêng biệt (`CommitArea`, `CompareSummary`,
  `CompareSummaryToolbarButton`, `ConflictSummaryCard`, `OperationBanner`,
  `TooManyChangesBanner`, `ActionButton`) đã đủ độc lập để tách file ngay —
  nhưng đang cùng sống trong 1 file khiến việc tìm code liên quan đến 1 tính
  năng cụ thể (vd: "conflict summary") phải kéo qua toàn bộ 8,370 dòng.
- Disable comment không có lý do → không rõ ai, khi nào quyết định giữ file
  lớn — thiếu accountability, nên bổ sung comment giải thích khi xử lý bug
  này dù chưa tách file ngay (theo `BUG-FE-HLD-006`).

## Bằng chứng

```
wc -l SourceControl.tsx                                → 8370
grep -n "^export function" SourceControl.tsx            → 11 hàm/component top-level
grep -c "useEffect(" SourceControl.tsx                  → 24
head -1 SourceControl.tsx                                → "/* eslint-disable max-lines */" (không giải thích)
```

## Đề xuất fix

1. Tách 11 export top-level đã liệt kê ở trên sang các file con theo nhóm:
   - `source-control-helpers.ts` (7 pure function dòng 354–774, không JSX)
   - `source-control-commit-area.tsx` (`CommitArea`)
   - `source-control-compare-summary.tsx` (`CompareSummary`,
     `CompareSummaryToolbarButton`, `shouldRefreshBranchCompareForStatusHead`,
     `shouldRefreshBranchCompareForRemoteStatus`, `shouldShowCompareSummary`)
   - `source-control-conflict-banner.tsx` (`ConflictSummaryCard`,
     `OperationBanner`, `TooManyChangesBanner`)
2. Component chính của file (chưa xác định tên — cần đọc phần chưa quét, dòng
   1–354 và 774–6556) rất có thể là component `SourceControl` mặc định; giữ
   lại sau cùng để tránh vỡ import trong lúc tách các phần phụ thuộc trước.
3. Bổ sung `-- Why:` cho disable comment ngay cả trước khi tách xong, theo
   đúng yêu cầu `BUG-FE-HLD-006`.

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- Liên quan: `BUG-FE-HLD-006` (disable không giải thích)
