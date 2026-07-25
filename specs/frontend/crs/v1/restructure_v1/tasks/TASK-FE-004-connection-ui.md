# TASK-FE-004 — Implement Connection Status UI

**Source Solution:** [SOL-FE-003](../solutions/SOL-FE-003-connection-ui.md)  
**Priority:** P1 — Phụ thuộc TASK-FE-001 (IRpcClient interface)  
**Loại:** Tạo files mới (React components)  
**Depends on:** TASK-FE-001 (IRpcClient type)

---

## Context

Web mode cần hiển thị trạng thái kết nối tới Orca backend. Khi WebSocket disconnect, app không crash — chỉ hiện banner nhỏ ở góc màn hình với nút Retry. Connection state được quản lý qua React Context (không dùng Zustand — không cần persist).

**Web-only feature**: Electron mode không import các files này. Tree-shaking sẽ exclude chúng khỏi Electron bundle.

---

## Input

Đọc trước khi implement:
- `src/platform/rpc-client-interface.ts` — IRpcClient interface (từ TASK-FE-001)
- Xem Sonner đã được dùng ở đâu trong codebase để import đúng cách

---

## Output — Files cần tạo

### File 1: `src/renderer/src/web/ConnectionStatusProvider.tsx` [TẠO MỚI]

#### Exports cần có

```typescript
export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

// Props
export interface ConnectionStatusProviderProps {
  children: React.ReactNode
  client: IRpcClient
  pollIntervalMs?: number  // default: 2000
}

// Component
export function ConnectionStatusProvider(props: ConnectionStatusProviderProps): JSX.Element

// Hooks
export function useConnectionStatus(): ConnectionStatus
export function useConnectionClient(): IRpcClient | null
export function useConnectionRetry(): () => void
```

#### Implementation logic

1. **State**: `useState<ConnectionStatus>` — init từ `client.isConnected() ? 'connected' : 'connecting'`
2. **Poll**: `setInterval(updateStatus, pollIntervalMs)` — gọi `setStatus(client.isConnected() ? 'connected' : 'disconnected')`
3. **retry()**: async function — `setStatus('connecting')` → `client.connect()` → `setStatus('connected')` / `catch → setStatus('disconnected')`
4. **Context default**: `{ status: 'connecting', client: null, retry: () => {} }`
5. **Children always render** — không block render khi disconnected
6. **Reconnect toast**: Khi `prevStatus === 'disconnected' && status === 'connected'`, gọi `toast.success('Reconnected to Orca backend', { duration: 3000 })`

#### Sonner toast integration

```typescript
import { toast } from 'sonner'

// Trong useEffect khi status thay đổi:
useEffect(() => {
  if (prevStatus === 'disconnected' && status === 'connected') {
    toast.success('Reconnected to Orca backend', { duration: 3000 })
  }
}, [status])
```

Để track `prevStatus`, dùng `useRef`:
```typescript
const prevStatusRef = useRef(status)
useEffect(() => {
  prevStatusRef.current = status
})
```

---

### File 2: `src/renderer/src/web/ConnectionStatusBanner.tsx` [TẠO MỚI]

#### Props

```typescript
export interface ConnectionStatusBannerProps {
  status: ConnectionStatus
  onRetry: () => void
}

export function ConnectionStatusBanner(props: ConnectionStatusBannerProps): JSX.Element | null
```

#### Behavior

- `status === 'connected'` → **return null** (invisible)
- `status === 'connecting'` → hiện banner vàng (#f59e0b) với spinner + text "Connecting to Orca backend..."
- `status === 'disconnected'` → hiện banner đỏ (#ef4444) với "⚠ Connection lost" + Retry button

#### Style yêu cầu (inline styles — không dùng CSS module)

```
position: fixed
bottom: 16px
right: 16px
z-index: 9999
border-radius: 8px
padding: 10px 16px
display: flex
align-items: center
gap: 10px
box-shadow: 0 4px 12px rgba(0,0,0,0.3)
font-size: 14px
color: white
```

#### Accessibility requirements

- Root element: `role="alert"` + `aria-live="polite"`
- Spinner element: `role="status"` + `aria-busy="true"` + className `animate-spin`
- Retry button: text "Retry" (để screen readers đọc được)

#### Spinner element

```tsx
<span role="status" aria-busy="true" className="animate-spin">↻</span>
```

#### Retry button style

```
background: rgba(255,255,255,0.2)
border: 1px solid rgba(255,255,255,0.4)
color: white
padding: 4px 10px
border-radius: 4px
cursor: pointer
font-size: 12px
```

---

## Acceptance Criteria

| # | Criteria | Verify bằng |
|---|----------|-------------|
| AC-1 | Provider cung cấp "connected" khi `client.isConnected() = true` | unit test |
| AC-2 | Provider cung cấp "disconnected" khi client disconnect | unit test |
| AC-3 | Status update theo poll interval | unit test (real timers) |
| AC-4 | Children render dù disconnected | unit test |
| AC-5 | Banner invisible khi connected | unit test |
| AC-6 | Banner hiện khi disconnected với Retry button | unit test |
| AC-7 | Banner hiện khi connecting với spinner | unit test |
| AC-8 | Retry button gọi `client.connect()` | unit test |
| AC-9 | `role="alert"` present khi disconnected | unit test |
| AC-10 | Reconnect toast hiện khi connect restore | unit test (mock sonner) |
| AC-11 | Không import file này trong Electron bundle | build check |

---

## Constraints

- **KHÔNG** add connection state vào Zustand store
- **KHÔNG** dùng CSS modules — chỉ inline styles
- **KHÔNG** import Electron modules
- `useConnectionStatus()` phải return `ConnectionStatus` type (không phải string)
- Sonner mock trong tests: `vi.mock('sonner', () => ({ toast: { success: vi.fn(), warning: vi.fn() } }))`

---

## Notes

Xem implementation guide chi tiết tại:
`specs/frontend/crs/v1/restructure_v1/solutions/SOL-FE-003-connection-ui.md` §4

---

## Execution Status

**Status:** ✅ DONE  
**Date:** 2026-07-23  
**Files Created:**
- `src/renderer/src/web/ConnectionStatusProvider.tsx` — React context với `useConnectionStatus()`, `useConnectionClient()`, `useConnectionRetry()`
- `src/renderer/src/web/ConnectionStatusBanner.tsx` — Fixed-position banner component
- `src/renderer/src/web/__tests__/ConnectionStatusProvider.test.tsx` — 5 test cases (happy-dom)
- `src/renderer/src/web/__tests__/ConnectionStatusBanner.test.tsx` — 6 test cases (happy-dom)
