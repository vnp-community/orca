# CR-005 — Build System Separation

**Status:** Proposed  
**Priority:** 🟡 Medium  
**Depends on:** CR-002, CR-004  
**Blocks:** CR-006

---

## Mục tiêu

Tổ chức lại build system để hỗ trợ ba targets độc lập:
1. **`electron`** — Ứng dụng desktop Electron (hiện tại, giữ nguyên)
2. **`node-server`** — Backend Node.js thuần cho server deployment
3. **`web`** — React frontend dưới dạng static SPA (dùng WebSocket)

Đồng thời tách biệt scripts trong `package.json` để tránh nhầm lẫn.

---

## Bối cảnh & Vấn đề

Hiện tại build system có:
- `electron-vite` cho Electron (main + preload + renderer)
- `vite.web.config.ts` cho renderer web build  
- `vite.server.config.ts` cho backend Node.js build (được thêm vào)

Vấn đề:
- `pnpm build:web` build renderer cho Electron, **không phải** cho web deployment
- Không có target rõ ràng cho "deploy to server as web app"
- Docker build hiện tại phải build cả Electron main process (không cần thiết)
- Không thể build `web` target mà không build cả backend

---

## Giải pháp Đề xuất

### 1. Tái cấu trúc `package.json` scripts

```json
{
  "scripts": {
    // === Electron targets (giữ nguyên) ===
    "dev": "electron-vite dev",
    "build": "electron-vite build",
    "build:electron": "electron-vite build",
    
    // === Web/Node targets (mới) ===
    "build:backend": "vite build -c vite.server.config.ts",
    "build:frontend:web": "vite build -c vite.web-spa.config.ts",
    "build:web-app": "pnpm build:backend && pnpm build:frontend:web",
    
    // === Deploy targets ===
    "build:relay": "...(giữ nguyên)",
    "build:all": "pnpm build:electron && pnpm build:relay",
    "build:server": "pnpm build:relay && pnpm build:backend && pnpm build:frontend:web",
    
    // === Dev server (mới) ===
    "dev:backend": "node --watch out/server/index.js",
    "dev:web": "vite dev -c vite.web-spa.config.ts"
  }
}
```

### 2. Tạo `vite.web-spa.config.ts` — SPA Frontend Build

```typescript
// vite.web-spa.config.ts (FILE MỚI)
import { resolve } from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

/**
 * Web SPA Build Config
 * 
 * Builds the React frontend for standalone web deployment.
 * Output goes to out/web/ (same location as current build:web)
 * but uses web-index.html as entry (WebSocket RPC mode).
 * 
 * Không có Electron dependencies — dùng WebSocket cho IPC.
 */
export default defineConfig({
  root: 'src/renderer',
  
  plugins: [react(), tailwindcss()],
  
  build: {
    outDir: resolve(__dirname, 'out/web'),
    emptyOutDir: true,
    rollupOptions: {
      input: {
        'web-index': resolve(__dirname, 'src/renderer/web-index.html')
      }
    }
  },
  
  define: {
    'import.meta.env.ORCA_PLATFORM': JSON.stringify('web'),
    'import.meta.env.ORCA_MODE': JSON.stringify('web')
  },
  
  resolve: {
    alias: {
      '@renderer': resolve(__dirname, 'src/renderer/src'),
      '@': resolve(__dirname, 'src/renderer/src'),
      // Stub electron — không có trong web build
      'electron': resolve(__dirname, 'src/platform/stubs/electron-stub.ts')
    }
  },
  
  server: {
    // Dev server: proxy API calls đến local Orca backend
    proxy: {
      '/ws/runtime/api': {
        target: 'ws://localhost:6768',
        ws: true
      },
      '/api': {
        target: 'http://localhost:6768',
        changeOrigin: true
      }
    }
  }
})
```

### 3. Cập nhật `vite.server.config.ts`

```typescript
// vite.server.config.ts (UPDATE)
import { resolve } from 'path'
import { defineConfig } from 'vite'

export default defineConfig({
  build: {
    target: 'node22',
    ssr: true,
    outDir: 'out/server',
    lib: {
      entry: {
        index: resolve(__dirname, 'src/server/index.ts'),
        'daemon-entry': resolve(__dirname, 'src/main/daemon/daemon-entry.ts')
      },
      formats: ['cjs'],
      fileName: (format, entryName) => `${entryName}.js`
    },
    rollupOptions: {
      external: [
        // Node built-ins
        'node:fs', 'node:path', 'node:os', 'node:child_process',
        'node:crypto', 'node:events', 'node:net', 'node:tls',
        'node:http', 'node:https', 'node:stream', 'node:util',
        'node:buffer', 'node:process',
        // Legacy (no node: prefix)
        'fs', 'path', 'os', 'child_process', 'crypto', 'events',
        'net', 'tls', 'http', 'https', 'stream', 'util',
        // Native add-ons
        'node-pty', 'better-sqlite3', 'keytar', 'cpu-features',
        // Third-party (will be in node_modules)
        'sqlite3', 'express', 'ws', 'ssh2'
      ]
    }
  },
  resolve: {
    alias: {
      // Map 'electron' → platform adapter wrapper (not mock)
      'electron': resolve(__dirname, 'src/platform/stubs/electron-node-wrapper.ts'),
      '@xterm/headless': resolve(__dirname, 'node_modules/@xterm/headless/lib-headless/xterm-headless.js'),
      '@xterm/addon-serialize': resolve(__dirname, 'node_modules/@xterm/addon-serialize/lib/addon-serialize.js'),
      '@xterm/addon-unicode11': resolve(__dirname, 'node_modules/@xterm/addon-unicode11/lib/addon-unicode11.js')
    }
  }
})
```

