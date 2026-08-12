# TASK-BIGFILE-016 — Move: `browser-pane-local.tsx`

**Loại:** Move (cơ học) · **Effort:** M
**Phụ thuộc:** TASK-BIGFILE-015 (làm sau, để tránh 2 task cùng sửa
`BrowserPane.tsx` song song và conflict dòng)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-010-browserpane.md`

## ⚠️ Kết quả thực thi (2026-08-12)

`BrowserPagePane` thực tế chiếm dòng 649–3,815 (đến hết file, sau khi
TASK-014/015 đã chạy trước trong cùng phiên) — khớp đúng "component còn lại
đến hết file" như doc mô tả, ranh giới tuyệt đối lệch so với số dòng gốc
(2,675–5,841) nhưng đó là hệ quả của việc TASK-014/015 đã xoá bớt phía trên,
không phải sai lệch ranh giới của bản thân component. Đã co-locate thêm
`preventAgentSendTargetOutsideDismiss` (giáp ngay trước, chỉ
`BrowserPagePane` dùng — xem ghi chú ở TASK-015). `BrowserPane.tsx` sau khi
xoá còn **513 dòng** (không phải ~120 như ước tính — do ~15 hàm/const phụ
trợ dùng chung giữa Remote/Local phải NẰM LẠI và được export cho cả 2 file
con import ngược, đúng theo thiết kế "export {...} cho 3 file trên" của
solution doc, chỉ là số lượng hàm chia sẻ nhiều hơn ước tính "ranh giới rõ
100%, không chia sẻ state" ban đầu). Toàn bộ các hàm này là pure function,
không phải module-private state dùng chéo kiểu `ipc/pty.ts`, nên không có
rủi ro HIGH/CRITICAL. Phát hiện + sửa 1 test phụ thuộc vị trí file:
`BrowserAnnotationSendMenuContent.test.tsx` đọc source `BrowserPane.tsx` để
đếm `<BrowserAnnotationSendMenuContent` — đã cập nhật trỏ sang
`browser-pane-local.tsx` (nơi JSX thực sự nằm sau khi move). `gitnexus
impact`/`detect_changes` không dùng được (MCP lỗi kết nối, CLI segfault) —
dùng grep thủ công thay thế.

## Input

- File nguồn: `frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx`
- Đọc **đúng dòng 2,675–5,841** (đây là phần LỚN NHẤT của file, ~3,166 dòng
  — nhưng ranh giới đã xác định rõ, chỉ cần đọc để copy, không cần thiết kế
  lại).
- Symbol cần chuyển: `BrowserPagePane`

## Output

- File mới: `frontend/src/renderer/src/components/browser-pane/browser-pane-local.tsx`
- File nguồn import `BrowserPagePane` từ file mới.

## Các bước

1. `gitnexus impact({target: "BrowserPagePane", direction: "upstream"})` —
   dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 2,675–5,841, copy nguyên văn + import cần thiết.
3. Tạo file mới, `export function BrowserPagePane(...)`.
4. Sửa `BrowserPane.tsx`: xoá định nghĩa gốc, thêm import từ file mới.
5. Sau bước này, `BrowserPane.tsx` chỉ còn phần wrapper (dòng 783–900,
   `export default function BrowserPane(...)`) + 2 dòng import — xác nhận
   file gốc còn ~120 dòng.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `BrowserPane.tsx` giảm
      xuống ~120 dòng (đã ra khỏi danh sách bigfile hoàn toàn)
- [ ] Test liên quan pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx
rm frontend/src/renderer/src/components/browser-pane/browser-pane-local.tsx
```
