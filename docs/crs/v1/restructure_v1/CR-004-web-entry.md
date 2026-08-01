# CR-004 — Web Frontend — HTTP/WebSocket Mode

**Status:** Proposed  
**Priority:** 🟠 High  
**Depends on:** CR-003  
**Blocks:** CR-005

---

## Mục tiêu

Cho phép React frontend (`src/renderer/`) hoạt động trong hai chế độ:
1. **Electron mode** (hiện tại): Dùng `window.electron.ipcRenderer` — **không thay đổi gì**
2. **Web mode** (mới): Dùng `WebSocketRpcClient` từ CR-003 — giao tiếp qua WebSocket

Nguyên tắc: **Thay đổi tối thiểu vào renderer code**, chỉ swap transport layer tại boundary.

---

## Bối cảnh & Vấn đề

Frontend hiện tại gọi backend qua pattern:
```typescript
// Ví dụ trong renderer component
const result = await window.electron.ipcRenderer.invoke('repos:list')
```

Hoặc qua custom hooks:
```typescript
// src/renderer/src/hooks/useIpc.ts (nếu có)
window.electron.ipcRenderer.on('repos:changed', callback)
```

Trong Web mode, `window.electron` không tồn tại — mọi call đều fail silently hoặc throw.

---

## Giải pháp Đề xuất

### 1. Platform Bridge — `src/renderer/src/platform/`

Tạo một abstraction layer trong renderer để swap transport:

```typescript
// src/renderer/src/platform/bridge.ts

import type { IRpcClient } from '../../../../platform/rpc-client-interface'

// Global instance — set during app initialization
let _bridge: IRpcClient | null = null

export function setBridge(client: IRpcClient): void {
  _bridge = client
}

export function getBridge(): IRpcClient {
  if (!_bridge) {
    throw new Error('Platform bridge not initialized')
  }
  return _bridge
}

// Convenience API matching existing usage patterns
export const bridge = {
  invoke: <T = any>(channel: string, ...args: any[]): Promise<T> => 
    getBridge().invoke(channel, ...args),
    
  send: (channel: string, ...args: any[]): void => 
    getBridge().send(channel, ...args),
    
  on: (channel: string, listener: (...args: any[]) => void): (() => void) => {
    return getBridge().on(channel, (_event, ...args) => listener(...args))
  },
  
  once: (channel: string, listener: (...args: any[]) => void): void => {
    getBridge().once(channel, (_event, ...args) => listener(...args))
  }
}
```

### 2. Khởi tạo Bridge tại App Root

```typescript
// src/renderer/src/main.tsx (MODIFY — thêm vài dòng)
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

// Platform bridge initialization
import { setBridge } from './platform/bridge'
import { createRpcClient } from '../../../platform/rpc-client-factory'

// Auto-detect mode and create appropriate client
const rpcClient = createRpcClient()

if (!rpcClient.isConnected()) {
  await rpcClient.connect()
}

setBridge(rpcClient)

// Render app after bridge is ready
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
```

### 3. Migration Path cho Renderer Code

**Giai đoạn 1 (CR-004)**: Tạo compatibility shim — không sửa existing renderer calls.

```typescript
// src/renderer/src/platform/electron-compat.ts
/**
 * Compatibility shim: exposes the same API as window.electron.ipcRenderer
 * but backed by our IRpcClient abstraction.
 * 
 * Mục đích: Cho phép existing renderer code chạy trong Web mode
 * mà KHÔNG cần sửa từng component.
 */
import { getBridge } from './bridge'

export function installElectronCompat(): void {
  if (typeof window === 'undefined') return
  
  // Nếu đang chạy trong Electron thật → bỏ qua
  if ((window as any).electron?.ipcRenderer) return
  
  // Tạo shim với cùng API như ipcRenderer
  const ipcRendererShim = {
    invoke: <T = any>(channel: string, ...args: any[]): Promise<T> =>
      getBridge().invoke(channel, ...args),
      
    send: (channel: string, ...args: any[]): void =>
      getBridge().send(channel, ...args),
      
    on: (channel: string, listener: (...args: any[]) => void): void => {
      getBridge().on(channel, (_event, ...args) => listener(...args))
    },
    
    once: (channel: string, listener: (...args: any[]) => void): void => {
      getBridge().once(channel, (_event, ...args) => listener(...args))
    },
    
    removeListener: (channel: string, listener: (...args: any[]) => void): void => {
      getBridge().off(channel, (_event, ...args) => listener(...args))
    }
  }
  
  // Install as window.electron.ipcRenderer
  ;(window as any).electron = {
    ipcRenderer: ipcRendererShim
  }
}
```

**Giai đoạn 2 (Future CRs)**: Từng bước migrate renderer code dùng `bridge.invoke()` trực tiếp.

### 4. Vite Build Configuration cho Web Mode

```typescript
// vite.web.config.ts (MODIFY)
import { resolve } from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  
  build: {
    outDir: 'out/web',
    rollupOptions: {
      input: {
        // Main web entry (dùng WebSocket RPC)
        'web-index': resolve(__dirname, 'src/renderer/web-index.html')
      }
    }
  },
  
  define: {
    // Compile-time flag để conditional import
    'import.meta.env.ORCA_PLATFORM': JSON.stringify('web')
  },
  
  resolve: {
    alias: {
      '@renderer': resolve('src/renderer/src'),
      '@': resolve('src/renderer/src'),
      // Stub out electron in web build
      'electron': resolve(__dirname, 'src/platform/stubs/electron-stub.ts')
    }
  }
})
```

### 5. `src/renderer/web-index.html` — Web Entry Point