### 4. `src/platform/stubs/electron-node-wrapper.ts`

```typescript
// src/platform/stubs/electron-node-wrapper.ts (FILE MỚI)
/**
 * Electron module stub for Node.js server builds.
 * 
 * Thay vì mock toàn bộ Electron API (như mocks/electron.ts hiện tại),
 * stub này delegate sang NodeAdapter thông qua platform context.
 * 
 * Điều này đảm bảo:
 * 1. Consistent behavior — không phải maintain 2 mock riêng
 * 2. Type safe — thông qua IPlatformServices interfaces
 * 3. Testable — có thể inject mock platform trong tests
 */
import { getPlatform } from '../context'
import { EventEmitter } from 'node:events'

// Lazy getters — chỉ resolve khi platform đã được init
export const app = new Proxy({} as any, {
  get(_target, prop) {
    try {
      return (getPlatform().app as any)[prop]
    } catch {
      // Platform not yet initialized — return no-op
      return typeof prop === 'string' && prop.startsWith('get')
        ? () => ''
        : () => {}
    }
  }
})

export const ipcMain = new Proxy({} as any, {
  get(_target, prop) {
    try {
      return (getPlatform().ipc as any)[prop]
    } catch {
      return () => {}
    }
  }
})

// BrowserWindow → NodeWindowManager.createWindow()
export class BrowserWindow extends EventEmitter {
  private _win: any
  
  constructor(options?: any) {
    super()
    try {
      this._win = getPlatform().windowManager.createWindow(options ?? {})
      // Proxy events from NodeWindow to this
      this._win.on('closed', () => this.emit('closed'))
    } catch {
      // Platform not ready
    }
  }
  
  get id() { return this._win?.id ?? 0 }
  get webContents() { return this._win ?? {} }
  
  isDestroyed() { return this._win?.isDestroyed() ?? true }
  isMinimized() { return this._win?.isMinimized() ?? false }
  isMaximized() { return this._win?.isMaximized() ?? false }
  isFullScreen() { return this._win?.isFullScreen() ?? false }
  isVisible() { return this._win?.isVisible() ?? true }
  isFocused() { return this._win?.isFocused() ?? true }
  
  show() { this._win?.show() }
  hide() { this._win?.hide() }
  focus() { this._win?.focus() }
  restore() { this._win?.restore() }
  close() { this._win?.close() }
  destroy() { this._win?.destroy() }
  
  loadURL(_url: string) { return Promise.resolve() }
  loadFile(_path: string) { return Promise.resolve() }
  
  static getAllWindows() {
    try {
      return getPlatform().windowManager.getAllWindows()
    } catch { return [] }
  }
  
  static getFocusedWindow() {
    try {
      return getPlatform().windowManager.getFocusedWindow()
    } catch { return null }
  }
  
  static fromId(_id: number) { return null }
  static fromWebContents(_wc: any) { return null }
}

// Other stubs (minimal, non-functional)
export const safeStorage = {
  isEncryptionAvailable: () => {
    try { return getPlatform().storage.isEncryptionAvailable() } catch { return false }
  },
  encryptString: (s: string) => {
    try { return getPlatform().storage.encryptString(s) } catch { return Buffer.from(s) }
  },
  decryptString: (b: Buffer) => {
    try { return getPlatform().storage.decryptString(b) } catch { return b.toString() }
  }
}

export const dialog = {
  showOpenDialog: async () => ({ canceled: true, filePaths: [] }),
  showSaveDialog: async () => ({ canceled: true }),
  showMessageBox: async () => ({ response: 0 })
}

export const nativeTheme = new EventEmitter()
;(nativeTheme as any).shouldUseDarkColors = true

export const shell = {
  openExternal: async (_url: string) => {},
  openPath: async (_p: string) => '',
  showItemInFolder: (_p: string) => {}
}

export const clipboard = {
  readText: () => '',
  writeText: (_text: string) => {}
}

export const Menu = {
  buildFromTemplate: () => ({ popup: () => {} }),
  setApplicationMenu: () => {},
  getApplicationMenu: () => null
}

export const session = {
  defaultSession: { 
    setPermissionRequestHandler: () => {},
    setPermissionCheckHandler: () => {},
    setDevicePermissionHandler: () => {},
    addWordToSpellCheckerDictionary: () => {},
    webRequest: { onBeforeSendHeaders: () => {} },
    getUserAgent: () => 'OrcaServer/1.0',
    clearStorageData: async () => {},
    clearCache: async () => {},
    on: () => {},
    removeListener: () => {}
  },
  fromPartition: () => session.defaultSession
}

export const powerMonitor = new EventEmitter()
export const autoUpdater = new EventEmitter()
export const powerSaveBlocker = {
  start: () => 1,
  stop: () => {},
  isStarted: () => false
}

export const nativeImage = {
  createEmpty: () => ({ isEmpty: () => true, resize: (o: any) => nativeImage.createEmpty(), toPNG: () => Buffer.from([]) }),
  createFromPath: () => nativeImage.createEmpty(),
  createFromBuffer: () => nativeImage.createEmpty(),
  createFromDataURL: () => nativeImage.createEmpty()
}

export const webContents = {
  getAllWebContents: () => [],
  getFocusedWebContents: () => null,
  fromId: () => null
}

export const systemPreferences = {
  getUserDefault: () => null,
  subscribeNotification: () => 0,
  unsubscribeNotification: () => {}
}

export const net = { isOnline: () => true }

export class Notification extends EventEmitter {
  constructor(_opts?: any) { super() }
  show() {}
  close() {}
}

export class BaseWindow extends EventEmitter {
  constructor() { super() }
}

export class WebContentsView {
  webContents: any = {}
}

export class Tray extends EventEmitter {
  constructor() { super() }
  setToolTip() {}
  setContextMenu() {}
  setImage() {}
}

export const screen = {
  getPrimaryDisplay: () => ({ workAreaSize: { width: 1920, height: 1080 }, scaleFactor: 1 }),
  getAllDisplays: () => [],
  on: () => {},
  removeListener: () => {}
}

// Default export (named exports above used by most code)
export default new Proxy({
  app, ipcMain, BrowserWindow, safeStorage, dialog, nativeTheme, shell,
  clipboard, Menu, Tray, session, powerMonitor, autoUpdater, powerSaveBlocker,
  nativeImage, webContents, systemPreferences, net, Notification, BaseWindow,
  WebContentsView, screen
}, {
  get(target, prop) {
    if (prop in target) return (target as any)[prop]
    console.warn(`[ElectronNodeWrapper] Accessed unimplemented property: ${String(prop)}`)
    return new Proxy({}, { get: () => () => {} })
  }
})
```

