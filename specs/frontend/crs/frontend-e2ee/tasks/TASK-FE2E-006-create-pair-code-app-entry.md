# TASK-FE2E-006 — Tạo `pair-code-app-entry.tsx` (copy `WebRoot`/`WebRootBoundary` từ `main.tsx`)

**Source Solution:** [SOL-FE2E-003](../solutions/SOL-FE2E-003-lazy-split-pairing-bundle.md) §2.1
**Priority:** P1
**Loại:** File mới
**Depends on:** TASK-FE2E-005
**Estimated:** 25 phút
**Status:** ✅ DONE — 2026-08-09

---

## Context

```bash
cat frontend/src/renderer/src/web/main.tsx
grep -n "lazyWithRetry" frontend/src/renderer/src/lib/lazy-with-retry.ts
```

Đọc trước toàn bộ `main.tsx` hiện tại — đặc biệt định nghĩa `WebRoot`/`WebRootBoundary` sẽ được **di chuyển nguyên vẹn** (không viết lại logic) sang file mới.

## Thay đổi cần thực hiện

**File mới:** `frontend/src/renderer/src/web/pair-code-app-entry.tsx`

Copy nguyên vẹn thân hàm `WebRoot`/`WebRootBoundary` hiện có trong `main.tsx`, với 2 điều chỉnh:
1. Đổi `import WebConnect from './WebConnect'` (tĩnh) → `const WebConnect = lazy(() => import('./WebConnect'))` dùng `lazyWithRetry` (`import { lazyWithRetry as lazy } from '@/lib/lazy-with-retry'`) — đúng pattern `main-web-bootstrap.tsx` đã dùng cho cùng component.
2. `App` cũng đổi sang `lazy(() => import('../App'))` thay vì import tĩnh (nếu `main.tsx` hiện đang import tĩnh).

Bọc `<WebConnect>`/`<App>` trong `<Suspense fallback={<div className="min-h-dvh bg-background" />}>`.

Export `mountPairCodeApp(): void` — hàm thực hiện `ReactDOM.createRoot(rootEl).render(...)`.

> Xem code đầy đủ tại [SOL-FE2E-003 §2.1](../solutions/SOL-FE2E-003-lazy-split-pairing-bundle.md#21-pair-code-app-entrytsx-file-mới--đúng-như-cr-đề-xuất--giữ-nguyên-tênvị-trí) — copy-paste trực tiếp từ đó, chỉ chỉnh sửa nếu `main.tsx` thật có khác biệt nhỏ so với snapshot trong solution (kiểm tra bằng Context ở trên trước khi copy).

> [!IMPORTANT]
> KHÔNG viết lại `decideWebPairingStartup`/`readPairingInputFromLocation`/logic `hasEnvironment` — copy y nguyên. Đây là bước di chuyển code (move), không phải viết lại (rewrite).

## Verify

```bash
cd frontend
node_modules/.bin/tsc --noEmit src/renderer/src/web/pair-code-app-entry.tsx 2>&1 | head -30 || true
# (tsc --noEmit toàn project hiện không chạy được — xem specs/frontend/bugs/hld-v1/tasks/NOTES.md —
#  dùng lệnh trên chỉ để bắt lỗi syntax cơ bản của riêng file mới)
```

## Definition of Done

- [x] File `pair-code-app-entry.tsx` tồn tại, export `mountPairCodeApp()`
- [x] `WebConnect`/`App` dùng `lazyWithRetry`, bọc `Suspense` — phát hiện `App` **đã lazy sẵn** trong `main.tsx` gốc (`const App = lazy(() => import('../App'))` dùng `lazyWithRetry` từ trước), chỉ `WebConnect` cần đổi từ import tĩnh sang lazy
- [x] Logic `WebRoot`/`WebRootBoundary` copy y nguyên từ `main.tsx` gốc — không đổi 1 dòng logic, chỉ đổi cách import 2 component
- [x] Chưa xoá gì ở `main.tsx` (làm ở TASK-FE2E-007) — xác nhận `main.tsx` chưa bị sửa
