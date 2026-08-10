# TASK-BIGFILE-014 — Move: `browser-pane-annotation-card.tsx`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-010-browserpane.md`

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
