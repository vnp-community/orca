# TASK-FE-020 — Tạo Admin Entry Point + Build Config

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md) §5
**Depends on:** TASK-FE-014, TASK-FE-015, TASK-FE-016, TASK-FE-017, TASK-FE-018, TASK-FE-019
**Blocks:** —
**Effort:** S (~25 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo entry point để Admin SPA có thể build và serve độc lập tại `/admin/`.

---

## Files cần tạo/sửa

### `src/renderer/admin-index.html` [NEW]

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="robots" content="noindex, nofollow" />
    <title>Orca Admin</title>
    <link rel="icon" href="/favicon.ico" />
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="./src/admin/admin-main.tsx"></script>
  </body>
</html>
```

### `src/renderer/src/admin/admin-main.tsx` [NEW]

```typescript
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { AdminApp } from './AdminApp'

// Guard: redirect to login if no session
// (Backend handles this via 401 redirect — frontend just mounts)
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AdminApp />
  </StrictMode>
)
```

### Vite config [MODIFY]

Tìm file `vite.config.ts` (hoặc tương đương trong renderer build config).

Thêm `admin-index.html` như second entry point:

```typescript
// vite.config.ts
export default defineConfig({
  build: {
    rollupOptions: {
      input: {
        main:  resolve(__dirname, 'index.html'),          // existing
        admin: resolve(__dirname, 'admin-index.html'),   // NEW
      }
    }
  }
})
```

---

## Constraints

- Admin SPA phải build thành **separate JS bundle** — không merge vào main bundle
- Backend phải serve `admin-index.html` khi route `/admin/*` được request
- `src/renderer/src/admin/` chỉ chứa Admin SPA code — không import từ main App

---

## Verify

```bash
# Build check
npx vite build
# Confirm admin bundle xuất hiện trong dist/
ls dist/ | grep admin
```
