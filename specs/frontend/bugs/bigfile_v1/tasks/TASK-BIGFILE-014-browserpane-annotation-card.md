# TASK-BIGFILE-014 — Move: `browser-pane-annotation-card.tsx`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-010-browserpane.md`

## ⚠️ Kết quả thực thi (2026-08-12)

Ranh giới dòng gốc trong doc (377–782) SAI — đã đọc lại file thực tế trước
khi sửa (theo nguyên tắc chung của đợt task này). `PendingBrowserAnnotationCard`
thực sự chỉ chiếm **377–524** (~148 dòng); 526–782 là 20 helper function
độc lập khác (`browserPageExists`, `isRemoteBrowserPageMissingError`,
`toDisplayUrl`, `retryBrowserTabLoad`, v.v.) dùng chung bởi cả
`RemoteBrowserPagePane` lẫn `BrowserPagePane` — KHÔNG di chuyển, ở lại
`BrowserPane.tsx` (export thêm để 2 file mới ở TASK-015/016 import lại).
Component không export ở file gốc — đã xác nhận và giữ đúng theo input.
`BROWSER_ANNOTATION_INTENT_OPTIONS` (const riêng, chỉ dùng trong component
này) được di chuyển cùng thay vì để lại mồ côi. `gitnexus impact` không dùng
được (MCP "Connection closed", CLI segfault) — thay bằng grep thủ công xác
nhận symbol chỉ có 1 nơi định nghĩa + 1 nơi dùng (không có test import trực
tiếp) trước khi di chuyển.

## Input

- File nguồn: `frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx`
- Đọc **đúng dòng 377–782**.
- Symbol cần chuyển: `PendingBrowserAnnotationCard` (component, không export
  ở file gốc — kiểm tra lại khi đọc; nếu không export, giữ không export ở
  file mới, chỉ import nội bộ)

## Output

- File mới: `frontend/src/renderer/src/components/browser-pane/browser-pane-annotation-card.tsx`
- File nguồn (`BrowserPane.tsx`) import `PendingBrowserAnnotationCard` từ
  file mới thay vì định nghĩa tại chỗ (KHÔNG cần barrel export nếu component
  này vốn không phải public API — chỉ cần `import { PendingBrowserAnnotationCard } from './browser-pane-annotation-card'`).

## Các bước

1. `gitnexus impact({target: "PendingBrowserAnnotationCard", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL (kiểm tra xem có file test import trực tiếp
   component này không, dù nó không export ở mức module — vẫn có thể được
   test qua cách khác).
2. Đọc dòng 377–782, copy nguyên văn + import cần thiết (props type nếu có
   định nghĩa ngay phía trên component, copy cùng).
3. Tạo file mới, `export function PendingBrowserAnnotationCard(...)` (export
   để file `BrowserPane.tsx` import được).
4. Sửa `BrowserPane.tsx`: xoá định nghĩa gốc, thêm
   `import { PendingBrowserAnnotationCard } from './browser-pane-annotation-card'`
   ở đầu file.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `BrowserPane.tsx` giảm
      ~405 dòng
- [ ] Test liên quan (nếu có, `browser-pane/*.test.tsx`) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx
rm frontend/src/renderer/src/components/browser-pane/browser-pane-annotation-card.tsx
```
