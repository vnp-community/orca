# CR-FE2E-003 — Code-split E2EE Pairing khỏi bundle multi-user

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-FE2E-003 |
| **Tên** | Lazy-load `WebConnect`/E2EE pairing module graph, không tải trong use case A |
| **Loại** | Performance / Bundle hygiene |
| **Priority** | P1 |
| **Phiên bản** | v5.1 |
| **Ngày tạo** | 2026-08-08 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-FE2E-001, CR-FE2E-002 |
| **Tác động HLD** | web-server-architecture.md §13 Build Configuration |

---

## 1. Vấn đề còn lại sau CR-FE2E-002

Sau CR-FE2E-002, `PairCodeFallback` không còn được gọi trong luồng multi-user — nhưng `main.tsx` vẫn **import tĩnh** cả hai nhánh (`bootstrapWebApp` VÀ `renderOriginalPairCodeApp`/`WebConnect`) ở đầu file, nên Vite vẫn đóng gói `WebConnect.tsx` → `web-runtime-client.ts` → `web-e2ee.ts` (TweetNaCl) vào **cùng 1 bundle** mà mọi browser tải, kể cả user chỉ bao giờ đi qua nhánh multi-user. Đây là phần "bỏ ở browser" còn thiếu — code vẫn chạy được (đúng, cần cho use case B) nhưng **không cần tải cho use case A**.

## 2. Thay đổi

### 2.1 Tách nhánh pairing thành dynamic import trong `main.tsx`

```diff
- import WebConnect from './WebConnect'
- import {
-   clearPairingInputFromAddressBar,
-   decideWebPairingStartup,
-   readPairingInputFromLocation
- } from './web-pairing'
- import {
-   createStoredWebRuntimeEnvironment,
-   readStoredWebRuntimeEnvironment,
-   saveStoredWebRuntimeEnvironment
- } from './web-runtime-environment'
- import { installWebPreloadApi } from './web-preload-api'
  ...
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
+ // Why: this branch only runs when /auth/config 404s (Desktop Pair Code
+ // sharing mode, CR-FE2E-003) — dynamic import keeps TweetNaCl + the E2EE
+ // pairing UI out of the bundle multi-user browsers actually download.
+ async function renderOriginalPairCodeApp(): Promise<void> {
+   const { mountPairCodeApp } = await import('./pair-code-app-entry')
+   mountPairCodeApp()
+ }
```

Tạo file mới `frontend/src/renderer/src/web/pair-code-app-entry.tsx` chứa nguyên vẹn `WebRoot`/`WebRootBoundary`/import `WebConnect`/`web-pairing`/`web-runtime-environment`/`installWebPreloadApi` — **copy y nguyên logic hiện có trong `main.tsx`**, không sửa hành vi, chỉ di chuyển để nó nằm sau 1 `import()` boundary.

### 2.2 `web-preload-api.ts` — giữ nguyên logic, xác nhận tree-shaking

`getRuntimeClientForEnvironment()` vẫn import cả `WebSessionClient` và `WebRuntimeClient` tĩnh — **không tách được** vì `installWebPreloadApi()` (dùng chung bởi cả 2 nhánh) cần cả 2. Chấp nhận: `WebRuntimeClient`/`web-e2ee.ts` vẫn nằm trong bundle chính qua đường này.

- [ ] Đo bundle size trước/sau bằng `pnpm --filter frontend build:web -- --report` (hoặc tool tương đương đã có trong `config/scripts/`).
- [ ] Nếu phần còn lại của `web-e2ee.ts` (qua `web-preload-api.ts`) vẫn đáng kể, ghi nhận thành **follow-up CR riêng** (tách `installWebPreloadApi` thành 2 biến thể theo nhánh) — **không** làm trong CR này để tránh trộn 2 rủi ro (dead-code splitting + client-selection refactor) trong 1 lần đổi.

### 2.3 KHÔNG đụng tới

- Hành vi runtime của use case B: `pair-code-app-entry.tsx` phải là **copy y nguyên**, không viết lại logic.
- Backend.

## 3. Rủi ro & Rollback

| Rủi ro | Giảm thiểu |
|---|---|
| Dynamic `import()` fail (network lỗi ngay lúc `/auth/config` 404) khiến use case B trắng trang | Thêm error boundary + retry giống `showErrorUi()` đã có cho nhánh multi-user |
| Test hiện tại của `main.tsx` mock import tĩnh, fail sau khi tách | Cập nhật test dùng `vi.mock('./pair-code-app-entry')` |
| Rollback | Revert 1 file (`main.tsx`) + xoá `pair-code-app-entry.tsx` — không đụng file nào khác nên rollback không rủi ro dây chuyền |

## 4. Acceptance Criteria

- [ ] Bundle JS tải khi `/auth/config` → 200 **không chứa** `web-e2ee.ts`/`web-runtime-client.ts`/`WebConnect.tsx` (kiểm bằng source-map-explorer hoặc tương đương).
- [ ] Bundle JS tải khi `/auth/config` → 404 **có đầy đủ** pairing UI, hành vi giống hệt trước CR này (test e2e Playwright cho use case B pass 100%).
- [ ] Không thay đổi `backend/`.
- [ ] Ghi lại số liệu bundle size trước/sau trong PR description.
