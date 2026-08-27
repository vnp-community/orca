# TASK-BIGFILE-026 — Investigate: `SourceControlInner` (~5,700 dòng)

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-020..025 đã xong · **Status:** ✅ Done (ghi chú
thiết kế — sinh 2 task Move mới TASK-BIGFILE-225, 226)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` (Giai đoạn 2)

## Input

- File nguồn: `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`
  (sau TASK 020–025, còn lại chủ yếu là `function SourceControlInner()`,
  dòng 803–6,506 gốc — đọc lại đúng vùng này, dòng có thể lệch nhẹ sau các
  task trước).
- Component: `SourceControlInner` — 7 `useState`, 24 `useEffect`. Được bọc
  bởi `React.memo` thành `SourceControl` (export default).

## Nhiệm vụ

1. Đọc toàn bộ `SourceControlInner` — xác định:
   - Phần state quản lý (branch, staged/unstaged files, conflict detection).
   - Phần JSX layout điều phối (render `CommitArea`, `CompareSummary`,
     `ConflictSummaryCard`, ... — nên đã NGẮN hơn nhiều sau khi TASK
     020–025 tách các sub-component chi tiết ra).
   - Bất kỳ khối logic lớn nào khác chưa lộ diện qua export (gợi ý cần kiểm
     tra: drag-and-drop file, diff view nội tuyến).
2. Với mỗi khối lớn phát hiện được, đánh giá có thể tách thành custom hook
   (`useSourceControlXxxState`) hay component riêng.

## Output

- Cập nhật `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` mục "Giai
  đoạn 2" với kế hoạch cụ thể (thay vì mô tả chung chung hiện tại).
- Task Move mới (`TASK-BIGFILE-036`, ... tiếp theo dãy số hiện có) cho từng
  khối đã xác định, theo đúng format task Move.

## Không làm trong task này

Không sửa code — 7 `useState`/24 `useEffect` trong 1 component có rủi ro cao
nếu tách sai, cần thiết kế trước khi thực thi.

## Kết quả điều tra (2026-08-12)

Sau TASK 020–025, `SourceControl.tsx` = 7,086 dòng. `SourceControlInner`
thực tế nằm dòng 576–6,278 (~5,703 dòng — khớp ước tính gốc). Số liệu "7
`useState`/24 `useEffect`" ở bug/solution gốc đã cũ; đếm lại thật:
**~25 `useState`, 23 `useEffect`, 63 `useCallback`, 37 `useMemo`,
22 `useRef`**. JSX điều phối chỉ còn 5,333–6,278 (~945 dòng, đúng như dự
đoán — đã ngắn nhiều sau Giai đoạn 1). Phần logic/state: 576–5,332
(~4,756 dòng).

**Phương pháp**: liệt kê toàn bộ hook bằng `grep -n`, nhóm theo tên biến,
rồi grep TOÀN VĂN từng tên trong cụm ứng viên để xem có bị đọc/ghi từ cụm
logic khác không (tương đương "field-span" ở `TASK-BIGFILE-035`/`054`, áp
dụng cho React state).

**Phát hiện cấu trúc riêng của component React** (khác class field ở
`orca-runtime.ts`): 1 effect "reset khi đổi worktree" (dòng 1,847–1,869)
gọi `setState` trực tiếp cho GẦN NHƯ MỌI cụm state UI cục bộ
(`setFilterExpanded`, `setCollapsedSections`, `setPendingDiscard`,
`setPendingDiffCommentsClear`, `setIsClearingDiffComments`,
`setIsExecutingBulk`, ...). Đây KHÔNG phải lý do chặn tách (không giống
method dọn dẹp chung phức tạp ở `orca-runtime.ts` — effect này chỉ gọi
setState đơn giản), nhưng mỗi hook tách ra PHẢI tự expose 1 hàm `reset()`
để effect này gọi tổng hợp, thay vì set trực tiếp state nội bộ hook từ
ngoài.

**2 cụm an toàn, đã sinh task Move** (xem
`../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` Giai đoạn 2 để có
đầy đủ bảng dữ liệu):

- `TASK-BIGFILE-225` — diff-comments panel state (721–826, ~105 dòng).
  Không bị đọc/ghi từ cụm nào khác ngoài effect reset dùng chung.
- `TASK-BIGFILE-226` — branch-compare + git-history refresh scheduling
  (4,552–4,857, ~305 dòng). Bị GỌI (không phải đọc field) từ 3 điểm ngoài
  cụm qua 1 callback-ref đã có sẵn — phụ thuộc 1 chiều, an toàn.

**Cụm KHÔNG an toàn — không sinh task Move**:

- `isExecutingBulk` (khai báo dòng 1,685): 1 flag khoá thực thi dùng chung
  bởi bulk-stage/bulk-unstage/discard-all/discard-entry **VÀ** sâu trong
  luồng Create-PR-intent (dòng 3,570). Tách "bulk actions" hay "discard"
  riêng cần tách flag này thành nhiều flag độc lập trước — thay đổi hành
  vi, ngoài phạm vi Move thuần.
- Luồng Create-PR-intent / hosted-review creation (~2,569–3,916, ~1,350
  dòng, 25% phần logic) — cụm lớn nhất, phụ thuộc chéo `commitDrafts`,
  branch-compare, hosted-review state, PR-generation records, và
  `isExecutingBulk`. Là lõi thật của component; cần 1 Investigate RIÊNG sau
  khi có kinh nghiệm với pattern hook từ 225/226.

Chi tiết đầy đủ (bảng dữ liệu, danh sách symbol, lý do từng quyết định) đã
ghi vào `../solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md` mục "Giai
đoạn 2".
