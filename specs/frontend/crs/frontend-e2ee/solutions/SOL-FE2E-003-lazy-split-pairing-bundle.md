# SOL-FE2E-003 — Code-split E2EE Pairing — Giải pháp implement (tái dùng `lazyWithRetry` có sẵn)

**CR:** [CR-FE2E-003](../../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-003-lazy-split-pairing-bundle.md)
**TDD Refs:** [TDD-FE-06 §9](../../../tdd/v4/06-web-entry.md#L151) (`ORCA_PLATFORM` flag, build-time conditional code), [TDD-FE-03 §"restructure_v1 Addendum"](../../../tdd/v5/03-runtime-client-layer.md#L364) (`IRpcClient`/`WebRuntimeClient` layering)
**Approach:** Refinement — CR đã có kế hoạch đúng hướng (`await import()` thủ công); solution này thay bằng tiện ích **đã có sẵn trong codebase** (`lazyWithRetry`) để không phải tự viết lại cơ chế retry/rollback mà CR tự liệt kê là rủi ro.

---

## 1. Phát hiện quan trọng — codebase đã có sẵn tiện ích cho đúng vấn đề CR-FE2E-003 lo ngại

CR-FE2E-003 mục 3 "Rủi ro & Rollback" liệt kê: *"Dynamic `import()` fail (network lỗi ngay lúc `/auth/config` 404) khiến use case B trắng trang"* — và đề xuất tự thêm error boundary + retry.

**Không cần tự viết** — `frontend/src/renderer/src/lib/lazy-with-retry.ts` đã tồn tại, đang được `main-web-bootstrap.tsx` dùng NGAY CHO CHÍNH `WebConnect`:

```ts
// main-web-bootstrap.tsx (thật, dòng 5, 39)
import { lazyWithRetry as lazy } from '@/lib/lazy-with-retry'
...
const WebConnect = lazy(() => import('./WebConnect'))
```

`lazyWithRetry()` (file `lazy-with-retry.ts:165-170`) bọc `React.lazy` với cơ chế: phát hiện lỗi tải chunk đã biết (`isKnownDynamicImportFailure`), thử `window.location.reload()` **một lần** để lấy asset manifest mới (trường hợp deploy mới làm URL chunk cũ 404), rồi mới throw nếu vẫn fail. Đây **chính xác** là loại resilience CR-FE2E-003 đang tự đề xuất viết lại.

**Kết luận:** `main.tsx` nên dùng cùng `lazyWithRetry` + `React.lazy`/`Suspense`, không phải tự viết `async function renderOriginalPairCodeApp() { await import(...) }` trần như CR mô tả — vừa nhất quán với `main-web-bootstrap.tsx` (cùng thư mục `web/`, cùng vấn đề), vừa có sẵn resilience mà không cần code mới.

## 2. Diff — điều chỉnh so với CR

### 2.1 `pair-code-app-entry.tsx` (file mới, đúng như CR đề xuất — giữ nguyên tên/vị trí)

```tsx
// frontend/src/renderer/src/web/pair-code-app-entry.tsx
// Why: copy nguyên vẹn WebRoot/WebRootBoundary hiện có trong main.tsx (CR-FE2E-003)
// — logic hành vi KHÔNG đổi, chỉ di chuyển sau 1 code-split boundary.
import React, { Suspense, useMemo, useState } from 'react'
import { lazyWithRetry as lazy } from '@/lib/lazy-with-retry'
import ReactDOM from 'react-dom/client'
import { useTranslation } from 'react-i18next'
import { RecoverableRenderErrorBoundary } from '../components/error-boundaries/RecoverableRenderErrorBoundary'
import {
  clearPairingInputFromAddressBar,
  decideWebPairingStartup,
  readPairingInputFromLocation
} from './web-pairing'
import {
  createStoredWebRuntimeEnvironment,
  readStoredWebRuntimeEnvironment,
  saveStoredWebRuntimeEnvironment
} from './web-runtime-environment'
import { installWebPreloadApi } from './web-preload-api'
import { I18nProvider } from '../i18n/I18nProvider'
import { translate } from '../i18n/i18n'

// Why: lazyWithRetry (not a bare `lazy(() => import(...))`) — matches
// main-web-bootstrap.tsx's own use of it for the exact same component, and
// gives one free-reload-on-chunk-failure retry before surfacing an error,
// covering the "dynamic import fails right when /auth/config 404s" risk
// CR-FE2E-003 flagged instead of hand-rolling retry logic here.
const WebConnect = lazy(() => import('./WebConnect'))

function WebRoot(): React.JSX.Element {
  // ... (copy y nguyên thân hàm WebRoot hiện có trong main.tsx, không đổi 1 dòng logic)
  const initialPairingInput = useMemo(() => readPairingInputFromLocation(window.location), [])
  const startupDecision = useMemo(() => {
    const decision = decideWebPairingStartup({
      initialPairingInput,
      hasStoredEnvironment: readStoredWebRuntimeEnvironment() !== null
    })
    if (
      decision.kind === 'auto-save-runtime-offer' ||
      (decision.kind === 'show-connect' && decision.initialPairingInput !== null)
    ) {
      clearPairingInputFromAddressBar()
    }
    return decision
  }, [initialPairingInput])
  const [hasEnvironment, setHasEnvironment] = useState(() => {
    if (startupDecision.kind === 'auto-save-runtime-offer') {
      saveStoredWebRuntimeEnvironment(
        createStoredWebRuntimeEnvironment({ name: 'Orca Server', offer: startupDecision.offer })
      )
      return true
    }
    return startupDecision.kind === 'use-stored-environment'
  })

  if (!hasEnvironment) {
    return (
      <Suspense fallback={<div className="min-h-dvh bg-background" />}>
        <WebConnect
          initialPairingInput={
            startupDecision.kind === 'show-connect' ? startupDecision.initialPairingInput : null
          }
          onConnected={() => setHasEnvironment(true)}
        />
      </Suspense>
    )
  }

  installWebPreloadApi()
  const App = lazy(() => import('../App'))
  return (
    <Suspense fallback={<div className="min-h-dvh bg-background" />}>
      <App />
    </Suspense>
  )
}

function WebRootBoundary(): React.JSX.Element {
  useTranslation()
  return (
    <RecoverableRenderErrorBoundary
      boundaryId="web.root"
      surface="web-root"
      title={translate('app.recoverableError.webTitle', 'Orca web hit a renderer error.')}
      description={translate(
        'app.recoverableError.webDescription',
        'Retry the web client or reconnect to the paired runtime.'
      )}
    >
      <WebRoot />
    </RecoverableRenderErrorBoundary>
  )
}

export function mountPairCodeApp(): void {
  const rootEl = document.getElementById('root')
  if (!rootEl) {
    return
  }
  ReactDOM.createRoot(rootEl).render(
    <I18nProvider>
      <WebRootBoundary />
    </I18nProvider>
  )
}
```

> [!IMPORTANT]
> `App` được `lazy()`-load bên trong `WebRoot` thay vì import tĩnh — đúng theo pattern `main-web-bootstrap.tsx` đã làm (dòng 40: `const App = lazy(() => import('../App'))`), giữ nhất quán 2 file, và giảm thêm kích thước chunk ban đầu của `pair-code-app-entry.tsx`.

### 2.2 `main.tsx` — thay import tĩnh bằng dynamic import trỏ tới file trên

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
  ... (xoá toàn bộ định nghĩa WebRoot/WebRootBoundary khỏi main.tsx — đã chuyển sang pair-code-app-entry.tsx)

+ // Why: only /auth/config 404 (Desktop Pair Code sharing, CR-FE2E-003) reaches
+ // this branch — dynamic import keeps TweetNaCl + the E2EE pairing UI out of
+ // the bundle every multi-user browser downloads.
  function renderOriginalPairCodeApp(): void {
+   void import('./pair-code-app-entry').then(({ mountPairCodeApp }) => mountPairCodeApp())
  }
```

**Khác 1 chi tiết so với CR:** CR đề xuất `async function renderOriginalPairCodeApp(): Promise<void>` với `await import(...)` trực tiếp. Solution này dùng `void import(...).then(...)` (không `async/await`) để giữ `renderOriginalPairCodeApp` là hàm đồng bộ như hiện tại (gọi trong `.then()`/`.catch()` của `fetch('/auth/config')` ở `main.tsx` — không cần thay đổi call site đó sang `async`). Tương đương về hành vi, ít thay đổi hơn ở call site.

## 3. Bundle size — cách đo cụ thể

CR mục 2.2 để checklist "đo bundle size" mở, chưa có lệnh cụ thể. Bổ sung:

```bash
cd frontend
pnpm build   # vite build — tạo out/web/assets/*.js với tên file kèm hash
ls -la out/web/assets/*.js | sort -k5 -n
# So sánh: trước CR, tìm chunk chứa "tweetnacl"/"nacl" trong tên hoặc nội dung —
# nếu đã nằm trong entry chunk chính (không tách riêng), kích thước entry chunk
# giảm sau khi áp CR này. Xác nhận bằng:
grep -l "nacl" out/web/assets/*.js
# Kỳ vọng SAU CR: chỉ chunk pair-code-app-entry-*.js chứa "nacl", không phải
# chunk entry chính (web-index-*.js hoặc tương đương).
```

## 4. Test Specifications

```ts
// frontend/src/renderer/src/web/__tests__/main-web.test.ts (hoặc file test main.tsx hiện có)
vi.mock('./pair-code-app-entry', () => ({
  mountPairCodeApp: vi.fn()
}))

it('dynamically imports pair-code-app-entry only when /auth/config 404s', async () => {
  mockFetch.mockResolvedValueOnce({ ok: false, status: 404 })
  await import('../main') // hoặc trigger tương đương theo cấu trúc test hiện có
  const { mountPairCodeApp } = await import('./pair-code-app-entry')
  expect(mountPairCodeApp).toHaveBeenCalled()
})

it('does NOT import pair-code-app-entry when /auth/config returns 200', async () => {
  mockFetch.mockResolvedValueOnce({ ok: true, status: 200 })
  const importSpy = vi.spyOn(await import('./pair-code-app-entry'), 'mountPairCodeApp')
  await import('../main')
  expect(importSpy).not.toHaveBeenCalled()
})
```

## Acceptance Criteria — Kết quả kỳ vọng

| # | Criteria (từ CR) | Điều chỉnh |
|---|---|---|
| AC-1 | Bundle 200-case không chứa `web-e2ee.ts`/`web-runtime-client.ts`/`WebConnect.tsx` | Giữ nguyên, verify bằng lệnh mục 3 |
| AC-2 | Bundle 404-case đầy đủ pairing UI, hành vi giống hệt | Giữ nguyên — `pair-code-app-entry.tsx` là copy nguyên vẹn, cộng thêm `lazyWithRetry` cho `WebConnect`/`App` (vốn `main-web-bootstrap.tsx` đã làm cho các component tương đương — hành vi runtime, không phải chỉ "giống hệt" mà còn **nhất quán hơn** với nhánh use case A) |
| AC-3 | Không đổi `backend/` | Giữ nguyên |
| AC-4 | Ghi số liệu bundle size trước/sau trong PR | Giữ nguyên, có lệnh cụ thể ở mục 3 |
