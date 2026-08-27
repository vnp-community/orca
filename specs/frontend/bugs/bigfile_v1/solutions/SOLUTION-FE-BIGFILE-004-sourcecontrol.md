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

## Giai đoạn 2 — Tách nội bộ `SourceControlInner` (kết quả điều tra thật, TASK-BIGFILE-026)

**Cập nhật (TASK-BIGFILE-026, đã làm — thay số liệu đoán ở trên bằng dữ liệu
thật):** sau Giai đoạn 1 (TASK 020–025), `SourceControl.tsx` còn 7,086 dòng.
`SourceControlInner` thực tế nằm ở dòng 576–6,278 (~5,703 dòng, gần khớp ước
tính "~5,700" ban đầu) — số liệu "7 `useState`/24 `useEffect`" ở bản gốc đã
cũ: đếm thật cho ra **~25 `useState`, 23 `useEffect`, 63 `useCallback`, 37
`useMemo`, 22 `useRef`**. JSX điều phối (`return (...)`) chỉ còn dòng
5,333–6,278 (~945 dòng) — đúng như dự đoán, đã ngắn nhiều sau Giai đoạn 1.
Phần logic/state (576–5,332, ~4,756 dòng) là phần cần phân tích.

### Phương pháp

Không đọc tuyến tính toàn bộ 4,756 dòng. Thay vào đó: liệt kê mọi
`useState`/`useEffect`/`useCallback`/`useMemo`/`useRef` bằng `grep`, nhóm
theo tên biến, rồi với mỗi cụm ứng viên — `grep` TOÀN BỘ tên state/hàm trong
cụm để xem có bị đọc/ghi từ NGOÀI phạm vi dòng của chính cụm đó không (tương
tự "field-span" dùng ở `TASK-BIGFILE-035`/`054` cho `orca-runtime.ts`, áp
dụng cho React state thay vì class field).

**Phát hiện quan trọng riêng cho component React (khác class field):** có 1
effect "reset khi đổi worktree" (dòng 1,847–1,869) chủ động gọi
`setPendingDiscard(null)`, `setPendingDiffCommentsClear(null)`,
`setIsClearingDiffComments(false)`, `setIsExecutingBulk(false)`, ... — tức
là MỌI cụm state UI cục bộ đều bị 1 effect dùng chung chạm tới. Đây KHÔNG
phải lý do để chặn tách (khác với method dọn dẹp chung ở `orca-runtime.ts`,
effect này chỉ gọi setState đơn giản) — nhưng mỗi hook tách ra cần tự expose
1 hàm `reset()`, và effect dùng chung sẽ gọi tổng hợp `hookA.reset()`,
`hookB.reset()`, ... thay vì set trực tiếp.

### 1. Cụm state "lõi" dùng chéo rộng — KHÔNG tách trong đợt này

| State/field | Dòng khai báo | Vấn đề |
|---|---|---|
| `isExecutingBulk` | 1,685 | Dùng chung bởi bulk-stage (4325), bulk-unstage (4356), stage-all-primary (4459), discard-all/discard-entry (5153–5301) **VÀ** sâu bên trong `runCreatePrIntent` (dòng 3,570) — 1 flag khoá thực thi cho 4 luồng hành động khác nhau. Tách "bulk actions" hay "discard" riêng đòi hỏi tách flag này thành nhiều flag độc lập trước — đó là thay đổi hành vi, ngoài phạm vi Move thuần. |
| Luồng "Create PR intent" / hosted review creation | ~2,569–3,916 (~1,350 dòng, 25% phần logic) | Cụm lớn nhất — `handlePullRequestCreated`, `handleGeneratePullRequestFieldsForActive`, `handleCreatePullRequest`, `createHostedReviewForCreatePrIntent`, `runCreatePrIntent`, ... phụ thuộc chéo `commitDrafts`, `branchCompare`, `hostedReviewCreationState`, PR-generation records, và `isExecutingBulk` ở trên. Đây là lõi thật của component — cần 1 Investigate RIÊNG (đọc kỹ luồng, có thể redesign trước khi tách), không Move cơ học được. |

### 2. 2 cụm tách được — ranh giới hẹp, phụ thuộc 1 chiều (an toàn)

| # | Task mới | Cụm | Dòng gốc | State sở hữu | Ghi chú |
|---|---|---|---|---|---|
| 1 | TASK-BIGFILE-225 | Diff-comments panel (copy/clear/expand) | 721–826 (+ 3 điểm reset tại 1,852–1854, JSX tại 5,382–5,493 và 6,109–6,138) | `diffCommentsExpanded`, `diffCommentsCopied`, `pendingDiffCommentsClear`, `isClearingDiffComments` | Không bị đọc/ghi từ cụm nào khác ngoài effect reset dùng chung. Input cần: `diffCommentsForActive`, `diffCommentsPrompt`, `activeWorktreeId`, `clearDiffComments`, `clearDiffCommentsForFile`. |
| 2 | TASK-BIGFILE-226 | Branch-compare + git-history refresh scheduling | 4,552–4,857 (~305 dòng) | `branchCompareInFlightRef`, `branchCompareRerunRef`, `branchCompareRunPromiseRef`, `refreshBranchCompareRef`, `branchCompareStatusHeadRef`, `branchCompareRemoteStatusRef`, `gitHistoryByWorktree`, `gitHistoryRequestSeqRef` | Không đọc `isExecutingBulk`/`pendingDiscard`/`commitDrafts`/PR-intent state (xác nhận bằng grep). Bị GỌI (không phải đọc field) từ 3 điểm ngoài cụm (dòng 1,969, 2,415, 2,503 — sau commit/remote action thành công) qua `refreshBranchCompareRef.current()` — interface callback-ref đã sẵn, 1 chiều (bên ngoài gọi vào, cụm này không gọi ra). |

**Tổng tách được đợt này (nếu 225+226 chạy): ~430 dòng** — khiêm tốn so với
4,756 dòng phần logic, nhưng đúng 2 cụm THẬT SỰ an toàn tìm được. Phần còn
lại (bulk/discard ~700 dòng gắn `isExecutingBulk`, PR-intent/hosted-review
~1,350 dòng, cộng nhiều đoạn store-selector/derived-value rải rác không tạo
thành cụm rõ ràng) cần thiết kế thêm hoặc chấp nhận ở lại `SourceControl.tsx`.

### Việc CHƯA làm (ngoài phạm vi Investigate)

- Không sửa code trong task này (TASK-BIGFILE-026).
- Không thiết kế lại `isExecutingBulk` thành nhiều flag độc lập — đây là
  thay đổi hành vi/kiến trúc, cần Investigate riêng SAU KHI có thêm test
  coverage cho luồng bulk-action/discard/create-PR-intent (hiện chưa rõ độ
  phủ test của 3 luồng này khi chạy đồng thời).
- Không Investigate sâu luồng Create PR intent (~1,350 dòng) — để lại 1 task
  Investigate riêng, đặt SAU khi 225/226 đã chạy và có thêm kinh nghiệm với
  pattern custom-hook trong file này.

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
