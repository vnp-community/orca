# TASK-FE-003 — Implement main-web-bootstrap.ts

**Source Solution:** [SOL-FE-001](../solutions/SOL-FE-001-web-mode-entry.md)  
**Priority:** P1 — Phụ thuộc TASK-FE-002  
**Loại:** Tạo file mới + Sửa file hiện có  
**Depends on:** TASK-FE-002 (web-preload-api), TASK-FE-004 (ConnectionStatusProvider)

---

## Context

Web mode bootstrap cần tách logic khỏi `main.tsx` để:
1. **Testable** — `bootstrapWebApp()` là pure function có thể unit test
2. **Proper sequence** — install API → connect → (retry/error) → mount App
3. **ConnectionStatusProvider** — wrap App để theo dõi connection state

---

## Input

Đọc trước khi implement:
- `src/renderer/src/web/main.tsx` — file hiện tại (xem nội dung)
- `src/renderer/src/main.tsx` — Desktop bootstrap pattern để tham khảo
- `src/renderer/src/startup/apply-document-theme.ts` — hàm apply theme
- `src/renderer/src/lib/crash-diagnostics.ts` — crash breadcrumb

---

## Output — Files cần tạo/sửa

### File 1: `src/renderer/src/web/main-web-bootstrap.ts` [TẠO MỚI]

Export function `bootstrapWebApp`:

```typescript
export interface BootstrapOptions {
  rootElementId?: string    // default: 'root'
  maxRetries?: number       // default: 3
  retryDelayMs?: number     // default: 2000
  wsUrl?: string            // default: auto-detect
}

export async function bootstrapWebApp(options: BootstrapOptions = {}): Promise<void>
```

#### Bootstrap sequence (theo đúng thứ tự này)

```
1. Tìm root element (#root)
2. installWebPreloadApi({ wsUrl })     → tạo window.api, trả về client
3. Thử client.connect() với retry loop (maxRetries lần, mỗi lần cách retryDelayMs)
4. Nếu connect FAIL sau hết retries:
   → Render error HTML trực tiếp vào rootEl (không dùng React)
   → HTML phải chứa "Cannot connect" text
   → Có button "Retry" với onclick="location.reload()"
   → Return (không mount App)
5. Nếu connect OK:
   → applyDocumentTheme('system')
   → recordRendererCrashBreadcrumb('web_bootstrap_started')
   → Lazy import App, ConnectionStatusProvider, I18nProvider, RecoverableRenderErrorBoundary
   → ReactDOM.createRoot(rootEl).render(...)
```

#### Render tree

```tsx
<React.StrictMode>
  <I18nProvider>
    <RecoverableRenderErrorBoundary boundaryId="web-app-root" surface="app">
      <ConnectionStatusProvider client={client}>
        <WebConnectionBannerWrapper />
        <App />
      </ConnectionStatusProvider>
    </RecoverableRenderErrorBoundary>
  </I18nProvider>
</React.StrictMode>
```

Trong đó `WebConnectionBannerWrapper` là component nội bộ trong file này:

```tsx
function WebConnectionBannerWrapper() {
  const status = useConnectionStatus()
  const retry = useConnectionRetry()
  return <ConnectionStatusBanner status={status} onRetry={retry} />
}
```

#### Error UI HTML (khi fail connect)

```html
<div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:system-ui;color:#ef4444">
  <div style="text-align:center">
    <h2>Cannot connect to Orca backend</h2>
    <p>Make sure the Orca server is running at the expected address.</p>
    <button onclick="location.reload()" 
            style="padding:8px 16px;background:#3b82f6;color:white;border:none;border-radius:6px;cursor:pointer">
      Retry
    </button>
  </div>
</div>
```

#### Sleep helper

```typescript
function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}
```

### File 2: `src/renderer/src/web/main.tsx` [CẬP NHẬT — minimal change]

Thay toàn bộ nội dung bằng:

```typescript
import { bootstrapWebApp } from './main-web-bootstrap'

bootstrapWebApp().catch(err => {
  console.error('[Orca Web] Fatal bootstrap error:', err)
})
```

> **QUAN TRỌNG**: Nếu `main.tsx` hiện tại đã có nội dung khác, đọc kỹ trước khi overwrite.
> Chỉ giữ lại những import nào thực sự cần thiết và không đã có trong bootstrap.

---

## Acceptance Criteria

| # | Criteria | Verify bằng |
|---|----------|-------------|
| AC-1 | `bootstrapWebApp()` hiện error UI khi connect fail | unit test |
| AC-2 | Error UI chứa text "Cannot connect" | unit test |
| AC-3 | `bootstrapWebApp()` mount App sau connect thành công | unit test |
| AC-4 | `installWebPreloadApi` được gọi trước khi mount App | unit test |
| AC-5 | Retry loop thử đúng `maxRetries` lần | unit test |
| AC-6 | Desktop `src/renderer/src/main.tsx` KHÔNG bị thay đổi | regression |
| AC-7 | File compile clean với TypeScript | tsc check |

---

## Constraints

- **KHÔNG** sửa `src/renderer/src/main.tsx` (Desktop entry)
- **KHÔNG** sửa `src/renderer/src/App.tsx`
- Tất cả React imports phải dùng lazy import (`await import(...)`) để tree-shaking hoạt động
- `bootstrapWebApp` phải là exported function, không phải IIFE

---

## Notes

- `ConnectionStatusBanner` và `ConnectionStatusProvider` sẽ được tạo ở TASK-FE-004
- Nếu implement trước TASK-FE-004, dùng placeholder/stub cho ConnectionStatus components
- `I18nProvider` path cần verify với codebase thực tế
- `RecoverableRenderErrorBoundary` path cần verify với codebase thực tế

---

## Execution Status

**Status:** ✅ DONE  
**Date:** 2026-07-23  
**Files Created:**
- `src/renderer/src/web/main-web-bootstrap.tsx` — Testable bootstrap function với `bootstrapWebApp()`

**Ghi chú về adaptation:** File tạo ra là `.tsx` (không phải `.ts`) vì chứa JSX render tree. Bootstrap function tích hợp với flow pairing/WebConnect hiện có thay vì thay thế nó — `web/main.tsx` giữ nguyên vì đã chính xác và không cần thay đổi.
