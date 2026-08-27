# SOLUTION-FE-BIGFILE-010 — Tách `BrowserPane.tsx` (5,841 dòng)

**Bug:** `../BUG-FE-BIGFILE-010-browserpane.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #5 — xem `SOLUTION-FE-BIGFILE-001` mục 3
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

## Cấu trúc file mới

Đây là file có ranh giới component **rõ ràng nhất** trong nhóm Critical — 3
component top-level tách biệt hoàn toàn, không có export nào xen giữa gợi ý
chia sẻ state ẩn:

| File mới | Nội dung | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `browser-pane-annotation-card.tsx` | `PendingBrowserAnnotationCard` | 377–782 | ~405 |
| `browser-pane-remote.tsx` | 11 type `Remote*`/`Pending*` (dòng 185–307) + `RemoteBrowserPagePane` | 185–307, 901–2,674 | ~1,900 |
| `browser-pane-local.tsx` | `BrowserPagePane` | 2,675–5,841 | ~3,166 |
| `BrowserPane.tsx` (giữ nguyên tên) | `export default function BrowserPane(...)` (wrapper, dòng 783–900) + `export { ... }` cho 3 file trên | 783–900 | ~120 |

## Các bước thực hiện

1. `gitnexus impact({target: "BrowserPane", direction: "upstream"})` +
   tương tự cho `RemoteBrowserPagePane`/`BrowserPagePane` nếu chúng được
   import trực tiếp ở nơi khác (không chỉ qua `BrowserPane` wrapper) — cần
   xác nhận trước khi tách vì có thể có test/story file import thẳng
   component con.
2. Tách `browser-pane-annotation-card.tsx` trước (nhỏ nhất, độc lập nhất).
3. Tách `browser-pane-remote.tsx` — copy 11 type (dòng 185–307) TRƯỚC
   `RemoteBrowserPagePane` (dòng 901–2,674) vào CÙNG 1 file (các type này
   đặt tên `Remote*` khớp trực tiếp với component, xác nhận qua bug doc gốc).
4. Tách `browser-pane-local.tsx` — copy `BrowserPagePane` nguyên văn.
5. `BrowserPane.tsx` giữ lại phần wrapper (dòng 783–900), thêm `export { ... }`
   cho 3 file trên. Nếu wrapper quyết định render component nào dựa trên
   props/context, xác nhận import đúng từ 2 file mới.

## Xác minh

- `pnpm run typecheck`, `pnpm run lint`
- Test hiện có (browser pane có khả năng cao có test riêng do liên quan CDP
  streaming — kiểm tra `browser-pane/*.test.tsx`)
- `gitnexus detect_changes({scope: "all"})`
- `node scripts/find-frontend-bigfiles.mjs` — kỳ vọng `BrowserPane.tsx` giảm
  từ 5,841 xuống ~120 dòng

## Rủi ro

**Thấp** — đây là ứng viên tách dễ nhất trong nhóm Critical, ranh giới
component đã rõ 100% từ trước, không cần thiết kế lại gì. Rủi ro chính duy
nhất: 64 `useEffect` tổng cộng trên 3 component — cần xác nhận không có
`useEffect` nào trong `BrowserPane` wrapper (dòng 783–900) đọc/ghi state nội
bộ của `RemoteBrowserPagePane`/`BrowserPagePane` qua closure (thay vì qua
props) — nếu có, phải giữ lại closure đó qua props/callback khi tách.

## Sau khi tách

Cả 2 component con (`RemoteBrowserPagePane` ~1,774 dòng,
`BrowserPagePane` ~3,166 dòng) vẫn còn trên ngưỡng High/Crit — không nằm
trong phạm vi solution này (bug gốc chỉ yêu cầu xử lý `BrowserPane.tsx` như 1
đơn vị). Đánh giá lại nhu cầu tách tiếp sau khi đo lại kích thước thực tế của
2 file mới.