```html
<!-- src/renderer/web-index.html (FILE MỚI) -->
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />
    <title>Orca Web</title>
    <!-- CSP cho web mode — không cần unsafe-eval như Electron -->
    <meta http-equiv="Content-Security-Policy" 
          content="default-src 'self'; connect-src 'self' ws: wss:; script-src 'self'">
  </head>
  <body>
    <div id="root"></div>
    <!-- Web mode entry — dùng WebSocket bridge -->
    <script type="module" src="./src/main-web.tsx"></script>
  </body>
</html>
```

### 6. `src/renderer/src/main-web.tsx` — Web Entry Script

```typescript
// src/renderer/src/main-web.tsx (FILE MỚI)
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

import { setBridge } from './platform/bridge'
import { installElectronCompat } from './platform/electron-compat'
import { WebSocketRpcClient } from '../../../platform/adapters/web/rpc-client'

async function main(): Promise<void> {
  // 1. Create WebSocket RPC client
  const client = new WebSocketRpcClient()
  
  // 2. Connect to backend
  try {
    await client.connect()
  } catch (err) {
    console.error('[Orca Web] Failed to connect to backend:', err)
    // Show error UI
    document.getElementById('root')!.innerHTML = `
      <div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:system-ui">
        <div>
          <h2>Cannot connect to Orca backend</h2>
          <p>Make sure the Orca server is running.</p>
          <button onclick="location.reload()">Retry</button>
        </div>
      </div>
    `
    return
  }
  
  // 3. Set bridge for React app
  setBridge(client)
  
  // 4. Install electron compat shim (for existing renderer code)
  installElectronCompat()
  
  // 5. Render app
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>
  )
}

main()
```

### 7. Connection Status Provider

```typescript
// src/renderer/src/platform/ConnectionStatusProvider.tsx (FILE MỚI)
import React, { createContext, useContext, useEffect, useState } from 'react'
import { getBridge } from './bridge'

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

const ConnectionContext = createContext<ConnectionStatus>('connecting')

export function ConnectionStatusProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<ConnectionStatus>('connecting')
  
  useEffect(() => {
    const bridge = getBridge()
    
    // Poll connection status
    const interval = setInterval(() => {
      setStatus(bridge.isConnected() ? 'connected' : 'disconnected')
    }, 1000)
    
    // Initial check
    setStatus(bridge.isConnected() ? 'connected' : 'connecting')
    
    return () => clearInterval(interval)
  }, [])
  
  return (
    <ConnectionContext.Provider value={status}>
      {children}
    </ConnectionContext.Provider>
  )
}

export function useConnectionStatus(): ConnectionStatus {
  return useContext(ConnectionContext)
}
```

---

## Phạm vi thay đổi

### Files mới
| File | Mô tả |
|------|-------|
| `[NEW] src/renderer/src/platform/bridge.ts` | IRpcClient singleton trong renderer |
| `[NEW] src/renderer/src/platform/electron-compat.ts` | Compatibility shim |
| `[NEW] src/renderer/src/platform/ConnectionStatusProvider.tsx` | Connection status |
| `[NEW] src/renderer/src/main-web.tsx` | Web mode entry script |
| `[NEW] src/renderer/web-index.html` | Web mode HTML entry |
| `[NEW] src/platform/stubs/electron-stub.ts` | Empty stub for web build |

### Files sửa đổi (minimal)
| File | Thay đổi |
|------|---------|
| `[MODIFY] vite.web.config.ts` | Thêm web entry point config |
| `[MODIFY] vite.server.config.ts` | Build web entry mới vào `out/web/` |

### Files KHÔNG thay đổi
- `src/renderer/src/main.tsx` — Electron entry, giữ nguyên
- `src/renderer/src/App.tsx` — Giữ nguyên
- `src/renderer/src/**/*.tsx` — Tất cả component giữ nguyên
- `src/preload/` — Giữ nguyên
- `src/main/` — **KHÔNG sửa**

---

## Feature Flags & Conditional Rendering

Một số tính năng chỉ có ở Electron (desktop notifications, native menus...). Dùng feature detection:

```typescript
// src/renderer/src/platform/features.ts
export const PlatformFeatures = {
  // Electron-only features
  nativeNotifications: !!(window as any).electron,
  nativeMenu: !!(window as any).electron,
  systemTray: !!(window as any).electron,
  fileDropzone: !!(window as any).electron || 'showOpenFilePicker' in window,
  
  // Web-only features
  browserShare: 'share' in navigator,
  
  // Universal features (backed by RPC)
  terminalPty: true,
  sshConnections: true,
  gitOperations: true,
  aiAgents: true,
  fileSystem: true  // via IPC/RPC handlers
}
```

---

## Rủi ro & Biện pháp

| Rủi ro | Biện pháp |
|--------|-----------|
| Electron compat shim bị dùng mãi, không migrate | Thêm dev warning khi shim được invoke |
| CSP blocks WebSocket | Cấu hình CSP đúng trong HTML meta và nginx |
| CORS issues | Server phải set `Access-Control-Allow-Origin` header |
| React hot reload in dev mode | Dùng Vite HMR qua WebSocket riêng (không conflict) |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

| File | Status |
|------|--------|
| `src/renderer/src/web/main-web-bootstrap.tsx` | ✅ Done — `bootstrapWebApp()` + `WebRoot` |
| `src/renderer/src/web/ConnectionStatusProvider.tsx` | ✅ Done |
| `src/renderer/src/web/ConnectionStatusBanner.tsx` | ✅ Done |
| `src/renderer/src/web/web-preload-api.ts` | ✅ Done (existing, kept) |
| Tests: 34/34 pass | ✅ Done |
