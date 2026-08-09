# TASK-FE2E-007 — `main.tsx`: thay import tĩnh bằng dynamic import tới `pair-code-app-entry.tsx`

**Source Solution:** [SOL-FE2E-003](../solutions/SOL-FE2E-003-lazy-split-pairing-bundle.md) §2.2
**Priority:** P1
**Loại:** Sửa file hiện có
**Depends on:** TASK-FE2E-006
**Status:** ✅ DONE — 2026-08-09

---

## Context

```bash
cat frontend/src/renderer/src/web/main.tsx
```

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/web/main.tsx`

1. Xoá các import tĩnh đã chuyển sang `pair-code-app-entry.tsx` ở TASK-FE2E-006: `WebConnect`, `web-pairing` (`clearPairingInputFromAddressBar`, `decideWebPairingStartup`, `readPairingInputFromLocation`), `web-runtime-environment` (`createStoredWebRuntimeEnvironment`, `readStoredWebRuntimeEnvironment`, `saveStoredWebRuntimeEnvironment`), `installWebPreloadApi`.
2. Xoá định nghĩa `WebRoot`/`WebRootBoundary` khỏi `main.tsx` (đã chuyển sang file mới).
3. Sửa `renderOriginalPairCodeApp`:

```diff
- function renderOriginalPairCodeApp() {
-   const rootEl = document.getElementById('root')
-   if (rootEl) {
-     ReactDOM.createRoot(rootEl).render(
-       <I18nProvider>
-         <WebRootBoundary />
-       </I18nProvider>
-     )
-   }
- }
+ // Why: only /auth/config 404 (Desktop Pair Code sharing, CR-FE2E-003) reaches
+ // this branch — dynamic import keeps TweetNaCl + the E2EE pairing UI out of
+ // the bundle every multi-user browser downloads.
+ function renderOriginalPairCodeApp(): void {
+   void import('./pair-code-app-entry').then(({ mountPairCodeApp }) => mountPairCodeApp())
+ }
```

> [!IMPORTANT]
> `renderOriginalPairCodeApp` giữ nguyên là hàm **đồng bộ** (`void import(...).then(...)`, không phải `async/await`) — không cần sửa call site trong `fetch('/auth/config').then(...)`/`.catch(...)` ở cuối file. Giữ nguyên đoạn `fetch('/auth/config')...` không đổi.

## Verify

```bash
cd frontend
grep -n "^import" src/renderer/src/web/main.tsx
# kỳ vọng: không còn import WebConnect/web-pairing/web-runtime-environment/installWebPreloadApi tĩnh

node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/web/__tests__/web-index-html.test.ts
```

## Definition of Done

- [x] `main.tsx` không còn import tĩnh nào của module pairing
- [x] `renderOriginalPairCodeApp` dùng dynamic `import('./pair-code-app-entry')`
- [x] `fetch('/auth/config')` branch logic không đổi (giữ nguyên nội dung, chỉ đổi vị trí — đặt trước hàm `renderOriginalPairCodeApp` thay vì sau, không ảnh hưởng hành vi vì function declaration hoisting)
- [x] File giảm mạnh: **~110 dòng → 25 dòng**
- [x] Test `web-index-html.test.ts` — không có regression mới (so sánh trước/sau: fail giống hệt, do thiếu `vite.web.config.ts` — pre-existing, không liên quan)
