# SOLUTION-FE-BIGFILE-008 — Tách `WorktreeList.tsx` (6,877 dòng)

**Bug:** `../BUG-FE-BIGFILE-008-worktreelist.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #4 — xem `SOLUTION-FE-BIGFILE-001` mục 3
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

## Cấu trúc file mới

| File mới | Nội dung | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `worktree-list-helpers.ts` | `countRecordKeysByReference`, `shouldAdjustWorktreeSidebarMeasuredRowScroll`, `resolvePendingSidebarReveal`, `renderRowContainsWorktree`, `getRenderRowKey`, `getWorktreeDragGroups`, `canKeepImportedWorktreesHidden` | 324–1,222 (không liên tục — 7 hàm rải rác, xem lưu ý) | ~250 (chỉ tính riêng phần hàm, không tính khoảng trống giữa các hàm dùng cho component) |
| `worktree-list-visibility-listener.ts` | `installWorktreeVisibleRefreshVisibilityListener` | 5,150–~5,170 | ~20 |
| `WorktreeList.tsx` (giữ nguyên tên) | Phần component chính (`export default WorktreeList` dòng 6,877) + `export { ... }` cho 2 file trên | — | ~6,600 (giảm nhẹ so với gốc, phần lớn vẫn còn trong component chính) |

**Lưu ý quan trọng:** khác với `ipc/pty.ts` (BUG-FE-BIGFILE-011), 7 hàm
helper ở đây **KHÔNG liên tục** — nằm rải rác từ dòng 324 đến 1,223, xen giữa
là code khác (rất có thể là type definition hoặc phần đầu component). Trước
khi tách, cần đọc để xác nhận:
1. 7 hàm này có thực sự độc lập (không đọc/ghi biến module-level chung với
   phần component ở giữa) hay không.
2. Khoảng trống giữa các hàm (vd dòng 342→1,050, gần 700 dòng) chứa gì — có
   thể là type definition dùng chung cần tách cùng, hoặc là phần đầu của
   component chính cần giữ lại.

## Các bước thực hiện

1. `gitnexus impact` cho từng hàm trong 7 hàm helper — xác nhận caller.
2. Đọc chi tiết dòng 324–1,223 (thay vì chỉ dựa vào kết quả grep top-level đã
   có) để xác nhận ranh giới chính xác trước khi cắt — đây là bước bắt buộc
   khác với các file có export liên tục.
3. Copy 7 hàm (đã xác nhận độc lập) sang `worktree-list-helpers.ts`.
4. Copy `installWorktreeVisibleRefreshVisibilityListener` (dòng 5,150, gần
   cuối file, ngay trước component chính) sang
   `worktree-list-visibility-listener.ts`.
5. `WorktreeList.tsx` thêm `export { ... } from ...` cho 2 file trên.

## Xác minh

- `pnpm run typecheck`, `pnpm run lint`
- Test hiện có (nếu có `WorktreeList.test.tsx` cùng thư mục)
- `gitnexus detect_changes({scope: "all"})`
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

Thấp cho 7 hàm helper NẾU xác nhận được chúng thực sự pure/độc lập ở bước 2 —
nhưng bước 2 bắt buộc phải làm cẩn thận hơn bình thường vì các export không
liên tục (khác các file khác trong đợt này). Nếu phát hiện các hàm này thực
ra phụ thuộc biến module-level chia sẻ với component chính, KHÔNG tách —
ghi chú lại phát hiện này vào bug `BUG-FE-BIGFILE-008` và bỏ qua bước này,
chuyển hướng sang phân tích lại component chính trước.

## Ngoài phạm vi solution này

Phần component chính (dòng ~1,223–5,150, ~3,900 dòng, chưa xác định tên chính
xác — cần đọc trực tiếp) là phần lớn nhất còn lại sau bước trên. Đề xuất đánh
giá lại SAU khi bước 1–5 hoàn tất và đo lại kích thước thực tế, quyết định có
cần 1 solution riêng (SOLUTION-FE-BIGFILE-008b) cho phần này hay không.
