# TASK-015 + TASK-016: Cập nhật Vite build configs

**Source:** SOL-BE-005  
**Phase:** 3 | **Effort:** S (45–60 min)  
**Depends on:** TASK-010 (electron-node-wrapper phải tồn tại)

---

## Objective

TASK-015: Cập nhật `vite.server.config.ts` để dùng `electron-node-wrapper.ts` thay vì `mocks/electron.ts`.

TASK-016: Tạo `vite.web-spa.config.ts` — Vite config riêng để build frontend web bundle (`out/web/`).

---

## TASK-015: Cập nhật `vite.server.config.ts`

### Context cần đọc trước

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
cat vite.server.config.ts
```

### Thay đổi cần thực hiện

**Tìm đoạn alias electron:**
```typescript
// TRƯỚC:
alias: {
  electron: resolve(__dirname, 'src/main/mocks/electron.ts')
  // hoặc tương tự
}
```

**Thay bằng:**
```typescript
// SAU:
alias: {
  electron: resolve(__dirname, 'src/platform/stubs/electron-node-wrapper.ts')
}
```

**Verify các mục khác trong `vite.server.config.ts`:**

File phải có các thành phần sau (thêm nếu chưa có):

```typescript
import { defineConfig, UserConfig } from 'vite'
import { resolve } from 'node:path'

export default defineConfig({
  build: {
    target: 'node22',
    outDir: 'out/server',
    ssr: true,          // Không bundle node_modules cơ bản
    lib: {
      entry: {
        index: resolve(__dirname, 'src/server/index.ts'),
        'daemon-entry': resolve(__dirname, 'src/main/daemon/daemon-entry.ts')
      },
      formats: ['cjs']
    },
    rollupOptions: {
      external: [
        // Native modules — không thể bundle
        'node-pty',
        'better-sqlite3',
        'keytar',
        'ssh2',
        'fsevents',
        // Node built-ins
        /^node:/,
        // All node_modules (SSR mode)
        /^[^.]/
      ],
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: '[name].js',
      }
    }
  },
  resolve: {
    alias: {
      // Alias 'electron' → NodeAdapter wrapper (KEY CHANGE)
      electron: resolve(__dirname, 'src/platform/stubs/electron-node-wrapper.ts'),
    }
  },
  define: {
    'process.env.ORCA_PLATFORM': JSON.stringify('node'),
  }
}) satisfies UserConfig
```

---

## TASK-016: Tạo `vite.web-spa.config.ts`

### File to create

**`vite.web-spa.config.ts`** (new file tại root):

```typescript
/**
 * Vite config for building the Web SPA bundle.
 *
 * Input:  src/renderer/web-index.html
 * Output: out/web/
 *
 * This build target:
 * - Stubs out 'electron' with a no-op shim (no real Electron IPC)
 * - Defines ORCA_PLATFORM='web' for conditional code paths
 * - Configures dev server to proxy WebSocket to local backend (:6768)
 *
 * Usage:
 *   pnpm build:frontend:web
 *   vite --config vite.web-spa.config.ts (dev mode)
 */
import { defineConfig, UserConfig } from 'vite'
import { resolve } from 'node:path'
import react from '@vitejs/plugin-react'

export default defineConfig({
  root: resolve(__dirname, 'src/renderer'),

  plugins: [react()],

  resolve: {
    alias: {
      // Stub Electron APIs with no-ops in web mode
      electron: resolve(__dirname, 'src/platform/stubs/electron-web-stub.ts'),
      // Alias @ to renderer src (same as electron-vite config)
      '@': resolve(__dirname, 'src/renderer/src'),
    }
  },

  define: {
    'import.meta.env.ORCA_PLATFORM': JSON.stringify('web'),
    'process.env.ORCA_PLATFORM': JSON.stringify('web'),
  },

  build: {
    outDir: resolve(__dirname, 'out/web'),
    emptyOutDir: true,
    rollupOptions: {
      input: {
        // Entry point is web-index.html, not index.html
        'web-index': resolve(__dirname, 'src/renderer/web-index.html'),
      },
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      }
    }
  },

  server: {
    port: 5174,  // Different from Electron dev port (5173)
    proxy: {
      // Proxy WebSocket to local backend in dev mode
      '/ws': {
        target: 'ws://localhost:6768',
        ws: true,
        changeOrigin: true
      },
      // Proxy API calls if needed
      '/api': {
        target: 'http://localhost:6768',
        changeOrigin: true
      }
    }
  }
}) satisfies UserConfig
```

### Cần tạo thêm: `src/platform/stubs/electron-web-stub.ts`

```typescript
/**
 * electron-web-stub.ts — No-op stub for web browser mode.
 *
 * In web mode, Electron APIs are not available at all.
 * This stub ensures imports don't crash.
 * window.api is provided by web-preload-api.ts instead.
 */

export const app = {
  getVersion: () => '0.0.0',
  getPath: () => '',
  isPackaged: false,
  whenReady: () => Promise.resolve(),
  quit: () => {},
  on: () => {},
  off: () => {},
}

export const ipcMain = {
  handle: () => {},
  removeHandler: () => {},
  on: () => {},
  off: () => {},
}

export class BrowserWindow {
  constructor(_opts?: any) {}
  static getAllWindows() { return [] }
  static getFocusedWindow() { return null }
  webContents = { send: () => {} }
  on() { return this }
  off() { return this }
}

export const shell = {
  openExternal: () => Promise.resolve(),
  openPath: () => Promise.resolve(''),
}

export const dialog = {
  showOpenDialog: () => Promise.resolve({ canceled: true, filePaths: [] }),
  showSaveDialog: () => Promise.resolve({ canceled: true, filePath: '' }),
}

export const safeStorage = {
  isEncryptionAvailable: () => false,
  encryptString: (s: string) => Buffer.from(s),
  decryptString: (b: Buffer) => b.toString(),
}

export const nativeTheme = {
  shouldUseDarkColors: false,
  themeSource: 'system',
  on: () => {},
}

export const clipboard = {
  readText: () => '',
  writeText: () => {},
}

export default {
  app, ipcMain, BrowserWindow, shell, dialog, safeStorage, nativeTheme, clipboard
}
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TASK-015: Verify server config uses electron-node-wrapper
grep "electron-node-wrapper\|mocks/electron" vite.server.config.ts
# Expected: electron-node-wrapper found, mocks/electron NOT found

# TASK-016: Build web SPA
pnpm build:frontend:web

# Verify output
ls out/web/
# Expected: web-index.html (or similar), assets/ directory

# TASK-015: Build backend
pnpm build:backend
# Verify no 'electron' require in output
grep "require('electron')" out/server/index.js 2>/dev/null | head -5
# Expected: empty

# Check daemon-entry also built
ls out/server/daemon-entry.js
```

---

## Done criteria

**TASK-015:**
- [x] `vite.server.config.ts` dùng `electron-node-wrapper.ts`, không còn `mocks/electron`
- [x] `pnpm build:backend` thành công → `out/server/index.js`
- [x] `out/server/daemon-entry.js` tồn tại
- [x] `out/server/index.js` không chứa `require('electron')`
- [x] `node22` là build target

**TASK-016:**
- [x] `vite.web-spa.config.ts` tạo thành công
- [x] `src/platform/stubs/electron-web-stub.ts` tạo thành công
- [x] `pnpm build:frontend:web` thành công → `out/web/web-index.html`
- [x] Dev server proxy WebSocket đến `:6768`
