# TASK-BIGFILE-026 — Investigate: `SourceControlInner` (~5,700 dòng)

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-020..025 đã xong · **Status:** ⬜ Todo
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