### 5. Output Directory Structure

```
out/
├── main/          # Electron main process (electron-vite)
├── preload/       # Electron preload scripts (electron-vite)  
├── renderer/      # Electron renderer (electron-vite, for Electron)
├── web/           # Web SPA (vite.web-spa.config.ts)
│   ├── web-index.html
│   └── assets/
├── server/        # Node.js backend (vite.server.config.ts)
│   ├── index.js
│   └── daemon-entry.js
└── relay/         # SSH relay binaries
    ├── linux-x64/
    └── ...
```

---

## Phạm vi thay đổi

### Files mới
| File | Mô tả |
|------|-------|
| `[NEW] vite.web-spa.config.ts` | SPA frontend build config |
| `[NEW] src/platform/stubs/electron-node-wrapper.ts` | Platform-aware electron stub |
| `[NEW] src/platform/stubs/electron-stub.ts` | Empty stub cho web browser build |

### Files sửa đổi
| File | Thay đổi |
|------|---------|
| `[MODIFY] package.json` | Thêm build scripts mới, tách build targets |
| `[MODIFY] vite.server.config.ts` | Dùng `electron-node-wrapper` thay vì `mocks/electron.ts` |

### Files KHÔNG thay đổi
- `electron.vite.config.ts` — Electron build config, giữ nguyên
- `src/main/` — **KHÔNG sửa**
- `src/renderer/src/main.tsx` — Electron entry, giữ nguyên

---

## CI/CD Considerations

```yaml
# .github/workflows/build.yml additions
jobs:
  build-electron:
    runs-on: ${{ matrix.os }}
    matrix:
      os: [macos-latest, ubuntu-latest, windows-latest]
    steps:
      - run: pnpm build:electron

  build-web-server:
    runs-on: ubuntu-latest
    steps:
      - run: pnpm build:server
      - uses: docker/build-push-action@v5
        with:
          context: .
          file: deploy/prod/Dockerfile
```

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

| File | Status |
|------|--------|
| `vite.web.config.ts` | ✅ Done — builds `out/web/` |
| `electron.vite.config.ts` | ✅ Done — Electron build config |
| `package.json` scripts | ✅ Done — `build:web`, `dev:web` etc. |
