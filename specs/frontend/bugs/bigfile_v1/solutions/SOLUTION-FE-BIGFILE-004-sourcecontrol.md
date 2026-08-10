# SOLUTION-FE-BIGFILE-004 — Tách `SourceControl.tsx` (8,370 dòng)

**Bug:** `../BUG-FE-BIGFILE-004-sourcecontrol.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #7 — xem `SOLUTION-FE-BIGFILE-001` mục 3
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

## Cấu trúc phát hiện thêm (so với bug gốc)

Đọc bổ sung xác nhận: component chính không phải là hàm ẩn danh mà có tên rõ
ràng — `SourceControlInner` (dòng 803–6,506, **~5,700 dòng**, chiếm 68% file),
được bọc bởi `React.memo` thành `SourceControl` (dòng 6,506) rồi export
default (dòng 6,507). Ngoài ra còn 2 sub-component KHÔNG export (module-
private, chỉ dùng nội bộ file): `SourceControlTreeDirectoryRow` (7,701),
`SourceControlBranchTreeDirectoryRow` (7,811).

## Cấu trúc file mới

| File mới | Nội dung | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `source-control-helpers.ts` | `resolveSourceControlBaseRef`, `resolveSourceControlCompareBaseRef`, `shouldClearBranchCompareForMissingBase`, `resolveSourceControlPickerBaseRef`, `BRANCH_REFRESH_INTERVAL_MS`, `normalizeSourceControlViewMode`, `readCommitDraftForWorktree`, `writeCommitDraftForWorktree`, `shouldRenderCommitArea`, `pickDefaultSourceControlAgent`, `refreshSourceControlAfterRemoteAction`, `clearRemoteActionErrorsForCompletedConflictOperations` | 354–802 (trước `SourceControlInner`) | ~450 |
| `source-control-commit-area.tsx` | `CommitArea` | 6,556–7,056 | ~500 |
| `source-control-compare-summary.tsx` | `shouldRefreshBranchCompareForStatusHead`, `shouldRefreshBranchCompareForRemoteStatus`, `shouldShowCompareSummary`, `CompareSummary`, `CompareSummaryToolbarButton` | 7,057–7,533 | ~475 |
| `source-control-banners.tsx` | `ConflictSummaryCard`, `OperationBanner`, `TooManyChangesBanner` | 7,534–7,700 | ~165 |
| `source-control-tree-rows.tsx` | `SourceControlTreeDirectoryRow`, `SourceControlBranchTreeDirectoryRow` (module-private trong file gốc — giữ KHÔNG export ở file mới nếu chỉ dùng trong `SourceControlInner`, kiểm tra lại khi tách) | 7,701–8,296 | ~595 |
| `source-control-action-button.tsx` | `ActionButton` | 8,297–8,370 | ~75 |
| `SourceControl.tsx` (giữ nguyên tên) | `SourceControlInner` + `React.memo` wrap + `export { ... }` cho các file trên | 803–6,507 | ~5,700 (chưa giảm nhiều — xem Giai đoạn 2) |

## Giai đoạn 1 — Tách phần đã export riêng (rủi ro thấp)

1. `gitnexus impact` cho từng export sẽ di chuyển (12 hàm + 5 component).
2. Tách `source-control-helpers.ts` trước (không JSX, không hook).
3. Tách 4 file component còn lại (`commit-area`, `compare-summary`,
   `banners`, `action-button`) — theo đúng thứ tự trong bảng, mỗi file 1
   commit.
4. Với `source-control-tree-rows.tsx`: xác nhận 2 component này có thực sự
   chỉ dùng nội bộ `SourceControlInner` hay không (chúng KHÔNG có `export`
   trong file gốc) — nếu đúng, tách sang file riêng vẫn OK (chỉ cần export từ
   file mới rồi import vào `SourceControlInner`, không cần barrel re-export
   ở `SourceControl.tsx` vì chúng chưa từng là public API).

Sau giai đoạn 1: `SourceControl.tsx` giảm từ 8,370 → ~5,700 dòng (giữ nguyên
`SourceControlInner`) — vẫn còn Critical, cần Giai đoạn 2.

## Giai đoạn 2 — Tách nội bộ `SourceControlInner` (~5,700 dòng, rủi ro cao hơn)

Đây là 1 component React đơn lẻ rất lớn (7 `useState` + 24 `useEffect`) —
không có ranh giới export sẵn như phần đã tách ở Giai đoạn 1. Cần đọc trực
tiếp trước khi thiết kế, nhưng gợi ý hướng tiếp cận dựa trên tên các component
đã tách (Commit Area, Compare Summary, Conflict Banner) — rất có thể
`SourceControlInner` chứa:

1. Logic quản lý state chung (branch, staged/unstaged files, conflict
   detection) — giữ lại trong component chính hoặc tách thành custom hook
   `useSourceControlState`.
2. JSX layout điều phối (render `CommitArea`, `CompareSummary`,
   `ConflictSummaryCard`, ... theo điều kiện) — phần này nên NGẮN sau khi các
   sub-component đã tách ở Giai đoạn 1, vì phần lớn JSX chi tiết đã chuyển đi.
3. Nếu sau khi tách JSX điều phối, phần còn lại vẫn > 2,000 dòng, khả năng
   cao còn 1-2 khối logic lớn khác chưa lộ diện qua export (vd: xử lý
   drag-and-drop file, xử lý diff view nội tuyến) — đọc trực tiếp để xác định
   trước khi tách tiếp, không đoán trước trong solution doc này.

## Xác minh

- `pnpm run typecheck`, `pnpm run lint`
- Test hiện có (`SourceControl.test.tsx` nếu có)
- `gitnexus detect_changes({scope: "all"})`
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

- Giai đoạn 1: **Thấp** — ranh giới export đã có sẵn, tương tự
  `SOLUTION-FE-BIGFILE-010` (`BrowserPane.tsx`).
- Giai đoạn 2: **Trung bình-cao** — cần đọc/thiết kế thêm trước khi tách,
  không có kế hoạch chi tiết sẵn trong solution này (chủ động để lại quyết
  định cho người thực hiện SAU khi đã đọc trực tiếp phần còn lại, tránh đưa
  ra kế hoạch sai dựa trên suy đoán).

## Bổ sung: disable comment thiếu giải thích

Theo `BUG-FE-HLD-006`, bổ sung `-- Why:` cho `/* eslint-disable max-lines */`
ở dòng 1 ngay cả trước khi tách xong hoàn toàn.
